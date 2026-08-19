package controller

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// videoProxyError returns a standardized OpenAI-style error response.
func videoProxyError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    errType,
		},
	})
}

func VideoProxy(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error", "task_id is required")
		return
	}

	userID := c.GetInt("id")
	task, exists, err := model.GetByTaskId(userID, taskID)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		// The content URL is returned to API clients as the task result_url. It must
		// remain dereferenceable even when the downloader does not send the original
		// API token (browsers, video tags, and some clients follow the URL directly).
		// Task IDs are high-entropy public handles, so fall back to a global lookup
		// for this read-only video proxy endpoint. Admin previews also rely on this.
		task, exists, err = model.GetByOnlyTaskId(taskID)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to query task by public id %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
			return
		}
	}
	if !exists || task == nil {
		videoProxyError(c, http.StatusNotFound, "invalid_request_error", "Task not found")
		return
	}

	if task.Status != model.TaskStatusSuccess {
		videoProxyError(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("Task is not completed yet, current status: %s", task.Status))
		return
	}

	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to get channel for task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to retrieve channel information")
		return
	}
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}

	var videoURL string
	proxy := channel.GetSetting().Proxy
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to create proxy client for task %s: %s", taskID, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy client")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "", nil)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to create request: %s", err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}
	// Preserve byte-range semantics for browser video seeking. Without this, the
	// gateway converts every Range request into a full upstream GET and always
	// returns 200, forcing clients to redownload the complete MP4.
	if rangeHeader := strings.TrimSpace(c.GetHeader("Range")); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	switch channel.Type {
	case constant.ChannelTypeGemini:
		apiKey := task.PrivateData.Key
		if apiKey == "" {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Missing stored API key for Gemini task %s", taskID))
			videoProxyError(c, http.StatusInternalServerError, "server_error", "API key not stored for task")
			return
		}
		videoURL, err = getGeminiVideoURL(channel, task, apiKey)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Gemini video URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve Gemini video URL")
			return
		}
		req.Header.Set("x-goog-api-key", apiKey)
	case constant.ChannelTypeVertexAi:
		videoURL, err = getVertexVideoURL(channel, task)
		if err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to resolve Vertex video URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to resolve Vertex video URL")
			return
		}
	case constant.ChannelTypeOpenAI, constant.ChannelTypeSora:
		if directURL := strings.TrimSpace(task.PrivateData.UpstreamResultURL); directURL != "" {
			videoURL = directURL
		} else if directURL := getStoredVideoURL(task); directURL != "" {
			videoURL = directURL
		} else if directURL := resolveUpstreamTaskVideoURL(c.Request.Context(), client, baseURL, channel.Key, task.GetUpstreamTaskID()); directURL != "" {
			videoURL = directURL
		} else if contentURL := privateVideoContentURL(baseURL, task.GetUpstreamTaskID()); contentURL != "" {
			videoURL = contentURL
			req.Header.Set("Authorization", "Bearer "+channel.Key)
		} else if outputURL := upstreamVideoOutputURL(baseURL, task.GetUpstreamTaskID()); outputURL != "" {
			videoURL = outputURL
		} else {
			videoURL = fmt.Sprintf("%s/v1/videos/%s/content", baseURL, task.GetUpstreamTaskID())
			req.Header.Set("Authorization", "Bearer "+channel.Key)
		}
	default:
		videoURL = getStoredVideoURL(task)
	}

	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Video URL is empty for task %s", taskID))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}
	wasRelativeVideoURL := strings.HasPrefix(videoURL, "/") && !strings.HasPrefix(videoURL, "//")
	videoURL = resolvePossiblyRelativeVideoURL(videoURL, baseURL)
	trustedChannelURL := isTrustedChannelVideoURL(videoURL, baseURL)

	if strings.HasPrefix(videoURL, "data:") {
		if err := writeVideoDataURL(c, videoURL); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to decode video data URL for task %s: %s", taskID, err.Error()))
			videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		}
		return
	}

	if !wasRelativeVideoURL && !trustedChannelURL {
		fetchSetting := system_setting.GetFetchSetting()
		if err := common.ValidateURLWithFetchSetting(videoURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
			logger.LogError(c.Request.Context(), fmt.Sprintf("Video URL blocked for task %s: %v", taskID, err))
			videoProxyError(c, http.StatusForbidden, "server_error", fmt.Sprintf("request blocked: %v", err))
			return
		}
	}

	req.URL, err = url.Parse(videoURL)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to parse URL %s: %s", videoURL, err.Error()))
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to create proxy request")
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch video from %s: %s", videoURL, err.Error()))
		videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound && shouldRetryVideoWithCacheBypass(videoURL, baseURL) {
		_ = resp.Body.Close()
		retryURL := addVideoCacheBypass(videoURL, task.UpdatedAt)
		retryReq := req.Clone(ctx)
		retryReq.URL, err = url.Parse(retryURL)
		if err == nil {
			resp, err = client.Do(retryReq)
			if err != nil {
				logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to fetch video from %s: %s", retryURL, err.Error()))
				videoProxyError(c, http.StatusBadGateway, "server_error", "Failed to fetch video content")
				return
			}
			defer resp.Body.Close()
			videoURL = retryURL
		}
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Upstream returned status %d for %s", resp.StatusCode, videoURL))
		videoProxyError(c, http.StatusBadGateway, "server_error",
			fmt.Sprintf("Upstream service returned status %d", resp.StatusCode))
		return
	}

	for _, key := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Last-Modified", "ETag", "Content-Disposition"} {
		if value := resp.Header.Get(key); value != "" {
			c.Writer.Header().Set(key, value)
		}
	}
	c.Writer.Header().Set("Cache-Control", "public, max-age=86400")
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(c.Writer, resp.Body); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Failed to stream video content: %s", err.Error()))
	}
}

func resolveUpstreamTaskVideoURL(ctx context.Context, client *http.Client, baseURL, apiKey, upstreamTaskID string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	upstreamTaskID = strings.TrimSpace(upstreamTaskID)
	if baseURL == "" || upstreamTaskID == "" {
		return ""
	}
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, baseURL+"/v1/videos/"+url.PathEscape(upstreamTaskID), nil)
	if err != nil {
		return ""
	}
	if key := strings.TrimSpace(apiKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return ""
	}
	var payload any
	if err := common.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return findVideoURLInPayload(payload)
}

func privateVideoContentURL(baseURL, upstreamTaskID string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	upstreamTaskID = strings.TrimPrefix(strings.TrimSpace(upstreamTaskID), "task_")
	if baseURL == "" || upstreamTaskID == "" || !strings.Contains(strings.ToLower(baseURL), "video-generation-upstream") {
		return ""
	}
	return baseURL + "/v1/videos/task/task_" + url.PathEscape(upstreamTaskID) + "/content"
}

func upstreamVideoOutputURL(baseURL, upstreamTaskID string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	upstreamTaskID = strings.TrimSpace(upstreamTaskID)
	if baseURL == "" || upstreamTaskID == "" || strings.Contains(strings.ToLower(baseURL), "api.openai.com") {
		return ""
	}
	name := upstreamTaskID
	if !strings.HasPrefix(name, "task_") {
		name = "task_" + name
	}
	return baseURL + "/outputs/" + url.PathEscape(name) + ".mp4"
}

func isTrustedChannelVideoURL(videoURL, baseURL string) bool {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	videoURL = strings.TrimSpace(videoURL)
	return baseURL != "" && strings.HasPrefix(videoURL, baseURL+"/")
}

func shouldRetryVideoWithCacheBypass(videoURL, baseURL string) bool {
	if !isTrustedChannelVideoURL(videoURL, baseURL) {
		return false
	}
	parsed, err := url.Parse(videoURL)
	if err != nil {
		return false
	}
	return strings.HasPrefix(parsed.Path, "/outputs/")
}

func addVideoCacheBypass(videoURL string, updatedAt int64) string {
	parsed, err := url.Parse(videoURL)
	if err != nil {
		return videoURL
	}
	if updatedAt <= 0 {
		updatedAt = time.Now().Unix()
	}
	q := parsed.Query()
	if q.Get("_cb") == "" {
		q.Set("_cb", fmt.Sprintf("%d", updatedAt))
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

func resolvePossiblyRelativeVideoURL(rawURL, baseURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || strings.HasPrefix(rawURL, "data:") || strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return rawURL
	}
	if !strings.HasPrefix(rawURL, "/") {
		return rawURL
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return rawURL
	}
	return baseURL + rawURL
}

func getStoredVideoURL(task *model.Task) string {
	if task == nil {
		return ""
	}
	if rel := storedTaskOutputURL(task); rel != "" {
		return rel
	}
	candidates := []string{task.GetResultURL()}
	if len(task.Data) > 0 {
		var payload map[string]any
		if err := common.Unmarshal(task.Data, &payload); err == nil {
			candidates = append(candidates, findVideoURLInPayload(payload))
		}
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || isTaskProxyContentURL(candidate, task.TaskID) {
			continue
		}
		return candidate
	}
	return ""
}

func storedTaskOutputURL(task *model.Task) string {
	if task == nil || len(task.Data) == 0 {
		return ""
	}
	var payload map[string]any
	if err := common.Unmarshal(task.Data, &payload); err != nil {
		return ""
	}
	localPath, _ := payload["local_path"].(string)
	localPath = strings.TrimSpace(localPath)
	if localPath == "" {
		return ""
	}
	idx := strings.LastIndex(localPath, "/")
	name := localPath
	if idx >= 0 {
		name = localPath[idx+1:]
	}
	if name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
		return ""
	}
	return "/outputs/" + url.PathEscape(name)
}

func findVideoURLInPayload(payload any) string {
	switch v := payload.(type) {
	case map[string]any:
		for _, key := range []string{"video_url", "videoUrl", "url", "output_url", "outputUrl", "remote_url", "remoteUrl"} {
			if value, ok := v[key].(string); ok && strings.TrimSpace(value) != "" {
				return value
			}
		}
		for _, value := range v {
			if found := findVideoURLInPayload(value); found != "" {
				return found
			}
		}
	case []any:
		for _, item := range v {
			if found := findVideoURLInPayload(item); found != "" {
				return found
			}
		}
	}
	return ""
}

func writeVideoDataURL(c *gin.Context, dataURL string) error {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data url")
	}

	header := parts[0]
	payload := parts[1]
	if !strings.HasPrefix(header, "data:") || !strings.Contains(header, ";base64") {
		return fmt.Errorf("unsupported data url")
	}

	mimeType := strings.TrimPrefix(header, "data:")
	mimeType = strings.TrimSuffix(mimeType, ";base64")
	if mimeType == "" {
		mimeType = "video/mp4"
	}

	videoBytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		videoBytes, err = base64.RawStdEncoding.DecodeString(payload)
		if err != nil {
			return err
		}
	}

	c.Writer.Header().Set("Content-Type", mimeType)
	c.Writer.Header().Set("Cache-Control", "public, max-age=86400")
	c.Writer.WriteHeader(http.StatusOK)
	_, err = c.Writer.Write(videoBytes)
	return err
}
