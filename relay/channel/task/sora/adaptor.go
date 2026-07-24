package sora

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string    `json:"type"`                // "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // for text type
	ImageURL *ImageURL `json:"image_url,omitempty"` // for image_url type
}

type ImageURL struct {
	URL string `json:"url"`
}

type responseTask struct {
	ID                 string `json:"id"`
	TaskID             string `json:"task_id,omitempty"` //兼容旧接口
	Object             string `json:"object"`
	Model              string `json:"model"`
	Status             string `json:"status"`
	Progress           int    `json:"progress"`
	CreatedAt          int64  `json:"created_at"`
	CompletedAt        int64  `json:"completed_at,omitempty"`
	ExpiresAt          int64  `json:"expires_at,omitempty"`
	Seconds            string `json:"seconds,omitempty"`
	Size               string `json:"size,omitempty"`
	RemixedFromVideoID string `json:"remixed_from_video_id,omitempty"`
	Error              *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func validateRemixRequest(c *gin.Context) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field prompt is required"), "invalid_request", http.StatusBadRequest)
	}
	// 存储原始请求到 context，与 ValidateMultipartDirect 路径保持一致
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	if info.Action == constant.TaskActionRemix {
		return validateRemixRequest(c)
	}
	return relaycommon.ValidateMultipartDirect(c, info)
}

// EstimateBilling 根据用户请求的 seconds 和 size 计算 OtherRatios。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	// remix 路径的 OtherRatios 已在 ResolveOriginTask 中设置
	if info.Action == constant.TaskActionRemix {
		return nil
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	seconds, _ := strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if seconds <= 0 {
		seconds = 4
	}

	size := req.Size
	if size == "" {
		size = "720x1280"
	}

	ratios := map[string]float64{
		"seconds": float64(seconds),
		"size":    1,
	}
	if size == "1792x1024" || size == "1024x1792" {
		ratios["size"] = 1.666667
	}
	return ratios
}

// ForceApplyBillingRatios lets selected OpenAI-compatible video aliases keep
// request-derived multipliers (seconds / size) even when the upstream base model
// remains in TASK_PRICE_PATCH for per-item billing. This supports public aliases
// that route to the same upstream model but are sold per second.
func (a *TaskAdaptor) ForceApplyBillingRatios(info *relaycommon.RelayInfo) bool {
	model := strings.TrimSpace(info.OriginModelName)
	return model == "seedance-video-fast-per-second" || model == "seedance-video-standard-per-second"
}

// UseRequestBillingRatios declares whether request-derived duration/size ratios
// should modify the configured per-item model price. Public Seedance 2.0 model
// IDs are fixed-price per generated video, independent of requested duration.
func (a *TaskAdaptor) UseRequestBillingRatios(info *relaycommon.RelayInfo) bool {
	switch strings.TrimSpace(info.OriginModelName) {
	case "seedance-2.0-fast-720p", "seedance-2.0-720p", "seedance-2.0-1080p":
		return false
	default:
		return true
	}
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info.Action == constant.TaskActionRemix {
		return fmt.Sprintf("%s/v1/videos/%s/remix", a.baseURL, info.OriginTaskID), nil
	}
	return fmt.Sprintf("%s/v1/videos", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	// Forward the authenticated New-API identity so private upstream account
	// pools can enforce per-user routing without exposing it to clients.
	req.Header.Set("X-New-API-User-ID", fmt.Sprintf("%d", info.UserId))
	req.Header.Set("X-New-API-Username", common.GetContextKeyString(c, constant.ContextKeyUserName))
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_request_body_failed")
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "read_body_bytes_failed")
	}
	contentType := c.GetHeader("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		var bodyMap map[string]interface{}
		if err := common.Unmarshal(cachedBody, &bodyMap); err == nil {
			bodyMap["model"] = info.UpstreamModelName
			// Re-apply parsed compatibility fields at the final JSON boundary. The
			// public decoder accepts legacy/string forms (for example
			// compliance_enabled="true"), but forwarding the original body map
			// would leak that string to strict Go upstream DTOs and cause an
			// immediate 400 before a task is persisted.
			if parsed, reqErr := relaycommon.GetTaskRequest(c); reqErr == nil {
				applyCanonicalVideoControls(bodyMap, parsed)
			}
			normalizeOpenAIVideoAspectBody(bodyMap)
			if newBody, err := common.Marshal(bodyMap); err == nil {
				return bytes.NewReader(newBody), nil
			}
		}
		return bytes.NewReader(cachedBody), nil
	}

	if strings.Contains(contentType, "multipart/form-data") {
		if upstreamVideoTaskPrefersJSON(a.baseURL) {
			if newBody, err := buildUpstreamVideoJSONFromMultipart(c, info); err == nil {
				c.Request.Header.Set("Content-Type", "application/json")
				return bytes.NewReader(newBody), nil
			}
		}
		formData, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return bytes.NewReader(cachedBody), nil
		}
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("model", info.UpstreamModelName)
		values := normalizeOpenAIVideoAspectForm(formData.Value)
		for key, vals := range values {
			if key == "model" {
				continue
			}
			for _, v := range vals {
				writer.WriteField(key, v)
			}
		}
		for fieldName, fileHeaders := range formData.File {
			for _, fh := range fileHeaders {
				f, err := fh.Open()
				if err != nil {
					continue
				}
				ct := fh.Header.Get("Content-Type")
				if ct == "" || ct == "application/octet-stream" {
					buf512 := make([]byte, 512)
					n, _ := io.ReadFull(f, buf512)
					ct = http.DetectContentType(buf512[:n])
					// Re-open after sniffing so the full content is copied below
					f.Close()
					f, err = fh.Open()
					if err != nil {
						continue
					}
				}
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fh.Filename))
				h.Set("Content-Type", ct)
				part, err := writer.CreatePart(h)
				if err != nil {
					f.Close()
					continue
				}
				io.Copy(part, f)
				f.Close()
			}
		}
		writer.Close()
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return &buf, nil
	}

	return common.ReaderOnly(storage), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func applyCanonicalVideoControls(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	if req.ComplianceEnabled != nil {
		body["compliance_enabled"] = *req.ComplianceEnabled
	}
	if mode := strings.TrimSpace(req.ComplianceMode); mode != "" {
		body["compliance_mode"] = mode
	}
	delete(body, "eye_mask_enabled")
	delete(body, "eye_mask_mode")
}

func normalizeSoraVideoStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "pending":
		return dto.VideoStatusQueued
	case "processing", "in_progress", "running":
		return dto.VideoStatusInProgress
	case "completed", "succeeded", "success":
		return dto.VideoStatusCompleted
	case "failed", "cancelled", "canceled":
		return dto.VideoStatusFailed
	default:
		return status
	}
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Sora response
	var dResp responseTask
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	upstreamID := dResp.ID
	if upstreamID == "" {
		upstreamID = dResp.TaskID
	}
	if upstreamID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// 使用公开 task_xxxx ID 和本地模型别名返回给客户端；上游模型名仅用于内部请求。
	dResp.ID = info.PublicTaskID
	dResp.TaskID = info.PublicTaskID
	if info.OriginModelName != "" {
		dResp.Model = info.OriginModelName
	}
	dResp.Status = normalizeSoraVideoStatus(dResp.Status)
	clientResponseBody, err := common.Marshal(dResp)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "application/json", clientResponseBody)
	return upstreamID, clientResponseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/v1/videos/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	switch resTask.Status {
	case "queued", "pending":
		taskResult.Status = model.TaskStatusQueued
	case "processing", "in_progress":
		taskResult.Status = model.TaskStatusInProgress
	case "completed":
		taskResult.Status = model.TaskStatusSuccess
		// Url intentionally left empty — the caller constructs the proxy URL using the public task ID
	case "failed", "cancelled":
		taskResult.Status = model.TaskStatusFailure
		if resTask.Error != nil {
			taskResult.Reason = resTask.Error.Message
		} else {
			taskResult.Reason = "task failed"
		}
	default:
	}
	if resTask.Progress > 0 && resTask.Progress < 100 {
		taskResult.Progress = fmt.Sprintf("%d%%", resTask.Progress)
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	data := task.Data
	var err error
	if data, err = sjson.SetBytes(data, "id", task.TaskID); err != nil {
		return nil, errors.Wrap(err, "set id failed")
	}
	if data, err = sjson.SetBytes(data, "task_id", task.TaskID); err != nil {
		return nil, errors.Wrap(err, "set task_id failed")
	}
	status := task.Status.ToVideoStatus()
	if status == dto.VideoStatusUnknown {
		var raw responseTask
		_ = common.Unmarshal(data, &raw)
		status = normalizeSoraVideoStatus(raw.Status)
	}
	if status != "" {
		if data, err = sjson.SetBytes(data, "status", status); err != nil {
			return nil, errors.Wrap(err, "set status failed")
		}
	}
	progress := strings.TrimSuffix(task.Progress, "%")
	if progress != "" {
		if n, convErr := strconv.Atoi(progress); convErr == nil {
			if data, err = sjson.SetBytes(data, "progress", n); err != nil {
				return nil, errors.Wrap(err, "set progress failed")
			}
		}
	}
	if task.Properties.OriginModelName != "" {
		if data, err = sjson.SetBytes(data, "model", task.Properties.OriginModelName); err != nil {
			return nil, errors.Wrap(err, "set model failed")
		}
	}
	if data, err = sanitizeOpenAIVideoTaskData(data); err != nil {
		return nil, err
	}
	if status == dto.VideoStatusCompleted {
		if data, err = ensureOpenAIVideoContentURL(data, task); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func sanitizeOpenAIVideoTaskData(data []byte) ([]byte, error) {
	var payload map[string]any
	if err := common.Unmarshal(data, &payload); err != nil {
		return nil, errors.Wrap(err, "unmarshal video task data failed")
	}
	scrubOpenAIVideoPayload(payload)
	return common.Marshal(payload)
}

func scrubOpenAIVideoPayload(payload map[string]any) {
	for _, key := range []string{
		"parent_email", "account_email", "local_path", "upstream_video_id", "video_id", "remote_task_id",
		"upstream_task_id", "chat_id", "chatId", "log_id", "logId", "conversation_id",
		"url", "video_url", "public_url", "download_url", "no_watermark_url", "watermark_url",
		"remote_url", "output_url", "upstream_video_url", "poster", "thumb",
	} {
		delete(payload, key)
	}
	for key, value := range payload {
		switch typed := value.(type) {
		case string:
			payload[key] = common.MaskUpstreamProviderInfo(typed)
		case map[string]any:
			scrubOpenAIVideoPayload(typed)
		case []any:
			for _, item := range typed {
				if child, ok := item.(map[string]any); ok {
					scrubOpenAIVideoPayload(child)
				}
			}
		}
	}
}

func ensureOpenAIVideoContentURL(data []byte, task *model.Task) ([]byte, error) {
	if task == nil {
		return data, nil
	}
	contentURL := taskcommon.BuildProxyURL(task.TaskID)
	if strings.TrimSpace(contentURL) == "" {
		return data, nil
	}
	var err error
	if data, err = sjson.SetBytes(data, "url", contentURL); err != nil {
		return nil, errors.Wrap(err, "set url failed")
	}
	if data, err = sjson.SetBytes(data, "video_url", contentURL); err != nil {
		return nil, errors.Wrap(err, "set video_url failed")
	}
	if data, err = sjson.SetBytes(data, "metadata.url", contentURL); err != nil {
		return nil, errors.Wrap(err, "set metadata url failed")
	}
	if videos := gjson.GetBytes(data, "videos"); videos.Exists() && videos.IsArray() {
		videos.ForEach(func(key, _ gjson.Result) bool {
			path := fmt.Sprintf("videos.%d.url", key.Int())
			data, err = sjson.SetBytes(data, path, contentURL)
			return err == nil
		})
		if err != nil {
			return nil, errors.Wrap(err, "set videos url failed")
		}
	}
	return data, nil
}

func upstreamVideoTaskPrefersJSON(baseURL string) bool {
	return !strings.Contains(strings.ToLower(baseURL), "api.openai.com")
}

func buildUpstreamVideoJSONFromMultipart(c *gin.Context, info *relaycommon.RelayInfo) ([]byte, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		formData, parseErr := common.ParseMultipartFormReusable(c)
		if parseErr != nil {
			return nil, parseErr
		}
		req = relaycommon.TaskSubmitReq{
			Prompt:         firstFormValue(formData.Value, "prompt"),
			Model:          firstFormValue(formData.Value, "model"),
			Size:           firstFormValue(formData.Value, "size"),
			Resolution:     firstFormValue(formData.Value, "resolution"),
			Aspect:         firstFormValue(formData.Value, "aspect"),
			AspectRatio:    firstFormValue(formData.Value, "aspect_ratio"),
			Ratio:          firstFormValue(formData.Value, "ratio"),
			Seconds:        firstFormValue(formData.Value, "seconds"),
			Image:          firstFormValue(formData.Value, "image"),
			InputReference: firstFormValue(formData.Value, "input_reference"),
			ComplianceMode: firstFormValue(formData.Value, "compliance_mode"),
		}
		if raw := strings.TrimSpace(firstFormValue(formData.Value, "compliance_enabled")); raw != "" {
			if enabled, convErr := strconv.ParseBool(raw); convErr == nil {
				req.ComplianceEnabled = &enabled
			}
		}
		if duration, convErr := strconv.Atoi(firstFormValue(formData.Value, "duration")); convErr == nil {
			req.Duration = duration
		}
		if images := formData.Value["images"]; len(images) > 0 {
			req.Images = append([]string(nil), images...)
		}
	}
	bodyMap := taskSubmitReqToUpstreamVideoBody(req, info.UpstreamModelName)
	normalizeOpenAIVideoAspectBody(bodyMap)
	return common.Marshal(bodyMap)
}

func firstFormValue(values map[string][]string, key string) string {
	if xs := values[key]; len(xs) > 0 {
		return xs[0]
	}
	return ""
}

func taskSubmitReqToUpstreamVideoBody(req relaycommon.TaskSubmitReq, upstreamModel string) map[string]interface{} {
	body := map[string]interface{}{
		"model":  upstreamModel,
		"prompt": req.Prompt,
	}
	if duration := taskSubmitDuration(req); duration > 0 {
		body["duration"] = duration
	}
	size := strings.TrimSpace(req.Size)
	if size == "" {
		size = strings.TrimSpace(req.Resolution)
	}
	if size != "" {
		body["size"] = size
	}
	aspect := strings.TrimSpace(req.AspectRatio)
	if aspect == "" {
		aspect = strings.TrimSpace(req.Ratio)
	}
	if aspect == "" {
		aspect = strings.TrimSpace(req.Aspect)
	}
	if aspect != "" {
		body["aspect_ratio"] = aspect
	}
	if req.ComplianceEnabled != nil {
		body["compliance_enabled"] = *req.ComplianceEnabled
	}
	if complianceMode := strings.TrimSpace(req.ComplianceMode); complianceMode != "" {
		body["compliance_mode"] = complianceMode
	}
	if imageRefs := collectUpstreamVideoImageRefs(req); len(imageRefs) > 0 {
		body["image_refs"] = imageRefs
		body["image_url"] = imageRefs[0]
	}
	if videoRefs := collectUpstreamVideoVideoRefs(req); len(videoRefs) > 0 {
		body["video_refs"] = videoRefs
		body["video_urls"] = videoRefs
		body["videos"] = videoRefs
		body["reference_videos"] = videoRefs
		body["video_url"] = videoRefs[0]
		body["reference_video"] = videoRefs[0]
	}
	if audioRefs := collectUpstreamVideoAudioRefs(req); len(audioRefs) > 0 {
		body["audio_refs"] = audioRefs
		body["audio_urls"] = audioRefs
		body["audios"] = audioRefs
		body["reference_audios"] = audioRefs
		body["audio_url"] = audioRefs[0]
		body["reference_audio"] = audioRefs[0]
	}
	return body
}

func taskSubmitDuration(req relaycommon.TaskSubmitReq) int {
	if req.Duration > 0 {
		return req.Duration
	}
	if seconds, err := strconv.Atoi(strings.TrimSpace(req.Seconds)); err == nil {
		return seconds
	}
	return 0
}

func collectUpstreamVideoImageRefs(req relaycommon.TaskSubmitReq) []string {
	return collectUpstreamVideoRefs(
		[]string{req.Image, req.InputReference, req.ImageURL, req.ReferenceImage},
		req.ImageRefs, req.ImageURLs, req.Images, req.ReferenceImages, req.ExtraImages,
	)
}

func collectUpstreamVideoVideoRefs(req relaycommon.TaskSubmitReq) []string {
	return collectUpstreamVideoRefs(
		[]string{req.Video, req.VideoURL, req.ReferenceVideo},
		req.VideoRefs, req.VideoURLs, req.Videos, req.ReferenceVideos, req.ExtraVideos,
	)
}

func collectUpstreamVideoAudioRefs(req relaycommon.TaskSubmitReq) []string {
	return collectUpstreamVideoRefs(
		[]string{req.AudioURL, req.ReferenceAudio},
		req.AudioRefs, req.AudioURLs, req.Audios, req.ReferenceAudios, req.ExtraAudios,
	)
}

func collectUpstreamVideoRefs(singles []string, groups ...[]string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		out = append(out, raw)
	}
	for _, raw := range singles {
		add(raw)
	}
	for _, group := range groups {
		for _, raw := range group {
			add(raw)
		}
	}
	return out
}

func normalizeOpenAIVideoAspectBody(body map[string]interface{}) {
	if body == nil {
		return
	}
	size, _ := body["size"].(string)
	if strings.TrimSpace(size) == "" {
		size, _ = body["resolution"].(string)
	}
	aspect, _ := body["aspect_ratio"].(string)
	if strings.TrimSpace(aspect) == "" {
		aspect, _ = body["ratio"].(string)
	}
	if strings.TrimSpace(aspect) == "" {
		aspect, _ = body["aspect"].(string)
	}
	aspect, size = normalizeOpenAIVideoAspectValues(aspect, size)
	if aspect != "" {
		body["aspect_ratio"] = aspect
	}
	if size != "" {
		body["size"] = size
	}
}

func normalizeOpenAIVideoAspectForm(values map[string][]string) map[string][]string {
	out := make(map[string][]string, len(values)+2)
	for k, v := range values {
		vv := append([]string(nil), v...)
		out[k] = vv
	}
	first := func(key string) string {
		if xs := out[key]; len(xs) > 0 {
			return xs[0]
		}
		return ""
	}
	aspect := first("aspect_ratio")
	if strings.TrimSpace(aspect) == "" {
		aspect = first("ratio")
	}
	aspect, size := normalizeOpenAIVideoAspectValues(aspect, first("size"))
	if aspect != "" {
		out["aspect_ratio"] = []string{aspect}
	}
	if size != "" {
		out["size"] = []string{size}
	}
	return out
}

func normalizeOpenAIVideoAspectValues(aspect, size string) (string, string) {
	aspect = strings.TrimSpace(strings.ToLower(aspect))
	size = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(size, "*", "x")))
	if aspect == "" {
		aspect = aspectFromOpenAIVideoSize(size)
	}
	if size == "" || isAspectRatioToken(size) {
		size = defaultOpenAIVideoSizeForAspect(aspect)
	}
	return aspect, size
}

func isAspectRatioToken(s string) bool {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "16:9", "9:16", "1:1", "4:3", "3:4", "21:9":
		return true
	default:
		return false
	}
}

func aspectFromOpenAIVideoSize(size string) string {
	if isAspectRatioToken(size) {
		return size
	}
	parts := strings.SplitN(size, "x", 2)
	if len(parts) != 2 {
		return ""
	}
	w, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	if w <= 0 || h <= 0 {
		return ""
	}
	if w == h {
		return "1:1"
	}
	if h > w {
		return "9:16"
	}
	return "16:9"
}

func defaultOpenAIVideoSizeForAspect(aspect string) string {
	switch strings.TrimSpace(strings.ToLower(aspect)) {
	case "9:16", "3:4":
		return "1080x1920"
	case "1:1":
		return "1080x1080"
	case "4:3":
		return "1440x1080"
	case "21:9":
		return "2520x1080"
	case "16:9":
		return "1920x1080"
	default:
		return ""
	}
}
