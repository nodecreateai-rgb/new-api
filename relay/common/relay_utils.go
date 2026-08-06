package common

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type HasPrompt interface {
	GetPrompt() string
}

type HasImage interface {
	HasImage() bool
}

func GetFullRequestURL(baseURL string, requestURL string, channelType int) string {
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)

	if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
		switch channelType {
		case constant.ChannelTypeOpenAI:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/v1"))
		case constant.ChannelTypeAzure:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/openai/deployments"))
		}
	}
	return fullRequestURL
}

func GetAPIVersion(c *gin.Context) string {
	query := c.Request.URL.Query()
	apiVersion := query.Get("api-version")
	if apiVersion == "" {
		apiVersion = c.GetString("api_version")
	}
	return apiVersion
}

func createTaskError(err error, code string, statusCode int, localError bool) *dto.TaskError {
	return &dto.TaskError{
		Code:       code,
		Message:    err.Error(),
		StatusCode: statusCode,
		LocalError: localError,
		Error:      err,
	}
}

func storeTaskRequest(c *gin.Context, info *RelayInfo, action string, requestObj TaskSubmitReq) {
	info.Action = action
	c.Set("task_request", requestObj)
}
func GetTaskRequest(c *gin.Context) (TaskSubmitReq, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return TaskSubmitReq{}, fmt.Errorf("request not found in context")
	}
	req, ok := v.(TaskSubmitReq)
	if !ok {
		return TaskSubmitReq{}, fmt.Errorf("invalid task request type")
	}
	return req, nil
}

func validatePrompt(prompt string) *dto.TaskError {
	if strings.TrimSpace(prompt) == "" {
		return createTaskError(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest, true)
	}
	return nil
}

func validateMultipartTaskRequest(c *gin.Context, info *RelayInfo, action string) (TaskSubmitReq, error) {
	var req TaskSubmitReq
	form, err := c.MultipartForm()
	if err != nil {
		return req, err
	}

	formData := c.Request.PostForm
	req = TaskSubmitReq{
		Prompt:         formData.Get("prompt"),
		Model:          formData.Get("model"),
		Mode:           formData.Get("mode"),
		Image:          formData.Get("image"),
		ImageURL:       formData.Get("image_url"),
		ReferenceImage: formData.Get("reference_image"),
		InputReference: formData.Get("input_reference"),
		Size:           formData.Get("size"),
		Resolution:     formData.Get("resolution"),
		Aspect:         formData.Get("aspect"),
		AspectRatio:    formData.Get("aspect_ratio"),
		Ratio:          formData.Get("ratio"),
		ComplianceMode: formData.Get("compliance_mode"),
		Metadata:       make(map[string]interface{}),
	}
	req.ImageRefs = append([]string(nil), formData["image_refs"]...)
	req.ImageURLs = append([]string(nil), formData["image_urls"]...)
	// Some clients submit several references as repeated historically singular
	// fields. Keep the scalar first item for compatibility, and also preserve the
	// complete repeated lists so the outbound canonical image_refs array is not
	// silently truncated to one image. This applies to every singular alias, not
	// only image_url: production clients also repeat image/input_reference.
	req.Images = append(req.Images, formData["image"]...)
	req.Images = append(req.Images, formData["input_reference"]...)
	req.ImageURLs = append(req.ImageURLs, formData["image_url"]...)
	req.ReferenceImages = append([]string(nil), formData["reference_images"]...)
	req.ReferenceImages = append(req.ReferenceImages, formData["reference_image"]...)
	req.ExtraImages = append([]string(nil), formData["extra_images"]...)
	req.Video = formData.Get("video")
	req.VideoURL = formData.Get("video_url")
	req.ReferenceVideo = formData.Get("reference_video")
	req.VideoRefs = append([]string(nil), formData["video_refs"]...)
	req.VideoURLs = append([]string(nil), formData["video_urls"]...)
	req.VideoURLs = append(req.VideoURLs, formData["video_url"]...)
	req.ReferenceVideos = append([]string(nil), formData["reference_videos"]...)
	req.ReferenceVideos = append(req.ReferenceVideos, formData["reference_video"]...)
	req.AudioURL = formData.Get("audio_url")
	req.ReferenceAudio = formData.Get("reference_audio")
	req.AudioRefs = append([]string(nil), formData["audio_refs"]...)
	req.AudioURLs = append([]string(nil), formData["audio_urls"]...)
	req.AudioURLs = append(req.AudioURLs, formData["audio_url"]...)
	req.ReferenceAudios = append([]string(nil), formData["reference_audios"]...)
	req.ReferenceAudios = append(req.ReferenceAudios, formData["reference_audio"]...)
	req.Audios = append([]string(nil), formData["audios"]...)
	req.ExtraAudios = append([]string(nil), formData["extra_audios"]...)
	if raw := strings.TrimSpace(formData.Get("compliance_enabled")); raw != "" {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			req.ComplianceEnabled = &enabled
		}
	}
	if strings.TrimSpace(req.AspectRatio) == "" {
		req.AspectRatio = strings.TrimSpace(req.Ratio)
	}
	if strings.TrimSpace(req.AspectRatio) == "" {
		req.AspectRatio = strings.TrimSpace(req.Aspect)
	}

	req.Seconds = strings.TrimSpace(formData.Get("seconds"))
	if durationStr := strings.TrimSpace(formData.Get("duration")); durationStr != "" {
		if duration, err := strconv.Atoi(durationStr); err == nil {
			req.Duration = duration
		}
	}
	if req.Duration <= 0 && req.Seconds != "" {
		if duration, err := strconv.Atoi(req.Seconds); err == nil {
			req.Duration = duration
		}
	}

	if images := formData["images"]; len(images) > 0 {
		req.Images = images
	}
	for _, field := range []string{"image", "image[]", "images", "image_refs", "image_urls", "reference_images", "extra_images", "input_reference"} {
		for _, fileHeader := range form.File[field] {
			dataURL, err := taskMultipartFileDataURL(fileHeader)
			if err != nil {
				return req, fmt.Errorf("read multipart %s: %w", field, err)
			}
			req.Images = append(req.Images, dataURL)
		}
	}

	for key, values := range formData {
		if len(values) > 0 && !isKnownTaskField(key) {
			if intVal, err := strconv.Atoi(values[0]); err == nil {
				req.Metadata[key] = intVal
			} else if floatVal, err := strconv.ParseFloat(values[0], 64); err == nil {
				req.Metadata[key] = floatVal
			} else {
				req.Metadata[key] = values[0]
			}
		}
	}
	return req, nil
}

func taskMultipartFileDataURL(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func ValidateMultipartDirect(c *gin.Context, info *RelayInfo) *dto.TaskError {
	var prompt string
	var model string
	var seconds int
	var size string
	var hasInputReference bool

	var req TaskSubmitReq
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		var err error
		req, err = validateMultipartTaskRequest(c, info, constant.TaskActionTextGenerate)
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
	} else if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_json", http.StatusBadRequest, true)
	}

	prompt = req.Prompt
	model = req.Model
	size = req.Size
	seconds, _ = strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if req.InputReference != "" {
		req.Images = append(req.Images, req.InputReference)
	}

	if strings.TrimSpace(req.Model) == "" {
		return createTaskError(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest, true)
	}

	if req.HasAudioReference() && !supportsAudioReference(info.OriginModelName) {
		return createTaskError(fmt.Errorf("audio reference is not supported for this video model"), "unsupported_audio_reference", http.StatusBadRequest, true)
	}
	if req.HasImage() || req.HasVideo() {
		hasInputReference = true
	}

	if taskErr := validatePrompt(prompt); taskErr != nil {
		return taskErr
	}

	action := constant.TaskActionTextGenerate
	if hasInputReference {
		action = constant.TaskActionGenerate
	}
	if strings.HasPrefix(model, "sora-2") {

		if size == "" {
			size = "720x1280"
		}

		if seconds <= 0 {
			seconds = 4
		}

		if model == "sora-2" && !lo.Contains([]string{"720x1280", "1280x720"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		if model == "sora-2-pro" && !lo.Contains([]string{"720x1280", "1280x720", "1792x1024", "1024x1792"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		// OtherRatios 已移到 Sora adaptor 的 EstimateBilling 中设置
	}

	storeTaskRequest(c, info, action, req)

	return nil
}

func supportsAudioReference(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(model, "seedance-720") || strings.HasPrefix(model, "klsdpro2") {
		return true
	}
	switch model {
	case "sd2.5", "seedance-2.5-omni", "sd2-mini", "seedance2_mini":
		return true
	case "seedance-video-fast", "seedance-video-standard",
		"seedance-video-fast-per-second", "seedance-video-standard-per-second",
		"seedance-2.0-fast-720p", "seedance-2.0-720p",
		"seedance-2.0-1080p", "seedance-2.0-4k":
		return true
	default:
		return false
	}
}

func isKnownTaskField(field string) bool {
	knownFields := map[string]bool{
		"prompt":             true,
		"model":              true,
		"mode":               true,
		"image":              true,
		"image_url":          true,
		"image_refs":         true,
		"image_urls":         true,
		"reference_image":    true,
		"reference_images":   true,
		"extra_images":       true,
		"images":             true,
		"video":              true,
		"video_url":          true,
		"video_refs":         true,
		"video_urls":         true,
		"reference_video":    true,
		"reference_videos":   true,
		"extra_videos":       true,
		"audio_url":          true,
		"audio_refs":         true,
		"audio_urls":         true,
		"reference_audio":    true,
		"reference_audios":   true,
		"extra_audios":       true,
		"audios":             true,
		"size":               true,
		"resolution":         true,
		"aspect":             true,
		"aspect_ratio":       true,
		"ratio":              true,
		"duration":           true,
		"input_reference":    true, // Sora 特有字段
		"compliance_enabled": true,
		"compliance_mode":    true,
	}
	return knownFields[field]
}

func ValidateBasicTaskRequest(c *gin.Context, info *RelayInfo, action string) *dto.TaskError {
	var err error
	contentType := c.GetHeader("Content-Type")
	var req TaskSubmitReq
	if strings.HasPrefix(contentType, "multipart/form-data") {
		req, err = validateMultipartTaskRequest(c, info, action)
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
	} else {
		// 为了metadata字段的兼容性，统一UnmarshalBodyReusable
		if err := common.UnmarshalBodyReusable(c, &req); err != nil {
			return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
		}
	}

	if taskErr := validatePrompt(req.Prompt); taskErr != nil {
		return taskErr
	}

	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{req.Image}
	}

	storeTaskRequest(c, info, action, req)
	return nil
}
