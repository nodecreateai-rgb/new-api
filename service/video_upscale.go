package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/tidwall/gjson"
)

// Upscale after generation is done in new-api so every video channel gets it
// without touching each bridge. upscaler2api handles long videos itself: it
// cuts them into per-account parts (15s at 720P, 10s at 1080P/2K/4K),
// upscales the parts in parallel and concatenates them.

func upscalerBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("UPSCALER2API_BASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://upscaler2api-upscaler2api-1:38761"
}

func upscalerKey() string {
	return strings.TrimSpace(firstEnv("UPSCALER2API_GATEWAY_KEY", "UPSCALER2API_API_KEY"))
}

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ParseUpscalerOption reads the "upscaler" request field: the mode name
// general / anime / portrait (true means general). Resolution is the
// upscaler default (720P). Also read from metadata.upscaler or a form field.
func ParseUpscalerOption(body []byte, form func(string) string) *model.TaskUpscaler {
	raw := ""
	if len(body) > 0 && gjson.ValidBytes(body) {
		for _, p := range []string{"upscaler", "metadata.upscaler"} {
			if r := gjson.GetBytes(body, p); r.Exists() {
				raw = r.String()
				break
			}
		}
	}
	if raw == "" && form != nil {
		raw = form("upscaler")
	}
	m := strings.ToLower(strings.TrimSpace(raw))
	switch m {
	case "", "0", "false", "no", "none", "off", "null":
		return nil
	case "true", "1", "yes", "on", "general", "default", "standard":
		m = "general"
	case "anime", "cartoon":
		m = "anime"
	case "portrait", "face", "human":
		m = "portrait"
	default:
		return nil
	}
	return &model.TaskUpscaler{Quality: "720P", Model: m, Style: "precise"}
}

func upscalerDo(ctx context.Context, method, path string, body []byte, timeout time.Duration) (*http.Response, error) {
	rctx, cancel := context.WithTimeout(ctx, timeout)
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(rctx, method, upscalerBaseURL()+path, rd)
	if err != nil {
		cancel()
		return nil, err
	}
	if k := upscalerKey(); k != "" {
		req.Header.Set("Authorization", "Bearer "+k)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := GetHttpClient().Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error { err := c.ReadCloser.Close(); c.cancel(); return err }

func submitUpscale(ctx context.Context, video []byte, o *model.TaskUpscaler) (string, error) {
	payload, err := common.Marshal(map[string]any{
		"model":         "a1d-video-upscale-720p",
		"video_base64":  "data:video/mp4;base64," + base64.StdEncoding.EncodeToString(video),
		"video_quality": o.Quality,
		"upscale_model": o.Model,
		"style":         o.Style,
	})
	if err != nil {
		return "", err
	}
	resp, err := upscalerDo(ctx, http.MethodPost, "/v1/videos", payload, 3*time.Minute)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	id := gjson.GetBytes(raw, "id").String()
	if resp.StatusCode >= 300 || id == "" {
		return "", fmt.Errorf("upscaler submit HTTP %d: %.300s", resp.StatusCode, raw)
	}
	return id, nil
}

// pollUpscale returns (video, done, err). done=false means still running.
func pollUpscale(ctx context.Context, id string) ([]byte, bool, error) {
	resp, err := upscalerDo(ctx, http.MethodGet, "/v1/videos/"+id, nil, 30*time.Second)
	if err != nil {
		return nil, false, nil // transient: keep waiting
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, true, fmt.Errorf("upscaler task %s not found", id)
	}
	if resp.StatusCode >= 300 {
		return nil, false, nil
	}
	switch strings.ToLower(gjson.GetBytes(raw, "status").String()) {
	case "completed", "succeeded", "success":
	case "failed", "failure", "error":
		msg := gjson.GetBytes(raw, "error.message").String()
		if msg == "" {
			msg = gjson.GetBytes(raw, "message").String()
		}
		return nil, true, fmt.Errorf("upscale failed: %s", msg)
	default:
		return nil, false, nil
	}
	dl, err := upscalerDo(ctx, http.MethodGet, "/v1/task/"+id+"/content", nil, 5*time.Minute)
	if err != nil {
		return nil, false, nil
	}
	defer dl.Body.Close()
	if dl.StatusCode != http.StatusOK {
		return nil, false, nil
	}
	body, err := io.ReadAll(io.LimitReader(dl.Body, maxVideoValidationBytes+1))
	if err != nil || len(body) == 0 {
		return nil, false, nil
	}
	if int64(len(body)) > maxVideoValidationBytes {
		return nil, true, fmt.Errorf("upscaled video too large: %d bytes", len(body))
	}
	return body, true, nil
}

// handleVideoUpscale runs one step of the upscale state machine for a task
// whose generation already succeeded. It returns:
//   - wait=true: upscale still running; caller keeps the task in progress.
//   - wait=false, video!=nil: use the upscaled video as the result.
//   - wait=false, video==nil: upscale gave up; keep the original video.
func handleVideoUpscale(ctx context.Context, task *model.Task, original []byte) (video []byte, wait bool) {
	up := task.PrivateData.Upscaler
	if up == nil || up.Done {
		return nil, false
	}
	if up.TaskID == "" {
		if len(original) == 0 {
			up.Done, up.Error = true, "source video unavailable"
			logger.LogWarn(ctx, fmt.Sprintf("Task %s upscale skipped: source video unavailable", task.TaskID))
			return nil, false
		}
		id, err := submitUpscale(ctx, original, up)
		if err != nil {
			up.Attempts++
			logger.LogWarn(ctx, fmt.Sprintf("Task %s upscale submit attempt %d failed: %v", task.TaskID, up.Attempts, err))
			if up.Attempts >= 5 {
				up.Done, up.Error = true, err.Error()
				return nil, false
			}
			return nil, true
		}
		up.TaskID, up.StartedAt = id, time.Now().Unix()
		logger.LogInfo(ctx, fmt.Sprintf("Task %s upscale submitted: %s (%s/%s/%s)", task.TaskID, id, up.Quality, up.Model, up.Style))
		return nil, true
	}
	out, done, err := pollUpscale(ctx, up.TaskID)
	if !done {
		if up.StartedAt > 0 && time.Since(time.Unix(up.StartedAt, 0)) > 90*time.Minute {
			up.Done, up.Error = true, "upscale timeout"
			return nil, false
		}
		return nil, true
	}
	if err != nil {
		up.Attempts++
		logger.LogWarn(ctx, fmt.Sprintf("Task %s upscale %s failed (attempt %d): %v", task.TaskID, up.TaskID, up.Attempts, err))
		if up.Attempts < 3 {
			up.TaskID = "" // resubmit
			return nil, true
		}
		up.Done, up.Error = true, err.Error()
		return nil, false
	}
	up.Done = true
	logger.LogInfo(ctx, fmt.Sprintf("Task %s upscale done: %d bytes", task.TaskID, len(out)))
	return out, false
}
