package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/r2"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

var uploadGeneratedVideo = uploadGeneratedVideoToR2

func persistGeneratedVideoToR2(ctx context.Context, task *model.Task, taskResult *relaycommon.TaskInfo, videoBytes []byte) string {
	if task == nil {
		return ""
	}
	cfg := r2.LoadConfig()
	if !cfg.Enabled() {
		return ""
	}
	body := videoBytes
	if len(body) == 0 {
		raw := ""
		if taskResult != nil {
			raw = strings.TrimSpace(taskResult.Url)
		}
		decoded, err := decodeVideoDataURL(raw)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("skip R2 upload for task %s: %v", task.TaskID, err))
			return ""
		}
		body = decoded
	}
	key, err := r2.ObjectKey(task.TaskID, time.Now())
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("skip R2 upload for task %s: %v", task.TaskID, err))
		return ""
	}
	if ctx == nil {
		ctx = context.Background()
	}
	publicURL, err := uploadGeneratedVideo(ctx, cfg, key, body)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("R2 upload failed for task %s, fallback to proxy: %v", task.TaskID, err))
		return ""
	}
	logger.LogInfo(ctx, fmt.Sprintf("uploaded video task %s to R2 key %s", task.TaskID, key))
	return publicURL
}

func uploadGeneratedVideoToR2(ctx context.Context, cfg r2.Config, key string, body []byte) (string, error) {
	client, err := r2.NewClient(cfg)
	if err != nil {
		return "", err
	}
	uploadCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return client.Upload(uploadCtx, key, bytes.NewReader(body), int64(len(body)), "video/mp4")
}

func decodeVideoDataURL(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "data:") {
		return nil, errors.New("empty video bytes")
	}
	comma := strings.IndexByte(raw, ',')
	if comma < 0 {
		return nil, errors.New("invalid data URL")
	}
	payload := raw[comma+1:]
	if strings.Contains(strings.ToLower(raw[:comma]), ";base64") {
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return base64.RawStdEncoding.DecodeString(payload)
		}
		return data, nil
	}
	decoded, err := url.QueryUnescape(payload)
	if err != nil {
		return nil, err
	}
	return []byte(decoded), nil
}
