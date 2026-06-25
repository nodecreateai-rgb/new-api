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
			AspectRatio:    firstFormValue(formData.Value, "aspect_ratio"),
			Seconds:        firstFormValue(formData.Value, "seconds"),
			Image:          firstFormValue(formData.Value, "image"),
			InputReference: firstFormValue(formData.Value, "input_reference"),
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
	if size := strings.TrimSpace(req.Size); size != "" {
		body["size"] = size
	}
	if aspect := strings.TrimSpace(req.AspectRatio); aspect != "" {
		body["aspect_ratio"] = aspect
	}
	if imageRefs := collectUpstreamVideoImageRefs(req); len(imageRefs) > 0 {
		body["image_refs"] = imageRefs
		body["image_url"] = imageRefs[0]
	}
	if audioRefs := collectUpstreamVideoAudioRefs(req); len(audioRefs) > 0 {
		body["audio_refs"] = audioRefs
		body["audio_url"] = audioRefs[0]
	}
	normalizeOpenAIVideoAspectBody(body)
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
	seen := make(map[string]struct{})
	out := make([]string, 0, len(req.Images)+len(req.ImageRefs)+len(req.ImageURLs)+3)
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
	add(req.Image)
	add(req.InputReference)
	add(req.ReferenceImage)
	for _, image := range req.ImageRefs {
		add(image)
	}
	for _, image := range req.ImageURLs {
		add(image)
	}
	for _, image := range req.Images {
		add(image)
	}
	return out
}

func collectUpstreamVideoAudioRefs(req relaycommon.TaskSubmitReq) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(req.AudioRefs)+len(req.AudioURLs)+len(req.Audios)+2)
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
	add(req.Audio)
	add(req.ReferenceAudio)
	for _, audio := range req.AudioRefs {
		add(audio)
	}
	for _, audio := range req.AudioURLs {
		add(audio)
	}
	for _, audio := range req.Audios {
		add(audio)
	}
	return out
}

func normalizeOpenAIVideoAspectBody(body map[string]interface{}) {
	if body == nil {
		return
	}
	size, _ := body["size"].(string)
	aspect, _ := body["aspect_ratio"].(string)
	prompt, _ := body["prompt"].(string)
	aspect, size = normalizeOpenAIVideoAspectValuesWithPrompt(aspect, size, prompt)
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
	aspect, size := normalizeOpenAIVideoAspectValuesWithPrompt(first("aspect_ratio"), first("size"), first("prompt"))
	if aspect != "" {
		out["aspect_ratio"] = []string{aspect}
	}
	if size != "" {
		out["size"] = []string{size}
	}
	return out
}

func normalizeOpenAIVideoAspectValues(aspect, size string) (string, string) {
	return normalizeOpenAIVideoAspectValuesWithPrompt(aspect, size, "")
}

func normalizeOpenAIVideoAspectValuesWithPrompt(aspect, size, prompt string) (string, string) {
	aspect = strings.TrimSpace(strings.ToLower(aspect))
	size = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(size, "*", "x")))
	if aspect == "" {
		aspect = aspectFromOpenAIVideoSize(size)
	}
	if aspect == "" {
		aspect = aspectFromPrompt(prompt)
	}
	if size == "" || isAspectRatioToken(size) {
		size = defaultOpenAIVideoSizeForAspect(aspect)
	}
	return aspect, size
}

func aspectFromPrompt(prompt string) string {
	p := strings.ToLower(strings.TrimSpace(prompt))
	if p == "" {
		return ""
	}
	if strings.Contains(p, "9:16") || strings.Contains(p, "竖屏") || strings.Contains(p, "纵向") || strings.Contains(p, "vertical") || strings.Contains(p, "portrait") {
		return "9:16"
	}
	if strings.Contains(p, "16:9") || strings.Contains(p, "横屏") || strings.Contains(p, "横向") || strings.Contains(p, "landscape") {
		return "16:9"
	}
	return ""
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
