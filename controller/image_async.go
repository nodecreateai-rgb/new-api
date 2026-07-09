package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

var imageSensitiveURLPattern = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)
var imageSensitivePathPattern = regexp.MustCompile(`(?i)(?:/app/scratch|/tmp|/var/lib/docker|/root)/[^\s"'<>]+`)

const imagePublicUpstreamError = "upstream image service temporarily unavailable, please retry"

func sanitizeImageTaskPublicError(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	low := strings.ToLower(reason)
	if strings.Contains(low, "felo.ai") || strings.Contains(low, "file.felo.ai") ||
		strings.Contains(low, "api-proxy") || strings.Contains(low, "upload image") ||
		strings.Contains(low, "bad record mac") || strings.Contains(low, "tls:") ||
		strings.Contains(low, "/app/scratch") || strings.Contains(low, "remote error") ||
		strings.Contains(low, "client.timeout") || strings.Contains(low, "context deadline exceeded") ||
		strings.Contains(low, "upstream status=") || strings.Contains(low, "paco-felo2api") {
		return imagePublicUpstreamError
	}
	clean := imageSensitiveURLPattern.ReplaceAllString(reason, "[upstream]")
	clean = imageSensitivePathPattern.ReplaceAllString(clean, "[file]")
	low = strings.ToLower(clean)
	if strings.Contains(low, "felo") || strings.Contains(low, "api-proxy") || strings.Contains(low, "paco-felo2api") {
		return imagePublicUpstreamError
	}
	return clean
}

func imageAsyncRequested(req *dto.ImageRequest) bool {
	return req != nil && ((req.Async != nil && *req.Async) || (req.AsyncTask != nil && *req.AsyncTask) || (req.ReturnTaskID != nil && *req.ReturnTaskID))
}

func imageRequestHasReferences(req *dto.ImageRequest) bool {
	if req == nil {
		return false
	}
	for _, raw := range []any{req.Images, req.ImageURL, req.ImageURLs, req.Image, req.Mask} {
		if rawImageFieldHasValue(raw) {
			return true
		}
	}
	return false
}

func rawImageFieldHasValue(raw any) bool {
	switch v := raw.(type) {
	case nil:
		return false
	case []byte:
		return jsonRawHasValue(v)
	case string:
		return strings.TrimSpace(v) != ""
	default:
		b, err := common.Marshal(v)
		return err == nil && jsonRawHasValue(b)
	}
}

func jsonRawHasValue(raw []byte) bool {
	value := strings.TrimSpace(string(raw))
	return value != "" && value != "null" && value != "[]" && value != "{}" && value != `""`
}

func shouldRouteImageRequestToFelo(req *dto.ImageRequest) bool {
	return req != nil && strings.EqualFold(strings.TrimSpace(req.Model), "gpt-image-2") && imageRequestHasReferences(req)
}

func RelayImageAsync(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ImageRequest) *types.NewAPIError {
	meta := req.GetTokenCountMeta()
	tokens, err := service.EstimateRequestToken(c, meta, info)
	if err != nil {
		return types.NewError(err, types.ErrorCodeCountTokenFailed)
	}
	info.SetEstimatePromptTokens(tokens)
	priceData, err := helper.ModelPriceHelper(c, info, tokens, meta)
	if err != nil {
		return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
	}
	if priceData.Quota == 0 {
		priceData.Quota = priceData.QuotaToPreConsume
		info.PriceData = priceData
	}
	if !priceData.FreeModel {
		if apiErr := service.PreConsumeBilling(c, priceData.QuotaToPreConsume, info); apiErr != nil {
			return apiErr
		}
	}
	var bodyBytes []byte
	contentType := "application/json"
	var bodyErr error
	if c.GetBool(contextKeyChatImageCompat) {
		bodyBytes, bodyErr = common.Marshal(req)
	} else {
		bodyStorage, err := common.GetBodyStorage(c)
		if err != nil {
			bodyErr = err
		} else {
			bodyBytes, contentType, bodyErr = imageAsyncRequestBody(c, bodyStorage)
		}
	}
	if bodyErr != nil {
		if info.Billing != nil {
			info.Billing.Refund(c)
		}
		return types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	publicTaskID := model.GenerateTaskID()
	if info.ChannelMeta == nil {
		info.InitChannelMeta(c)
	}
	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	info.TaskRelayInfo.PublicTaskID = publicTaskID
	feloRoute := shouldRouteImageRequestToFelo(req)
	info.Action = imageActionFromPath(c.Request.URL.Path)
	if feloRoute {
		info.Action = "edit_image"
	}
	task := model.InitTask(constant.TaskPlatformImage, info)
	task.Action = info.Action
	task.Status = model.TaskStatusSubmitted
	task.Progress = "10%"
	task.SubmitTime = time.Now().Unix()
	task.Quota = priceData.QuotaToPreConsume
	task.Data = imageTaskInitialData(req)
	if info.Billing != nil {
		task.PrivateData.BillingSource = info.BillingSource
		task.PrivateData.SubscriptionId = info.SubscriptionId
		task.PrivateData.TokenId = info.TokenId
	}
	task.PrivateData.BillingContext = &model.TaskBillingContext{ModelPrice: priceData.ModelPrice, GroupRatio: priceData.GroupRatioInfo.GroupRatio, ModelRatio: priceData.ModelRatio, OtherRatios: priceData.OtherRatios, OriginModelName: info.OriginModelName, PerCallBilling: priceData.UsePrice}
	if err := task.Insert(); err != nil {
		if info.Billing != nil {
			info.Billing.Refund(c)
		}
		return types.NewErrorWithStatusCode(err, types.ErrorCodeBadResponse, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
	}
	service.LogTaskConsumption(c, info)

	go runImageAsyncTask(publicTaskID, c.GetInt("channel_id"), common.GetContextKeyString(c, constant.ContextKeyChannelKey), contentType, bodyBytes, feloRoute)
	c.JSON(http.StatusAccepted, imageAcceptedTaskResponse(task))
	return nil
}

func imageAsyncRequestBody(c *gin.Context, storage common.BodyStorage) ([]byte, string, error) {
	contentType := c.Request.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "multipart/form-data") {
		bodyBytes, err := storage.Bytes()
		return bodyBytes, contentType, err
	}
	form, err := c.MultipartForm()
	if err != nil {
		return nil, contentType, err
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for key, vals := range form.Value {
		for _, val := range vals {
			if err := writer.WriteField(key, val); err != nil {
				_ = writer.Close()
				return nil, contentType, err
			}
		}
	}
	for key, files := range form.File {
		for _, fh := range files {
			part, err := writer.CreateFormFile(key, fh.Filename)
			if err != nil {
				_ = writer.Close()
				return nil, contentType, err
			}
			file, err := fh.Open()
			if err != nil {
				_ = writer.Close()
				return nil, contentType, err
			}
			_, copyErr := io.Copy(part, file)
			_ = file.Close()
			if copyErr != nil {
				_ = writer.Close()
				return nil, contentType, copyErr
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, contentType, err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

func imageActionFromPath(path string) string {
	if strings.Contains(path, "edit") || strings.Contains(path, "edits") {
		return "edit_image"
	}
	if strings.Contains(path, "/videos") {
		return "generate_image_video"
	}
	return "generate_image"
}

func imageTaskInitialData(req *dto.ImageRequest) []byte {
	m := map[string]any{"model": req.Model, "prompt": req.Prompt, "size": req.Size, "quality": req.Quality, "aspect_ratio": req.AspectRatio}
	b, _ := common.Marshal(m)
	return b
}

func imageAcceptedTaskResponse(task *model.Task) map[string]any {
	return map[string]any{"id": task.TaskID, "task_id": task.TaskID, "taskId": task.TaskID, "object": "task", "kind": imageKindFromAction(task.Action), "status": "queued", "created": task.SubmitTime, "updated": time.Now().Unix(), "task_url": imageTaskURL(task)}
}

func runImageAsyncTask(publicTaskID string, channelID int, key string, contentType string, requestBody []byte, routeToFelo bool) {
	ctx := context.Background()
	task, exists, err := model.GetByOnlyTaskId(publicTaskID)
	if err != nil || !exists {
		return
	}
	preStatus := task.Status
	ch, err := model.CacheGetChannel(channelID)
	if err != nil {
		failImageTask(ctx, task, preStatus, fmt.Sprintf("get channel failed: %v", err))
		return
	}
	baseURL := ch.GetBaseURL()
	urlPath := "/v1/images/generations"
	if task.Action == "edit_image" && !routeToFelo {
		urlPath = "/v1/images/edits"
	}
	if routeToFelo {
		baseURL = imageFeloBaseURL()
		urlPath = "/v1/images/generations"
		key = ""
	}
	upstreamURL := strings.TrimRight(baseURL, "/") + urlPath
	task.Status = model.TaskStatusInProgress
	task.Progress = "30%"
	task.StartTime = time.Now().Unix()
	_, _ = task.UpdateWithStatus(preStatus)

	asyncBody, asyncContentType, err := ensureAsyncPayload(contentType, requestBody, routeToFelo)
	if err != nil {
		failImageTask(ctx, task, model.TaskStatusInProgress, err.Error())
		return
	}
	upReq, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(asyncBody))
	if err != nil {
		failImageTask(ctx, task, model.TaskStatusInProgress, err.Error())
		return
	}
	upReq.Header.Set("Content-Type", asyncContentType)
	if key != "" {
		upReq.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := doImageAsyncSubmit(upReq, routeToFelo)
	if err != nil {
		failImageTask(ctx, task, model.TaskStatusInProgress, err.Error())
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		failImageTask(ctx, task, model.TaskStatusInProgress, err.Error())
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		failImageTask(ctx, task, model.TaskStatusInProgress, fmt.Sprintf("upstream status=%d body=%s", resp.StatusCode, common.LocalLogPreview(string(body))))
		return
	}

	var parsed map[string]any
	if common.Unmarshal(body, &parsed) == nil {
		if upstreamTaskID := firstString(parsed["task_id"], parsed["taskId"], parsed["id"]); upstreamTaskID != "" && strings.EqualFold(asString(parsed["object"]), "task") {
			task.PrivateData.UpstreamTaskID = upstreamTaskID
			task.Data = body
			task.Progress = "30%"
			if won, err := task.UpdateWithStatus(model.TaskStatusInProgress); err != nil || !won {
				logger.LogError(ctx, fmt.Sprintf("image async upstream task save failed task=%s err=%v won=%v", publicTaskID, err, won))
			}
			return
		}
	}

	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	if common.Unmarshal(body, &parsed) == nil {
		normalized, resultURL := normalizeImageTaskResult(publicTaskID, parsed)
		if resultURL != "" {
			task.PrivateData.ResultURL = resultURL
		}
		if normalizedBody, err := common.Marshal(normalized); err == nil {
			task.Data = normalizedBody
		} else {
			task.Data = body
		}
	} else {
		task.Data = body
	}
	if won, err := task.UpdateWithStatus(model.TaskStatusInProgress); err != nil || !won {
		logger.LogError(ctx, fmt.Sprintf("image async update failed task=%s err=%v won=%v", publicTaskID, err, won))
	}
}

func doImageAsyncSubmit(req *http.Request, routeToFelo bool) (*http.Response, error) {
	attempts := 1
	if routeToFelo {
		attempts = 4
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !routeToFelo || !isRetryableImageSubmitError(err) || attempt == attempts {
			break
		}
		time.Sleep(time.Duration(attempt) * 750 * time.Millisecond)
	}
	return nil, lastErr
}

func isRetryableImageSubmitError(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())
	for _, marker := range []string{
		"lookup ", "server misbehaving", "no such host", "connection refused", "connection reset", "i/o timeout", "timeout", "temporary failure", "bad record mac", "tls:",
	} {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

func imageFeloBaseURL() string {
	if v := strings.TrimSpace(os.Getenv("IMAGE_FELO2API_BASE_URL")); v != "" {
		return v
	}
	return "http://paco-felo2api-yzexs0-felo2api-1:43188"
}

func imageFeloTaskBaseURL(task *model.Task, fallback string) string {
	if task != nil && task.PrivateData.UpstreamTaskID != "" && task.Action == "edit_image" {
		return imageFeloBaseURL()
	}
	return fallback
}

func ensureAsyncPayload(contentType string, body []byte, routeToFelo bool) ([]byte, string, error) {
	if strings.Contains(strings.ToLower(contentType), "multipart/form-data") {
		if routeToFelo {
			return feloJSONPayloadFromMultipart(contentType, body)
		}
		return body, contentType, nil
	}
	var m map[string]any
	if err := common.Unmarshal(body, &m); err != nil {
		return nil, contentType, err
	}
	m["async"] = true
	m["return_task_id"] = true
	m["response_format"] = "url"
	if routeToFelo {
		normalizeFeloImagePayload(m)
	}
	out, err := common.Marshal(m)
	return out, "application/json", err
}

func normalizeFeloImagePayload(m map[string]any) {
	if m == nil {
		return
	}
	m["model"] = "gpt-image-2"
	if _, ok := m["reference_images"]; !ok {
		for _, key := range []string{"image", "images", "image_url", "image_urls"} {
			if v, ok := m[key]; ok && rawImageFieldHasValue(v) {
				m["reference_images"] = normalizeFeloReferenceImages(v)
				break
			}
		}
	} else {
		m["reference_images"] = normalizeFeloReferenceImages(m["reference_images"])
	}
	delete(m, "image_url")
	delete(m, "image_urls")
	delete(m, "mask")
}

func normalizeFeloReferenceImages(v any) any {
	if !rawImageFieldHasValue(v) {
		return nil
	}
	switch typed := v.(type) {
	case []any:
		return typed
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []byte:
		var parsed any
		if common.Unmarshal(typed, &parsed) == nil {
			return normalizeFeloReferenceImages(parsed)
		}
		return []any{string(typed)}
	default:
		return []any{typed}
	}
}

func feloJSONPayloadFromMultipart(contentType string, body []byte) ([]byte, string, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, contentType, err
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, contentType, errors.New("multipart boundary missing")
	}
	form, err := multipart.NewReader(bytes.NewReader(body), boundary).ReadForm(32 << 20)
	if err != nil {
		return nil, contentType, err
	}
	defer form.RemoveAll()

	payload := make(map[string]any)
	if form != nil {
		for key, values := range form.Value {
			if len(values) == 0 {
				continue
			}
			if len(values) == 1 {
				payload[key] = values[0]
			} else {
				items := make([]any, 0, len(values))
				for _, value := range values {
					items = append(items, value)
				}
				payload[key] = items
			}
		}
		var refs []any
		for _, field := range []string{"image", "image[]", "images", "reference_images"} {
			for _, fh := range form.File[field] {
				dataURL, err := multipartFileDataURL(fh)
				if err != nil {
					return nil, contentType, err
				}
				refs = append(refs, dataURL)
			}
		}
		if len(refs) > 0 {
			payload["reference_images"] = refs
		}
	}
	normalizeFeloImagePayload(payload)
	out, err := common.Marshal(payload)
	return out, "application/json", err
}

func multipartFileDataURL(fh *multipart.FileHeader) (string, error) {
	file, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	mimeType := fh.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func failImageTask(ctx context.Context, task *model.Task, from model.TaskStatus, reason string) {
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.FailReason = sanitizeImageTaskPublicError(reason)
	won, err := task.UpdateWithStatus(from)
	if err != nil || !won {
		return
	}
	if task.Quota != 0 {
		service.RefundTaskQuota(ctx, task, reason)
	}
}

func ImageOrRelayTaskFetch(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		taskID = c.Param("id")
	}
	userID := c.GetInt("id")
	if task, exists, err := model.GetByTaskId(userID, taskID); err == nil && exists && task.Platform == constant.TaskPlatformImage {
		c.JSON(http.StatusOK, imageTaskResponse(task))
		return
	}
	RelayTaskFetch(c)
}

func ImageTaskFetch(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		taskID = c.Param("id")
	}
	userID := c.GetInt("id")
	task, exists, err := model.GetByTaskId(userID, taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, map[string]any{"error": "task_not_found"})
		return
	}
	if task.Platform != constant.TaskPlatformImage {
		c.JSON(http.StatusBadRequest, map[string]any{"error": "not_image_task"})
		return
	}
	c.JSON(http.StatusOK, imageTaskResponse(task))
}

func imageTaskResponse(task *model.Task) map[string]any {
	status := "running"
	switch task.Status {
	case model.TaskStatusSubmitted, model.TaskStatusQueued, model.TaskStatusNotStart:
		status = "queued"
	case model.TaskStatusInProgress:
		status = "running"
	case model.TaskStatusSuccess:
		status = "succeeded"
	case model.TaskStatusFailure:
		status = "failed"
	}
	out := map[string]any{"id": task.TaskID, "task_id": task.TaskID, "taskId": task.TaskID, "object": "task", "kind": imageKindFromAction(task.Action), "status": status, "progress": task.Progress, "created": task.SubmitTime, "updated": task.UpdatedAt, "task_url": imageTaskURL(task)}
	if task.FailReason != "" {
		out["error"] = sanitizeImageTaskPublicError(task.FailReason)
	}
	if task.Status == model.TaskStatusSuccess && len(task.Data) > 0 {
		var result map[string]any
		if common.Unmarshal(task.Data, &result) == nil {
			out["result"] = result
			if d, ok := result["data"]; ok {
				out["data"] = d
			} else if nested, ok := result["result"].(map[string]any); ok {
				if d, ok := nested["data"]; ok {
					out["data"] = d
				}
			}
		} else {
			out["result"] = string(task.Data)
		}
	}
	return out
}

func imageTaskURL(task *model.Task) string {
	if task != nil && task.Action == "edit_image" {
		return "/v1/images/edits/" + task.TaskID
	}
	return "/v1/images/generations/" + task.TaskID
}

func imageKindFromAction(action string) string {
	if action == "edit_image" {
		return "image_edit"
	}
	return "image_generation"
}

func firstString(vals ...any) string {
	for _, v := range vals {
		if s := asString(v); s != "" {
			return s
		}
	}
	return ""
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

type imageTaskAdaptor struct{}

func NewImageTaskAdaptor() service.TaskPollingAdaptor        { return &imageTaskAdaptor{} }
func (a *imageTaskAdaptor) Init(info *relaycommon.RelayInfo) {}
func (a *imageTaskAdaptor) FetchTask(baseURL string, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID := asString(body["task_id"])
	if taskID == "" {
		taskID = asString(body["id"])
	}
	if asString(body["action"]) == "edit_image" {
		baseURL = imageFeloBaseURL()
		key = ""
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/task/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	return http.DefaultClient.Do(req)
}
func (a *imageTaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var m map[string]any
	if err := common.Unmarshal(respBody, &m); err != nil {
		return nil, err
	}
	status := strings.ToLower(asString(m["status"]))
	taskInfo := &relaycommon.TaskInfo{TaskID: firstString(m["task_id"], m["taskId"], m["id"])}
	switch status {
	case "queued", "submitted", "pending":
		taskInfo.Status = string(model.TaskStatusQueued)
	case "running", "processing", "in_progress":
		taskInfo.Status = string(model.TaskStatusInProgress)
	case "succeeded", "success", "completed", "done":
		taskInfo.Status = string(model.TaskStatusSuccess)
	case "failed", "failure", "error":
		taskInfo.Status = string(model.TaskStatusFailure)
	default:
		return nil, fmt.Errorf("unknown image task status %q", status)
	}
	if p := asString(m["progress"]); p != "" {
		taskInfo.Progress = p
	}
	if taskInfo.Progress == "" {
		switch taskInfo.Status {
		case string(model.TaskStatusQueued):
			taskInfo.Progress = "20%"
		case string(model.TaskStatusInProgress):
			taskInfo.Progress = "30%"
		case string(model.TaskStatusSuccess), string(model.TaskStatusFailure):
			taskInfo.Progress = "100%"
		}
	}
	taskInfo.Reason = sanitizeImageTaskPublicError(firstString(m["error"], m["message"]))
	if b, err := common.Marshal(m); err == nil {
		taskInfo.RemoteUrl = string(b)
	}
	taskInfo.Url = extractImageResultURL(m)
	return taskInfo, nil
}
func (a *imageTaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	return 0
}

func extractImageResultURL(m map[string]any) string {
	if u := extractImageURLFromAny(m["data"]); u != "" {
		return u
	}
	if result, ok := m["result"].(map[string]any); ok {
		if u := extractImageURLFromAny(result["data"]); u != "" {
			return u
		}
	}
	return firstString(m["url"], m["result_url"], m["output_url"])
}

func extractImageURLFromAny(v any) string {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	first, ok := arr[0].(map[string]any)
	if !ok {
		return ""
	}
	return firstString(first["url"], first["output_url"])
}

func normalizeImageTaskResult(taskID string, payload map[string]any) (map[string]any, string) {
	resultURL := ""
	normalizeData := func(v any) any {
		arr, ok := v.([]any)
		if !ok {
			return v
		}
		for i, item := range arr {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if u := firstString(itemMap["url"], itemMap["output_url"]); u != "" {
				if resultURL == "" {
					resultURL = u
				}
				delete(itemMap, "b64_json")
				arr[i] = itemMap
				continue
			}
			if b64 := firstString(itemMap["b64_json"]); b64 != "" {
				if u, err := persistImageTaskOutput(taskID, i, b64); err == nil && u != "" {
					itemMap["url"] = u
					delete(itemMap, "b64_json")
					if resultURL == "" {
						resultURL = u
					}
					arr[i] = itemMap
				}
			}
		}
		return arr
	}
	if data, ok := payload["data"]; ok {
		payload["data"] = normalizeData(data)
	}
	if result, ok := payload["result"].(map[string]any); ok {
		if data, ok := result["data"]; ok {
			result["data"] = normalizeData(data)
			payload["result"] = result
		}
	}
	if resultURL == "" {
		resultURL = extractImageResultURL(payload)
	}
	return payload, resultURL
}

func persistImageTaskOutput(taskID string, index int, b64 string) (string, error) {
	b64 = strings.TrimSpace(b64)
	if strings.HasPrefix(b64, "data:") {
		comma := strings.IndexByte(b64, ',')
		if comma >= 0 {
			b64 = b64[comma+1:]
		}
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(b64)
		if err != nil {
			return "", err
		}
	}
	name := taskID
	if !strings.HasPrefix(name, "task_") {
		name = "task_" + name
	}
	if index > 0 {
		name = fmt.Sprintf("%s_%d", name, index+1)
	}
	fileName := name + imageOutputExt(data)
	outputDir := os.Getenv("IMAGE_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "/data/output"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(outputDir, fileName), data, 0644); err != nil {
		return "", err
	}
	return "/output/" + fileName, nil
}

func imageOutputExt(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return ".png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return ".jpg"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp"
	}
	return ".png"
}

var _ = errors.New
