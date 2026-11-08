package sora

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"time"

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

// jsonString accepts JSON strings or numbers so upstreams that echo
// `"seconds": 8` still parse into the Sora task DTO.
type jsonString string

func (s *jsonString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*s = ""
		return nil
	}
	if data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		*s = jsonString(str)
		return nil
	}
	*s = jsonString(string(data))
	return nil
}

func (s jsonString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

type responseTask struct {
	ID                 string     `json:"id"`
	TaskID             string     `json:"task_id,omitempty"` //兼容旧接口
	Object             string     `json:"object"`
	Model              string     `json:"model"`
	Status             string     `json:"status"`
	Progress           int        `json:"progress"`
	CreatedAt          int64      `json:"created_at"`
	CompletedAt        int64      `json:"completed_at,omitempty"`
	ExpiresAt          int64      `json:"expires_at,omitempty"`
	Seconds            jsonString `json:"seconds,omitempty"`
	Size               string     `json:"size,omitempty"`
	RemixedFromVideoID string     `json:"remixed_from_video_id,omitempty"`
	URL                string     `json:"url,omitempty"`
	VideoURL           string     `json:"video_url,omitempty"`
	OutputURL          string     `json:"output_url,omitempty"`
	DownloadURL        string     `json:"download_url,omitempty"`
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
	if taskErr := relaycommon.ValidateMultipartDirect(c, info); taskErr != nil {
		return taskErr
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err == nil && strings.EqualFold(strings.TrimSpace(info.OriginModelName), "kling-o3-omni") && len(collectUpstreamVideoAudioRefs(req)) > 0 {
		return &dto.TaskError{Code: "audio_reference_not_supported", Message: "audio reference is not supported", StatusCode: http.StatusBadRequest, LocalError: true, Error: fmt.Errorf("audio reference is not supported")}
	}
	if err == nil && isKlingO3Model(info.OriginModelName) {
		if imageCount := len(collectUpstreamVideoImageRefs(req)); imageCount > 2 {
			return &dto.TaskError{Code: "too_many_reference_images", Message: "kling-o3 supports at most 2 reference images", StatusCode: http.StatusBadRequest, LocalError: true, Error: fmt.Errorf("kling-o3 supports at most 2 reference images, got %d", imageCount)}
		}
	}
	if err == nil && isSeedanceModel(info.OriginModelName) && promptMentionsImageReference(req.Prompt) && !req.HasImage() {
		return &dto.TaskError{Code: "missing_reference_image", Message: "prompt references an image, but no image material was received; send image/image_url/reference_image/image_refs", StatusCode: http.StatusBadRequest, LocalError: true, Error: fmt.Errorf("missing reference image material")}
	}
	if err == nil {
		if taskErr := minimaxH3MaxReferenceImageError(info.OriginModelName, collectUpstreamVideoImageRefs(req)); taskErr != nil {
			return taskErr
		}
	}
	return nil
}

func minimaxH3MaxReferenceImageError(model string, imageRefs []string) *dto.TaskError {
	if !strings.EqualFold(strings.TrimSpace(model), "minimax-h3-max") || len(imageRefs) > 0 {
		return nil
	}
	return &dto.TaskError{
		Code:       "missing_reference_image",
		Message:    "至少上传一张参考图",
		StatusCode: http.StatusBadRequest,
		LocalError: true,
		Error:      fmt.Errorf("至少上传一张参考图"),
	}
}

func isSeedanceModel(model string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(model)), "seedance")
}

func isKlingO3Model(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "kling-o3")
}

var imageReferenceMentionRE = regexp.MustCompile(`(?i)(?:@?image\s*0*[1-9]\d*|参考图(?:片)?\s*0*[1-9]\d*|图片\s*0*[1-9]\d*)`)
var chineseImageReferenceRE = regexp.MustCompile(`(?i)@?(?:参考图(?:片)?|图片)\s*0*([1-9]\d*)`)
var plainImageReferenceRE = regexp.MustCompile(`(?i)(?:@?image)\s*0*([1-9]\d*)`)
var compactImageReferenceBoundaryRE = regexp.MustCompile(`(@Image[1-9]\d*)([^\s，。！？、；：,.!?;:])`)

func promptMentionsImageReference(prompt string) bool {
	return imageReferenceMentionRE.MatchString(prompt)
}

func canonicalizeImageReferencePrompt(prompt string, imageCount int) string {
	if imageCount <= 0 {
		return prompt
	}
	normalize := func(re *regexp.Regexp, text string) string {
		return re.ReplaceAllStringFunc(text, func(match string) string {
			parts := re.FindStringSubmatch(match)
			if len(parts) != 2 {
				return match
			}
			n, err := strconv.Atoi(parts[1])
			if err != nil || n < 1 || n > imageCount {
				return match
			}
			return fmt.Sprintf("@Image%d", n)
		})
	}
	prompt = normalize(chineseImageReferenceRE, prompt)
	prompt = normalize(plainImageReferenceRE, prompt)
	prompt = compactImageReferenceBoundaryRE.ReplaceAllString(prompt, "$1 $2")

	// Seedance treats uploaded style blobs as weak, unindexed context unless the
	// prompt explicitly binds each @ImageN handle. Preserve the user's wording,
	// but append only missing references so every uploaded image is semantically
	// available to the model instead of effectively falling back to Image1.
	present := make(map[int]bool, imageCount)
	for _, match := range plainImageReferenceRE.FindAllStringSubmatch(prompt, -1) {
		if len(match) != 2 {
			continue
		}
		if n, err := strconv.Atoi(match[1]); err == nil && n >= 1 && n <= imageCount {
			present[n] = true
		}
	}
	// Multi-reference Seedance requests need every handle to be explicit. Keep
	// singular requests unchanged for backward compatibility, and do not append a
	// contradictory binding when the prompt contains an out-of-range index.
	missing := make([]string, 0, imageCount-len(present))
	if imageCount > 1 && len(present) > 0 {
		for n := 1; n <= imageCount; n++ {
			if !present[n] {
				missing = append(missing, fmt.Sprintf("@Image%d", n))
			}
		}
	}
	if len(missing) > 0 {
		prompt = strings.TrimSpace(prompt) + "\n参考素材绑定（全部必须使用）：" + strings.Join(missing, " ")
	}
	return prompt
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
	if resolved := relaycommon.ResolveTaskSubmitDuration(req.Duration, req.Seconds); resolved > 0 {
		seconds = resolved
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
	case "sd2.5", "sd2-mini", "sd2-fast", "sd2-c6", "sd2-c7", "seedance-720", "seedance-2.0-mini", "seedance-2.0-mini-480p", "seedance-2.0-mini-720p", "seedance-2.0-mini-480p-c2", "seedance-2.0-mini-720p-c2", "seedance-2.0-fast", "seedance-2.0-fast-480p", "seedance-2.0-fast-480p-c1", "grok-imagine-720p", "seedance-2.0-fast-720p", "seedance-2.0-fast-720p-c1", "seedance-2.0-fast-720p-c2", "seedance-2.0-fast-720p-c3", "seedance-2.0-720p", "seedance-2.0-1080p", "seedance-2.0", "seedance-2.5", "seedance-2.0-480p", "seedance-2.5-480p", "seedance-2.5-720p", "seedance-2.5-1080p", "seedance-2.0-c1", "seedance-2.5-c1", "seedance-2.5-c2", "seedance-2.0-c2", "seedance-2.0-480p-c2", "seedance-2.0-fast-c2", "seedance-2.0-fast-480p-c2", "seedance-2.0-mini-c2", "wan-3.0-c2", "wan-3.0-480p-c2", "wan-3.0", "wan-3.0-c1", "wan-3.0-480p", "wan-3.0-480p-c3", "seedance-2.0-480p-c3", "seedance-2.0-mini-480p-c3", "seedance-2.0-fast-480p-c3", "seedance-2.5-c3", "minimax-h3", "minimax-h3-max", "minimax-h3-2k", "minimax-h3-768p", "seedance-2.5-720p-c4", "seedance-2.0-720p-c1", "seedance-2.0-720p-c2", "seedance-2.0-720p-c3", "seedance-2.5-720p-c5", "sora-2", "kling-o3", "kling-o3-omni", "wan3.0-video-prime":
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
			var parsed relaycommon.TaskSubmitReq
			if req, reqErr := relaycommon.GetTaskRequest(c); reqErr == nil {
				parsed = req
				if err := originModelMediaLimit(info.OriginModelName, parsed); err != nil {
					return nil, err
				}
				applyCanonicalVideoControls(bodyMap, parsed)
				applyOriginModelDurationLimit(bodyMap, parsed, info.OriginModelName)
				applyParsedReferences(bodyMap, parsed)
			}
			normalizeOpenAIVideoAspectBody(bodyMap)
			applyOriginModelResolution(bodyMap, info.OriginModelName)
			applyOriginModelAudio(bodyMap, info.OriginModelName)
			sanitizeOriginModelUpstreamBody(bodyMap, parsed, info.OriginModelName)
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

// applyParsedReferences copies parsed media onto the JSON body when the
// original payload used an alias the upstream create API does not read
// (image_refs, reference_image, audio_refs, …). Fields the upstream already
// accepts are left untouched.
func applyParsedReferences(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	if imgs := collectUpstreamVideoImageRefs(req); len(imgs) > 0 &&
		!bodyHasMedia(body, "image", "image_url", "images", "image_urls", "reference_images", "input_reference") {
		body["images"] = imgs
	}
	if vids := collectUpstreamVideoVideoRefs(req); len(vids) > 0 &&
		!bodyHasMedia(body, "video", "video_url", "videos", "video_urls", "reference_videos") {
		body["videos"] = vids
	}
	if auds := collectUpstreamVideoAudioRefs(req); len(auds) > 0 &&
		!bodyHasMedia(body, "audio", "audio_url", "audios", "audio_urls", "reference_audios") {
		body["audios"] = auds
	}
}

func bodyHasMedia(body map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		switch v := body[key].(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return true
			}
		case []string:
			if len(v) > 0 {
				return true
			}
		case []interface{}:
			if len(v) > 0 {
				return true
			}
		case map[string]interface{}:
			if len(v) > 0 {
				return true
			}
		}
	}
	return false
}

func applyCanonicalVideoControls(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	if req.PromptComplianceEnabled != nil {
		body["prompt_compliance_enabled"] = *req.PromptComplianceEnabled
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

func applySD2C7DurationLimit(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	duration := taskSubmitDuration(req)
	if duration <= 0 {
		return
	}
	if duration < 5 {
		duration = 5
	}
	if duration > 15 {
		duration = 15
	}
	body["duration"] = duration
	body["seconds"] = strconv.Itoa(duration)
}

func applySD25DurationLimit(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	duration := taskSubmitDuration(req)
	if duration <= 0 {
		// Leave duration unset so dola2api can infer clip length from the prompt
		// (e.g. "30 秒") instead of forcing the legacy 10s default.
		delete(body, "duration")
		delete(body, "seconds")
		return
	}
	if duration < 5 {
		duration = 5
	}
	if duration > 30 {
		duration = 30
	}
	body["duration"] = duration
	body["seconds"] = strconv.Itoa(duration)
}

func applyOriginModelDurationLimit(body map[string]interface{}, req relaycommon.TaskSubmitReq, originModel string) {
	switch strings.TrimSpace(originModel) {
	case "sd2.5":
		applySD25DurationLimit(body, req)
	case "sd2-c7":
		applySD2C7DurationLimit(body, req)
	case "seedance-2.5-c2":
		applySeedance25C2DurationLimit(body, req)
	case "seedance-2.0-mini-480p-c2":
		applyCatalogDurationLimit(body, req, 15)
	case "seedance-2.0-mini-720p":
		applyCatalogDurationLimit(body, req, 15)
	case "kling-o3-omni":
		applyCatalogDurationBounds(body, req, 5, 3, 12)
	case "seedance-2.0-480p", "seedance-2.0-fast-480p-c1", "seedance-2.0-fast-720p":
		applyCatalogDurationBounds(body, req, 5, 4, 15)
	case "seedance-2.0-fast-720p-c1", "seedance-2.0-fast-720p-c3":
		applyCatalogDurationChoices(body, req, 15, 5, 10, 15)
	case "seedance-2.0-fast-720p-c2", "seedance-2.0-720p-c2", "seedance-2.0-720p-c3":
		applyCatalogDurationBounds(body, req, 4, 4, 15)
	case "seedance-2.5-720p-c5":
		applyCatalogDurationBounds(body, req, 5, 5, 30)
	case "seedance-2.5-480p":
		applyCatalogDurationBounds(body, req, 5, 4, 30)
	case "seedance-2.0-mini-720p-c2", "seedance-2.0-mini-720P-c2":
		applyCatalogDurationLimit(body, req, 10)
	case "seedance-2.0-fast-480p":
		applyCatalogDurationRange(body, req, 10, 15)
	case "grok-imagine-720p":
		applyCatalogDurationLimit(body, req, 15)
	case "seedance-2.0-c2", "seedance-2.0-480p-c2", "seedance-2.0-fast-c2", "seedance-2.0-fast-480p-c2", "seedance-2.0-mini-c2":
		applyCatalogDurationLimit(body, req, 15)
	case "wan-3.0-c2":
		applyCatalogDurationLimit(body, req, 30)
	case "wan-3.0", "wan-3.0-c1":
		applyCatalogDurationBounds(body, req, 5, 2, 20)
	case "wan-3.0-480p":
		applyCatalogDurationBounds(body, req, 5, 2, 30)
	case "wan-3.0-480p-c3":
		applyCatalogDurationBounds(body, req, 5, 2, 15)
	case "seedance-2.0-480p-c3", "seedance-2.0-mini-480p-c3", "seedance-2.0-fast-480p-c3":
		applyCatalogDurationBounds(body, req, 5, 4, 15)
	case "seedance-2.5-c3":
		applyCatalogDurationBounds(body, req, 5, 4, 28)
	case "wan-3.0-480p-c2":
		applyCatalogDurationLimit(body, req, 20)
	case "minimax-h3":
		applyCatalogDurationLimit(body, req, 15)
	case "minimax-h3-768p":
		applyCatalogDurationBounds(body, req, 5, 4, 15)
	case "seedance-2.0-720p-c1":
		applyCatalogDurationBounds(body, req, 5, 5, 15)
	case "minimax-h3-2k", "minimax-h3-c3":
		applyCatalogDurationBounds(body, req, 5, 4, 15)
	case "seedance-2.5-720p-c4":
		applyCatalogDurationBounds(body, req, 5, 4, 30)
	case "seedance-2.5", "wan3.0-video-prime":
		applyCatalogDurationBounds(body, req, 5, 1, 30)
	case "minimax-h3-max":
		applyCatalogDurationBounds(body, req, 12, 5, 12)
	case "sora-2":
		applyCatalogDurationLimit(body, req, 12)
	case "seedance-2.0-720p":
		applyFixedCatalogDuration(body, 15)
	}
}

func applyOriginModelAudio(body map[string]interface{}, originModel string) {
	if body == nil {
		return
	}
	switch strings.TrimSpace(originModel) {
	case "seedance-2.0-fast-720p-c1", "seedance-2.0-fast-720p-c2", "seedance-2.0-fast-720p-c3", "seedance-2.0-720p-c1", "seedance-2.0-720p-c2", "seedance-2.0-720p-c3", "seedance-2.5-720p-c5":
		body["generate_audio"] = true
		body["generateAudio"] = true
	}
}

func sanitizeOriginModelUpstreamBody(body map[string]interface{}, req relaycommon.TaskSubmitReq, originModel string) {
	originModel = strings.TrimSpace(originModel)
	switch originModel {
	case "minimax-h3-768p":
		sanitizeMoyinH3VideoBody(body, req)
	case "seedance-2.0-720p-c1":
		sanitizeWanfeng720pC1VideoBody(body, req)
	case "seedance-2.0-fast-720p-c1":
		sanitizeOpenaifitFast720pC1VideoBody(body, req)
	case "seedance-2.0-fast-720p-c2":
		sanitizeXxzFast720pC2VideoBody(body, req)
	case "seedance-2.0-fast-720p-c3":
		sanitizeJm2apiFast720pC3VideoBody(body, req)
	case "seedance-2.0-720p-c2", "seedance-2.0-720p-c3", "seedance-2.5-720p-c5":
		sanitizeXingapi720pVideoBody(body, req, originModel)
	}
}

// sanitizeMoyinH3VideoBody keeps only fields MoyinAI /v1/videos accepts.
// Clients (and the shared Sora adaptor) send flova/OpenAI aliases such as
// image_refs, audio_refs, ratio, and compliance_enabled. MoyinAI treats those
// undeclared keys as a 400 "请求参数格式不正确".
func sanitizeMoyinH3VideoBody(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	images := collectUpstreamVideoImageRefs(req)
	if len(images) == 0 {
		images = collectBodyMedia(body,
			"images", "image_refs", "image_urls", "reference_images", "extra_images",
			"image", "image_url", "reference_image", "input_reference")
	}
	audios := collectUpstreamVideoAudioRefs(req)
	if len(audios) == 0 {
		audios = collectBodyMedia(body,
			"audios", "audio_refs", "audio_urls", "reference_audios", "extra_audios",
			"audio", "audio_url", "reference_audio")
	}
	keep := map[string]struct{}{
		"model": {}, "prompt": {}, "seconds": {}, "duration": {},
		"aspect_ratio": {}, "aspectRatio": {},
		"size": {}, "resolution": {},
		"images": {}, "audios": {},
		"metadata": {}, "parameters": {},
	}
	for key := range body {
		if _, ok := keep[key]; !ok {
			delete(body, key)
		}
	}
	if len(images) > 0 {
		body["images"] = images
	} else {
		delete(body, "images")
	}
	if len(audios) > 0 {
		body["audios"] = audios
	} else {
		delete(body, "audios")
	}
}

// sanitizeWanfeng720pC1VideoBody keeps the public OpenAI-video fields this
// 720p C1 route accepts, forces 9:16, and copies image/video/audio aliases.
func sanitizeWanfeng720pC1VideoBody(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	images := collectUpstreamVideoImageRefs(req)
	if len(images) == 0 {
		images = collectBodyMedia(body,
			"images", "image_refs", "image_urls", "reference_images", "extra_images",
			"image", "image_url", "reference_image", "input_reference")
	}
	videos := collectUpstreamVideoVideoRefs(req)
	if len(videos) == 0 {
		videos = collectBodyMedia(body,
			"videos", "video_refs", "video_urls", "reference_videos", "extra_videos",
			"video", "video_url", "reference_video")
	}
	audios := collectUpstreamVideoAudioRefs(req)
	if len(audios) == 0 {
		audios = collectBodyMedia(body,
			"audios", "audio_refs", "audio_urls", "reference_audios", "extra_audios",
			"audio", "audio_url", "reference_audio")
	}
	keep := map[string]struct{}{
		"model": {}, "prompt": {}, "seconds": {}, "duration": {},
		"aspect_ratio": {}, "resolution": {}, "size": {},
		"images": {}, "videos": {}, "audios": {},
		"generateAudio": {}, "generate_audio": {},
		"first_image": {}, "last_image": {},
		"metadata": {}, "parameters": {},
	}
	for key := range body {
		if _, ok := keep[key]; !ok {
			delete(body, key)
		}
	}
	body["aspect_ratio"] = "9:16"
	body["resolution"] = "720p"
	body["size"] = "720p"
	if _, ok := body["generateAudio"]; !ok {
		if _, hasSnake := body["generate_audio"]; !hasSnake {
			body["generateAudio"] = true
		}
	}
	if len(images) > 0 {
		body["images"] = images
	} else {
		delete(body, "images")
	}
	if len(videos) > 0 {
		body["videos"] = videos
	} else {
		delete(body, "videos")
	}
	if len(audios) > 0 {
		body["audios"] = audios
	} else {
		delete(body, "audios")
	}
}

// sanitizeOpenaifitFast720pC1VideoBody keeps only fields this 720p Fast C1
// route accepts. The upstream rejects unknown keys and duration/aspect aliases
// sent together (seconds+duration, ratio+aspect_ratio+size).
func sanitizeOpenaifitFast720pC1VideoBody(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	images := collectUpstreamVideoImageRefs(req)
	if len(images) == 0 {
		images = collectBodyMedia(body,
			"images", "image_refs", "image_urls", "reference_images", "extra_images",
			"image", "image_url", "reference_image", "input_reference")
	}
	videos := collectUpstreamVideoVideoRefs(req)
	if len(videos) == 0 {
		videos = collectBodyMedia(body,
			"videos", "video_refs", "video_urls", "reference_videos", "extra_videos",
			"video", "video_url", "reference_video")
	}
	audios := collectUpstreamVideoAudioRefs(req)
	if len(audios) == 0 {
		audios = collectBodyMedia(body,
			"audios", "audio_refs", "audio_urls", "reference_audios", "extra_audios",
			"audio", "audio_url", "reference_audio")
	}
	ratio := ""
	for _, key := range []string{"ratio", "aspect_ratio", "aspect"} {
		if s, ok := body[key].(string); ok {
			s = strings.TrimSpace(s)
			if s != "" {
				ratio = s
				break
			}
		}
	}
	if ratio == "" {
		ratio = strings.TrimSpace(req.AspectRatio)
	}
	switch ratio {
	case "16:9", "9:16", "1:1", "4:3", "3:4":
	default:
		ratio = "16:9"
	}
	model, _ := body["model"].(string)
	prompt, _ := body["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		prompt = strings.TrimSpace(req.Prompt)
	}
	seconds := body["seconds"]
	for key := range body {
		delete(body, key)
	}
	if strings.TrimSpace(model) != "" {
		body["model"] = model
	}
	if strings.TrimSpace(prompt) != "" {
		body["prompt"] = prompt
	}
	if seconds != nil {
		body["seconds"] = seconds
	}
	body["resolution"] = "720p"
	body["ratio"] = ratio
	if len(images) > 0 {
		body["image_urls"] = images
		body["images"] = images
	}
	if len(videos) > 0 {
		body["videos"] = videos
		body["video_urls"] = videos
	}
	if len(audios) > 0 {
		body["audios"] = audios
		body["audio_urls"] = audios
	}
}

// sanitizeXxzFast720pC2VideoBody keeps OpenAI-video fields this 720p Fast C2
// route accepts. Do not send size (WxH) together with ratio, or duration
// together with seconds. Videos are dropped: this public alias allows none.
func sanitizeXxzFast720pC2VideoBody(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	images := collectUpstreamVideoImageRefs(req)
	if len(images) == 0 {
		images = collectBodyMedia(body,
			"images", "image_refs", "image_urls", "reference_images", "extra_images",
			"image", "image_url", "reference_image", "input_reference")
	}
	audios := collectUpstreamVideoAudioRefs(req)
	if len(audios) == 0 {
		audios = collectBodyMedia(body,
			"audios", "audio_refs", "audio_urls", "reference_audios", "extra_audios",
			"audio", "audio_url", "reference_audio")
	}
	ratio := ""
	for _, key := range []string{"ratio", "aspect_ratio", "aspect"} {
		if s, ok := body[key].(string); ok {
			s = strings.TrimSpace(s)
			if s != "" {
				ratio = s
				break
			}
		}
	}
	if ratio == "" {
		ratio = strings.TrimSpace(req.AspectRatio)
	}
	switch ratio {
	case "16:9", "9:16", "1:1", "4:3", "3:4", "21:9":
	default:
		ratio = "16:9"
	}
	model, _ := body["model"].(string)
	prompt, _ := body["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		prompt = strings.TrimSpace(req.Prompt)
	}
	secs := req.Duration
	if secs <= 0 {
		switch v := body["seconds"].(type) {
		case string:
			secs, _ = strconv.Atoi(strings.TrimSpace(v))
		case float64:
			secs = int(v)
		case int:
			secs = v
		}
	}
	if secs <= 0 {
		if v, ok := body["duration"].(float64); ok {
			secs = int(v)
		}
	}
	if secs < 4 {
		secs = 5
	} else if secs > 15 {
		secs = 15
	}
	for key := range body {
		delete(body, key)
	}
	content := []map[string]interface{}{}
	if strings.TrimSpace(prompt) != "" {
		content = append(content, map[string]interface{}{"type": "text", "text": prompt})
	}
	for _, u := range images {
		content = append(content, map[string]interface{}{
			"type": "image_url", "role": "reference_image",
			"image_url": map[string]interface{}{"url": u},
		})
	}
	for _, u := range audios {
		content = append(content, map[string]interface{}{
			"type": "audio_url", "role": "reference_audio",
			"audio_url": map[string]interface{}{"url": u},
		})
	}
	if strings.TrimSpace(model) != "" {
		body["model"] = model
	}
	body["content"] = content
	if strings.TrimSpace(prompt) != "" {
		body["prompt"] = prompt
	}
	body["duration"] = secs
	body["seconds"] = secs
	body["ratio"] = ratio
	body["resolution"] = "720p"
}

// sanitizeJm2apiFast720pC3VideoBody keeps OpenAI-video fields the jm2api 720p
// Fast route accepts: 720p, 5/10/15s, 9 images, 3 videos, 3 audios. C2 then
// drops video refs to match its public cap.
func sanitizeJm2apiFast720pC3VideoBody(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	images := collectUpstreamVideoImageRefs(req)
	if len(images) == 0 {
		images = collectBodyMedia(body,
			"images", "image_refs", "image_urls", "reference_images", "extra_images",
			"image", "image_url", "reference_image", "input_reference")
	}
	videos := collectUpstreamVideoVideoRefs(req)
	if len(videos) == 0 {
		videos = collectBodyMedia(body,
			"videos", "video_refs", "video_urls", "reference_videos", "extra_videos",
			"video", "video_url", "reference_video")
	}
	audios := collectUpstreamVideoAudioRefs(req)
	if len(audios) == 0 {
		audios = collectBodyMedia(body,
			"audios", "audio_refs", "audio_urls", "reference_audios", "extra_audios",
			"audio", "audio_url", "reference_audio")
	}
	ratio := ""
	for _, key := range []string{"ratio", "aspect_ratio", "aspect"} {
		if s, ok := body[key].(string); ok {
			s = strings.TrimSpace(s)
			if s != "" {
				ratio = s
				break
			}
		}
	}
	if ratio == "" {
		ratio = strings.TrimSpace(req.AspectRatio)
	}
	switch ratio {
	case "16:9", "9:16", "1:1", "4:3", "3:4", "21:9":
	default:
		ratio = "16:9"
	}
	model, _ := body["model"].(string)
	prompt, _ := body["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		prompt = strings.TrimSpace(req.Prompt)
	}
	secs := taskSubmitDuration(req)
	if secs <= 0 {
		switch v := body["seconds"].(type) {
		case string:
			secs, _ = strconv.Atoi(strings.TrimSpace(v))
		case float64:
			secs = int(v)
		case int:
			secs = v
		}
	}
	if secs <= 0 {
		switch v := body["duration"].(type) {
		case float64:
			secs = int(v)
		case int:
			secs = v
		case string:
			secs, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	choices := []int{5, 10, 15}
	if secs <= 0 {
		secs = 15
	} else {
		best := choices[0]
		bestDist := secs - best
		if bestDist < 0 {
			bestDist = -bestDist
		}
		for _, choice := range choices[1:] {
			dist := secs - choice
			if dist < 0 {
				dist = -dist
			}
			if dist < bestDist || (dist == bestDist && choice > best) {
				best = choice
				bestDist = dist
			}
		}
		secs = best
	}
	for key := range body {
		delete(body, key)
	}
	if strings.TrimSpace(model) != "" {
		body["model"] = model
	}
	if strings.TrimSpace(prompt) != "" {
		body["prompt"] = prompt
	}
	body["seconds"] = secs
	body["duration"] = secs
	body["resolution"] = "720p"
	body["ratio"] = ratio
	body["aspect_ratio"] = ratio
	body["generate_audio"] = true
	if len(images) > 0 {
		body["images"] = images
		body["image_urls"] = images
		body["image_refs"] = images
		body["reference_images"] = images
	}
	if len(videos) > 0 {
		body["videos"] = videos
		body["video_urls"] = videos
		body["video_refs"] = videos
		body["reference_videos"] = videos
	}
	if len(audios) > 0 {
		body["audios"] = audios
		body["audio_urls"] = audios
		body["audio_refs"] = audios
		body["reference_audios"] = audios
	}
}

// sanitizeXingapi720pVideoBody maps public image_refs/audio_refs/video_refs
// onto the OpenAI-video fields this 720p C2 route's upstream accepts.
func sanitizeXingapi720pVideoBody(body map[string]interface{}, req relaycommon.TaskSubmitReq, originModel string) {
	if body == nil {
		return
	}
	originModel = strings.TrimSpace(originModel)
	images := collectUpstreamVideoImageRefs(req)
	if len(images) == 0 {
		images = collectBodyMedia(body,
			"images", "image_refs", "image_urls", "reference_images", "extra_images",
			"image", "image_url", "reference_image", "input_reference")
	}
	videos := collectUpstreamVideoVideoRefs(req)
	if len(videos) == 0 {
		videos = collectBodyMedia(body,
			"videos", "video_refs", "video_urls", "reference_videos", "extra_videos",
			"video", "video_url", "reference_video")
	}
	audios := collectUpstreamVideoAudioRefs(req)
	if len(audios) == 0 {
		audios = collectBodyMedia(body,
			"audios", "audio_refs", "audio_urls", "reference_audios", "extra_audios",
			"audio", "audio_url", "reference_audio")
	}
	if originModel == "seedance-2.0-720p-c2" {
		videos = nil
	}
	ratio := ""
	for _, key := range []string{"ratio", "aspect_ratio", "aspect"} {
		if s, ok := body[key].(string); ok {
			s = strings.TrimSpace(s)
			if s != "" {
				ratio = s
				break
			}
		}
	}
	if ratio == "" {
		ratio = strings.TrimSpace(req.AspectRatio)
	}
	if originModel == "seedance-2.0-720p-c3" {
		ratio = "9:16"
	} else {
		switch ratio {
		case "16:9", "9:16", "1:1", "4:3", "3:4", "21:9":
		default:
			ratio = "16:9"
		}
	}
	model, _ := body["model"].(string)
	prompt, _ := body["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		prompt = strings.TrimSpace(req.Prompt)
	}
	secs := taskSubmitDuration(req)
	if secs <= 0 {
		switch v := body["seconds"].(type) {
		case string:
			secs, _ = strconv.Atoi(strings.TrimSpace(v))
		case float64:
			secs = int(v)
		case int:
			secs = v
		}
	}
	if secs <= 0 {
		switch v := body["duration"].(type) {
		case float64:
			secs = int(v)
		case int:
			secs = v
		case string:
			secs, _ = strconv.Atoi(strings.TrimSpace(v))
		}
	}
	minSecs := 4
	maxSecs := 15
	if originModel == "seedance-2.5-720p-c5" {
		minSecs = 5
		maxSecs = 30
	}
	if secs < minSecs {
		secs = minSecs
	} else if secs > maxSecs {
		secs = maxSecs
	}
	for key := range body {
		delete(body, key)
	}
	if strings.TrimSpace(model) != "" {
		body["model"] = model
	}
	if strings.TrimSpace(prompt) != "" {
		body["prompt"] = prompt
	}
	// This route's 2.5 model rejects string seconds and a sibling duration
	// field. Send a JSON number only. 2.5 also floors at 5s.
	body["seconds"] = secs
	body["resolution"] = "720p"
	body["ratio"] = ratio
	body["generate_audio"] = true
	if len(images) > 0 {
		body["images"] = images
		body["image_urls"] = images
	}
	if len(videos) > 0 {
		body["videos"] = videos
		body["video_urls"] = videos
	}
	if len(audios) > 0 {
		body["audios"] = audios
		body["audio_urls"] = audios
	}
}

func collectBodyMedia(body map[string]interface{}, keys ...string) []string {
	if body == nil {
		return nil
	}
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
	for _, key := range keys {
		switch v := body[key].(type) {
		case string:
			add(v)
		case []string:
			for _, item := range v {
				add(item)
			}
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					add(s)
				}
			}
		}
	}
	return out
}

func applyOriginModelResolution(body map[string]interface{}, originModel string) {
	if body == nil {
		return
	}
	switch strings.TrimSpace(originModel) {
	case "seedance-2.0-480p", "seedance-2.0-fast-480p-c1", "seedance-2.5-480p", "seedance-2.0-mini-480p-c2", "seedance-2.0-fast-480p", "seedance-2.0-480p-c2", "seedance-2.0-fast-480p-c2", "wan-3.0-480p-c2", "wan-3.0", "wan-3.0-c1", "wan-3.0-480p", "wan-3.0-480p-c3", "seedance-2.0-480p-c3", "seedance-2.0-mini-480p-c3", "seedance-2.0-fast-480p-c3", "seedance-2.5-c3", "seedance-2.5":
		body["resolution"] = "480p"
		body["size"] = "480p"
	case "seedance-2.0-720p", "seedance-2.5-720p", "seedance-2.5-720p-c4", "seedance-2.5-720p-c5", "seedance-2.0-mini-720p", "seedance-2.0-mini-720p-c2", "seedance-2.0-mini-720P-c2", "grok-imagine-720p", "sora-2", "seedance-2.0-fast-720p", "seedance-2.0-fast-720p-c1", "seedance-2.0-fast-720p-c2", "seedance-2.0-fast-720p-c3", "seedance-2.0-720p-c1", "seedance-2.0-720p-c2", "seedance-2.0-720p-c3", "wan3.0-video-prime":
		body["resolution"] = "720p"
		body["size"] = "720p"
	case "minimax-h3-2k", "minimax-h3-c3":
		body["resolution"] = "2K"
		body["size"] = "2K"
	case "minimax-h3", "minimax-h3-max", "minimax-h3-768p":
		body["resolution"] = "768p"
		body["size"] = "768p"
	case "seedance-2.5-1080p":
		body["resolution"] = "1080p"
		body["size"] = "1080p"
	}
}

func applySeedance25C2DurationLimit(body map[string]interface{}, req relaycommon.TaskSubmitReq) {
	if body == nil {
		return
	}
	duration := taskSubmitDuration(req)
	if duration <= 0 {
		delete(body, "duration")
		delete(body, "seconds")
		return
	}
	if duration > 15 {
		duration = 15
	}
	body["duration"] = duration
	body["seconds"] = strconv.Itoa(duration)
}

func applyFixedCatalogDuration(body map[string]interface{}, seconds int) {
	if body == nil || seconds <= 0 {
		return
	}
	body["duration"] = seconds
	body["seconds"] = strconv.Itoa(seconds)
}

func applyCatalogDurationBounds(body map[string]interface{}, req relaycommon.TaskSubmitReq, fallback, min, max int) {
	if body == nil || max <= 0 {
		return
	}
	duration := taskSubmitDuration(req)
	if duration <= 0 {
		duration = fallback
	}
	if min > 0 && duration < min {
		duration = min
	}
	if duration > max {
		duration = max
	}
	if duration <= 0 {
		return
	}
	body["duration"] = duration
	body["seconds"] = strconv.Itoa(duration)
}

func applyCatalogDurationRange(body map[string]interface{}, req relaycommon.TaskSubmitReq, fallback, max int) {
	if body == nil || max <= 0 {
		return
	}
	duration := taskSubmitDuration(req)
	if duration <= 0 {
		duration = fallback
	}
	if duration > max {
		duration = max
	}
	if duration <= 0 {
		return
	}
	body["duration"] = duration
	body["seconds"] = strconv.Itoa(duration)
}

func applyCatalogDurationLimit(body map[string]interface{}, req relaycommon.TaskSubmitReq, max int) {
	applyCatalogDurationRange(body, req, max, max)
}

func applyCatalogDurationChoices(body map[string]interface{}, req relaycommon.TaskSubmitReq, fallback int, choices ...int) {
	if body == nil || len(choices) == 0 {
		return
	}
	duration := taskSubmitDuration(req)
	if duration <= 0 {
		duration = fallback
	}
	best := choices[0]
	bestDist := duration - best
	if bestDist < 0 {
		bestDist = -bestDist
	}
	for _, choice := range choices[1:] {
		dist := duration - choice
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist || (dist == bestDist && choice > best) {
			best = choice
			bestDist = dist
		}
	}
	if best <= 0 {
		return
	}
	body["duration"] = best
	body["seconds"] = strconv.Itoa(best)
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
	return doRestartWindowFetch(client, req)
}

func doRestartWindowFetch(client *http.Client, req *http.Request) (*http.Response, error) {
	const maxAttempts = 15
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		current := req
		if attempt > 1 {
			current = req.Clone(req.Context())
		}
		resp, err := client.Do(current)
		if err == nil {
			return resp, nil
		}
		if attempt == maxAttempts || !channel.IsRestartWindowTransportError(err) {
			return nil, err
		}
		delay := time.Duration(attempt) * time.Second
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
		}
	}
	return nil, fmt.Errorf("task fetch failed after restart-window retries")
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

	switch normalizeSoraVideoStatus(resTask.Status) {
	case dto.VideoStatusQueued:
		taskResult.Status = model.TaskStatusQueued
	case dto.VideoStatusInProgress:
		taskResult.Status = model.TaskStatusInProgress
	case dto.VideoStatusCompleted:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Url = firstNonEmptyVideoURL(resTask.VideoURL, resTask.URL, resTask.OutputURL, resTask.DownloadURL)
	case dto.VideoStatusFailed:
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

func firstNonEmptyVideoURL(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "data:video/") {
			return value
		}
	}
	return ""
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
	if usesFast480pStyleResponse(task.Properties.OriginModelName) {
		data = buildFast480pStyleVideo(task, status, data)
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
		"thumbnail_url", "thumb_url", "file_url", "fileUrl", "file_thumb_url",
	} {
		delete(payload, key)
	}
	for key, value := range payload {
		switch typed := value.(type) {
		case string:
			payload[key] = common.MaskPublicTaskText(key, typed)
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
	contentURL := task.PublicResultURL()
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
		if raw := strings.TrimSpace(firstFormValue(formData.Value, "prompt_compliance_enabled")); raw != "" {
			if enabled, convErr := strconv.ParseBool(raw); convErr == nil {
				req.PromptComplianceEnabled = &enabled
			}
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
	if err := originModelMediaLimit(info.OriginModelName, req); err != nil {
		return nil, err
	}
	bodyMap := taskSubmitReqToUpstreamVideoBody(req, info.UpstreamModelName)
	applyOriginModelDurationLimit(bodyMap, req, info.OriginModelName)
	normalizeOpenAIVideoAspectBody(bodyMap)
	applyOriginModelResolution(bodyMap, info.OriginModelName)
	applyOriginModelAudio(bodyMap, info.OriginModelName)
	sanitizeOriginModelUpstreamBody(bodyMap, req, info.OriginModelName)
	return common.Marshal(bodyMap)
}

func firstFormValue(values map[string][]string, key string) string {
	if xs := values[key]; len(xs) > 0 {
		return xs[0]
	}
	return ""
}

func taskSubmitReqToUpstreamVideoBody(req relaycommon.TaskSubmitReq, upstreamModel string) map[string]interface{} {
	imageRefs := collectUpstreamVideoImageRefs(req)
	body := map[string]interface{}{
		"model":  upstreamModel,
		"prompt": canonicalizeImageReferencePrompt(req.Prompt, len(imageRefs)),
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
	if req.PromptComplianceEnabled != nil {
		body["prompt_compliance_enabled"] = *req.PromptComplianceEnabled
	}
	if req.ComplianceEnabled != nil {
		body["compliance_enabled"] = *req.ComplianceEnabled
	}
	if complianceMode := strings.TrimSpace(req.ComplianceMode); complianceMode != "" {
		body["compliance_mode"] = complianceMode
	}
	if len(imageRefs) > 0 {
		body["image_refs"] = imageRefs
		body["images"] = imageRefs
		body["image_urls"] = imageRefs
		body["reference_images"] = imageRefs
		// Keep the legacy scalar alias only for a genuinely singular request.
		// Multi-reference upstreams already consume image_refs; repeating the
		// first item in image_url makes non-deduplicating backends upload it twice.
		if len(imageRefs) == 1 {
			body["image_url"] = imageRefs[0]
		}
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
	return relaycommon.ResolveTaskSubmitDuration(req.Duration, req.Seconds)
}

func collectUpstreamVideoImageRefs(req relaycommon.TaskSubmitReq) []string {
	return collectUpstreamVideoRefs(
		[]string{req.Image, req.InputReference, req.ImageURL, req.ReferenceImage},
		req.ImageRefs, req.ImageURLs, req.Images, req.ReferenceImages, req.ExtraImages,
	)
}

// originModelMediaLimit enforces per-model reference caps.
// Resolution and duration are applied separately.
func originModelMediaLimit(origin string, req relaycommon.TaskSubmitReq) error {
	switch strings.TrimSpace(origin) {
	case "seedance-2.0-mini-720p", "seedance-2.0-fast-480p-c1", "seedance-2.0-fast-720p", "seedance-2.0-480p":
		if len(collectUpstreamVideoVideoRefs(req)) > 0 {
			return fmt.Errorf("this model accepts at most 0 reference videos")
		}
		if len(collectUpstreamVideoImageRefs(req)) > 9 {
			return fmt.Errorf("this model accepts at most 9 reference images")
		}
		if len(collectUpstreamVideoAudioRefs(req)) > 3 {
			return fmt.Errorf("this model accepts at most 3 reference audios")
		}
	case "wan-3.0-480p", "wan3.0-video-prime":
		if len(collectUpstreamVideoVideoRefs(req)) > 0 {
			return fmt.Errorf("this model accepts at most 0 reference videos")
		}
		if len(collectUpstreamVideoImageRefs(req)) > 10 {
			return fmt.Errorf("this model accepts at most 10 reference images")
		}
		if len(collectUpstreamVideoAudioRefs(req)) > 5 {
			return fmt.Errorf("this model accepts at most 5 reference audios")
		}
	case "seedance-2.5":
		if len(collectUpstreamVideoVideoRefs(req)) > 0 {
			return fmt.Errorf("this model accepts at most 0 reference videos")
		}
		if len(collectUpstreamVideoImageRefs(req)) > 30 {
			return fmt.Errorf("this model accepts at most 30 reference images")
		}
		if len(collectUpstreamVideoAudioRefs(req)) > 10 {
			return fmt.Errorf("this model accepts at most 10 reference audios")
		}
	case "minimax-h3-768p":
		if len(collectUpstreamVideoVideoRefs(req)) > 0 {
			return fmt.Errorf("this model accepts at most 0 reference videos")
		}
		if len(collectUpstreamVideoImageRefs(req)) > 9 {
			return fmt.Errorf("this model accepts at most 9 reference images")
		}
		if len(collectUpstreamVideoAudioRefs(req)) > 3 {
			return fmt.Errorf("this model accepts at most 3 reference audios")
		}
	case "seedance-2.0-720p-c1":
		if len(collectUpstreamVideoImageRefs(req)) > 9 {
			return fmt.Errorf("this model accepts at most 9 reference images")
		}
		if len(collectUpstreamVideoVideoRefs(req)) > 3 {
			return fmt.Errorf("this model accepts at most 3 reference videos")
		}
		if len(collectUpstreamVideoAudioRefs(req)) > 3 {
			return fmt.Errorf("this model accepts at most 3 reference audios")
		}
	case "seedance-2.0-fast-720p-c1", "seedance-2.0-fast-720p-c3":
		if len(collectUpstreamVideoImageRefs(req)) > 9 {
			return fmt.Errorf("this model accepts at most 9 reference images")
		}
		if len(collectUpstreamVideoVideoRefs(req)) > 3 {
			return fmt.Errorf("this model accepts at most 3 reference videos")
		}
		if len(collectUpstreamVideoAudioRefs(req)) > 3 {
			return fmt.Errorf("this model accepts at most 3 reference audios")
		}
	case "seedance-2.0-fast-720p-c2":
		if len(collectUpstreamVideoVideoRefs(req)) > 0 {
			return fmt.Errorf("this model accepts at most 0 reference videos")
		}
		images := collectUpstreamVideoImageRefs(req)
		if len(images) > 9 {
			return fmt.Errorf("this model accepts at most 9 reference images")
		}
		audios := collectUpstreamVideoAudioRefs(req)
		if len(audios) > 3 {
			return fmt.Errorf("this model accepts at most 3 reference audios")
		}
		if len(audios) > 0 && len(images) == 0 {
			return fmt.Errorf("this model requires at least 1 reference image when using reference audio")
		}
	case "seedance-2.0-720p-c2":
		if len(collectUpstreamVideoVideoRefs(req)) > 0 {
			return fmt.Errorf("this model accepts at most 0 reference videos")
		}
		if len(collectUpstreamVideoImageRefs(req)) > 9 {
			return fmt.Errorf("this model accepts at most 9 reference images")
		}
		if len(collectUpstreamVideoAudioRefs(req)) > 3 {
			return fmt.Errorf("this model accepts at most 3 reference audios")
		}
	case "seedance-2.0-720p-c3":
		if len(collectUpstreamVideoImageRefs(req)) > 9 {
			return fmt.Errorf("this model accepts at most 9 reference images")
		}
		if len(collectUpstreamVideoVideoRefs(req)) > 3 {
			return fmt.Errorf("this model accepts at most 3 reference videos")
		}
		if len(collectUpstreamVideoAudioRefs(req)) > 3 {
			return fmt.Errorf("this model accepts at most 3 reference audios")
		}
	case "seedance-2.5-720p-c5":
		if len(collectUpstreamVideoImageRefs(req)) > 30 {
			return fmt.Errorf("this model accepts at most 30 reference images")
		}
		if len(collectUpstreamVideoVideoRefs(req)) > 10 {
			return fmt.Errorf("this model accepts at most 10 reference videos")
		}
		if len(collectUpstreamVideoAudioRefs(req)) > 10 {
			return fmt.Errorf("this model accepts at most 10 reference audios")
		}
	}
	return nil
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

// usesFast480pStyleResponse lists third-party C2/C3 models whose upstream query
// payload (nested data.*, videoprep_ ids, upstream stage text) must be rebuilt
// into the same flat shape seedance-2.0-fast-480p returns.
func usesFast480pStyleResponse(model string) bool {
	switch model {
	case "seedance-2.0-fast-720p-c2", "seedance-2.0-720p-c2", "seedance-2.0-720p-c3", "seedance-2.5-720p-c5":
		return true
	}
	return false
}

func buildFast480pStyleVideo(task *model.Task, status string, raw []byte) []byte {
	resolution := "720p"
	if strings.Contains(task.Properties.OriginModelName, "480p") {
		resolution = "480p"
	}
	duration := int64(0)
	if bc := task.PrivateData.BillingContext; bc != nil && bc.OtherRatios != nil {
		duration = int64(bc.OtherRatios["seconds"])
	}
	for _, k := range []string{"seconds", "duration", "data.seconds", "data.duration"} {
		if duration > 0 {
			break
		}
		duration = gjson.GetBytes(raw, k).Int()
	}
	aspect := ""
	for _, k := range []string{"aspect_ratio", "ratio", "data.aspect_ratio", "data.ratio"} {
		if v := gjson.GetBytes(raw, k).String(); v != "" {
			aspect = v
			break
		}
	}
	created := task.SubmitTime
	if created == 0 {
		created = task.CreatedAt
	}
	progress := 0
	if n, err := strconv.Atoi(strings.TrimSuffix(task.Progress, "%")); err == nil {
		progress = n
	}
	out := map[string]any{
		"id":           task.TaskID,
		"task_id":      task.TaskID,
		"object":       "task",
		"type":         "video",
		"model":        task.Properties.OriginModelName,
		"status":       status,
		"progress":     progress,
		"resolution":   resolution,
		"created_at":   created,
		"submitted_at": created,
		"updated_at":   task.UpdatedAt,
	}
	if duration > 0 {
		out["duration"] = duration
	}
	if aspect != "" {
		out["aspect_ratio"] = aspect
	}
	if task.FinishTime > 0 {
		out["finished_at"] = task.FinishTime
	}
	if status == dto.VideoStatusFailed {
		out["error"] = map[string]any{"message": common.PublicErrorMessage(task.FailReason)}
	}
	b, err := common.Marshal(out)
	if err != nil {
		return raw
	}
	return b
}
