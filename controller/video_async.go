package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

type videoUpstreamJob struct {
	PublicTaskID string
	UpstreamURL  string
	ContentType  string
	RequestBody  []byte
	APIKey       string
	Proxy        string
	UserID       int
	Username     string
	Compliance   service.ComplianceMaterial
}

func relayVideoSubmitAsync(c *gin.Context, relayInfo *relaycommon.RelayInfo) {
	var result *relay.TaskSubmitResult
	var taskErr *dto.TaskError
	accepted := false
	defer func() {
		if !accepted && taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		if taskErr != nil {
			respondTaskError(c, taskErr)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	var channel *model.Channel
	if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
		channel = lockedCh
		if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
			taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
			return
		}
	} else {
		var channelErr *types.NewAPIError
		channel, channelErr = getTaskChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", http.StatusInternalServerError)
			return
		}
	}
	addUsedChannel(c, channel.Id)
	bodyStorage, bodyErr := common.GetBodyStorage(c)
	if bodyErr != nil {
		if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
			taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
		} else {
			taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
		}
		return
	}
	c.Request.Body = io.NopCloser(bodyStorage)

	if relayInfo.TaskRelayInfo == nil {
		relayInfo.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	relayInfo.DeferUpstreamSubmit = true
	result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
	if taskErr != nil {
		return
	}

	task := model.InitTask(result.Platform, relayInfo)
	task.PrivateData.PendingUpstream = true
	task.PrivateData.BillingSource = relayInfo.BillingSource
	task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
	task.PrivateData.TokenId = relayInfo.TokenId
	task.PrivateData.Key = common.GetContextKeyString(c, constant.ContextKeyChannelKey)
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:      relayInfo.PriceData.ModelPrice,
		GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio:      relayInfo.PriceData.ModelRatio,
		OtherRatios:     relayInfo.PriceData.OtherRatios,
		OriginModelName: relayInfo.OriginModelName,
		PerCallBilling:  !common.IsPerSecondTaskModel(relayInfo.OriginModelName),
	}
	task.Quota = result.Quota
	task.Action = relayInfo.Action
	task.Status = model.TaskStatusSubmitted
	task.Progress = "10%"
	task.Data = model.JSONRaw(queuedOpenAIVideoJSON(task, relayInfo))
	if result.Platform != constant.TaskPlatformImage && result.Platform != constant.TaskPlatformPay {
		var raw []byte
		if bs, err := common.GetBodyStorage(c); err == nil {
			raw, _ = bs.Bytes()
		}
		task.PrivateData.Upscaler = service.ParseUpscalerOption(raw, func(k string) string {
			if c.Request.MultipartForm != nil {
				if v := c.Request.MultipartForm.Value[k]; len(v) > 0 {
					return v[0]
				}
			}
			return ""
		})
	}
	if insertErr := task.Insert(); insertErr != nil {
		taskErr = service.TaskErrorWrapper(insertErr, "insert_task_failed", http.StatusInternalServerError)
		return
	}
	if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
		common.SysError("settle task billing error: " + settleErr.Error())
	}
	service.LogTaskConsumption(c, relayInfo)
	accepted = true

	complianceOpt := service.ComplianceMaterial{}
	if service.CompliancePassthrough(relayInfo.OriginModelName) {
		complianceOpt.Passthrough = true
	} else {
		complianceOpt, _, _ = service.InspectComplianceFromContext(c)
	}
	job := videoUpstreamJob{
		PublicTaskID: task.TaskID,
		UpstreamURL:  result.UpstreamURL,
		ContentType:  result.ContentType,
		RequestBody:  result.RequestBody,
		APIKey:       task.PrivateData.Key,
		UserID:       relayInfo.UserId,
		Username:     common.GetContextKeyString(c, constant.ContextKeyUserName),
		Compliance:   complianceOpt,
	}
	if channel != nil {
		if full, err := model.CacheGetChannel(channel.Id); err == nil && full != nil {
			channel = full
		}
		job.Proxy = channel.GetSetting().Proxy
		if job.APIKey == "" {
			job.APIKey = channel.Key
		}
	}
	gopool.Go(func() { runVideoUpstreamSubmit(job) })

	otherRatios := relayInfo.PriceData.OtherRatios
	if otherRatios == nil {
		otherRatios = map[string]float64{}
	}
	if ratiosJSON, err := common.Marshal(otherRatios); err == nil {
		c.Header("X-New-Api-Other-Ratios", string(ratiosJSON))
	}
	c.JSON(http.StatusOK, queuedOpenAIVideoObject(task, relayInfo))
}

func queuedOpenAIVideoObject(task *model.Task, info *relaycommon.RelayInfo) *dto.OpenAIVideo {
	ov := dto.NewOpenAIVideo()
	ov.ID = task.TaskID
	ov.TaskID = task.TaskID
	ov.Model = info.OriginModelName
	ov.Status = dto.VideoStatusQueued
	ov.CreatedAt = task.SubmitTime
	ov.SetProgressStr(task.Progress)
	if info.PriceData.OtherRatios != nil {
		if seconds := info.PriceData.OtherRatios["seconds"]; seconds > 0 {
			ov.Seconds = formatVideoSeconds(seconds)
		}
	}
	return ov
}

func queuedOpenAIVideoJSON(task *model.Task, info *relaycommon.RelayInfo) []byte {
	body, err := common.Marshal(queuedOpenAIVideoObject(task, info))
	if err != nil {
		return []byte(`{"object":"video","status":"queued"}`)
	}
	return body
}

func formatVideoSeconds(seconds float64) string {
	if seconds == float64(int(seconds)) {
		return strconv.Itoa(int(seconds))
	}
	return strconv.FormatFloat(seconds, 'f', -1, 64)
}

func runVideoUpstreamSubmit(job videoUpstreamJob) {
	ctx := context.Background()
	task, exists, err := model.GetByOnlyTaskId(job.PublicTaskID)
	if err != nil || !exists {
		logger.LogError(ctx, "video async submit missing task "+job.PublicTaskID)
		return
	}
	preStatus := task.Status
	task.Status = model.TaskStatusInProgress
	task.Progress = "20%"
	task.StartTime = time.Now().Unix()
	if _, err := task.UpdateWithStatus(preStatus); err != nil {
		logger.LogError(ctx, "video async submit status update failed: "+err.Error())
	}

	treated, treatedType, treatErr := service.ApplyComplianceMaterials(ctx, job.RequestBody, job.ContentType, job.Compliance)
	if treatErr != nil {
		failVideoTask(ctx, task, model.TaskStatusInProgress, treatErr.Error())
		return
	}
	job.RequestBody = treated
	if treatedType != "" {
		job.ContentType = treatedType
	}

	resp, err := doVideoUpstreamCreate(ctx, job)
	if err != nil {
		failVideoTask(ctx, task, model.TaskStatusInProgress, err.Error())
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		failVideoTask(ctx, task, model.TaskStatusInProgress, err.Error())
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		failVideoTask(ctx, task, model.TaskStatusInProgress, string(body))
		return
	}
	upstreamID := relay.ParseUpstreamVideoTaskID(body)
	if upstreamID == "" {
		failVideoTask(ctx, task, model.TaskStatusInProgress, "upstream task id missing")
		return
	}

	var payload map[string]any
	if common.Unmarshal(body, &payload) != nil {
		payload = map[string]any{}
	}
	payload["id"] = job.PublicTaskID
	payload["task_id"] = job.PublicTaskID
	if task.Properties.OriginModelName != "" {
		payload["model"] = task.Properties.OriginModelName
	}
	clientBody, err := common.Marshal(payload)
	if err != nil {
		clientBody = body
	}
	task.PrivateData.PendingUpstream = false
	task.PrivateData.UpstreamTaskID = upstreamID
	task.Data = model.JSONRaw(clientBody)
	task.Status = model.TaskStatusInProgress
	task.Progress = "30%"
	if won, err := task.UpdateWithStatus(model.TaskStatusInProgress); err != nil || !won {
		logger.LogError(ctx, "video async upstream id save failed for "+job.PublicTaskID)
	}
}

func doVideoUpstreamCreate(ctx context.Context, job videoUpstreamJob) (*http.Response, error) {
	client, err := service.GetHttpClientWithProxy(job.Proxy)
	if err != nil {
		return nil, err
	}
	const maxAttempts = 15
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, job.UpstreamURL, bytes.NewReader(job.RequestBody))
		if reqErr != nil {
			return nil, reqErr
		}
		if job.ContentType != "" {
			req.Header.Set("Content-Type", job.ContentType)
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
		if job.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+job.APIKey)
		}
		if job.UserID > 0 {
			req.Header.Set("X-New-API-User-ID", strconv.Itoa(job.UserID))
		}
		if job.Username != "" {
			req.Header.Set("X-New-API-Username", job.Username)
		}
		resp, doErr := client.Do(req)
		if doErr == nil {
			return resp, nil
		}
		lastErr = doErr
		if attempt == maxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("video upstream create failed")
	}
	return nil, lastErr
}

func failVideoTask(ctx context.Context, task *model.Task, from model.TaskStatus, reason string) {
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.PrivateData.PendingUpstream = false
	task.FailReason = common.PublicErrorMessage(reason)
	won, err := task.UpdateWithStatus(from)
	if err != nil || !won {
		return
	}
	if task.Quota != 0 {
		service.RefundTaskQuota(ctx, task, reason)
	}
}
