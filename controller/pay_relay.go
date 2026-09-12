package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const payModelName = "pay"

func RelayPay(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "server_error",
			},
		})
		return
	}
	relayInfo.InitChannelMeta(c)
	relayInfo.OriginModelName = payModelName
	relayInfo.Action = constant.PayActionPay

	priceData, err := helper.ModelPriceHelperPerCall(c, relayInfo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}
	relayInfo.PriceData = priceData

	if !priceData.FreeModel {
		if apiErr := service.PreConsumeBilling(c, priceData.Quota, relayInfo); apiErr != nil {
			c.JSON(apiErr.StatusCode, gin.H{"error": apiErr.ToOpenAIError()})
			return
		}
	}

	billed := false
	defer func() {
		if !billed && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	bodyStorage, err := common.GetBodyStorage(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}
	bodyBytes, err := bodyStorage.Bytes()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	publicTaskID := model.GenerateTaskID()
	if relayInfo.TaskRelayInfo == nil {
		relayInfo.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	relayInfo.TaskRelayInfo.PublicTaskID = publicTaskID

	task := model.InitTask(constant.TaskPlatformPay, relayInfo)
	task.Action = constant.PayActionPay
	task.Status = model.TaskStatusSubmitted
	task.Progress = "10%"
	task.SubmitTime = time.Now().Unix()
	task.Quota = priceData.QuotaToPreConsume
	task.Data = payTaskInitialData(bodyBytes)
	if relayInfo.Billing != nil {
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
	}
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:      priceData.ModelPrice,
		GroupRatio:      priceData.GroupRatioInfo.GroupRatio,
		ModelRatio:      priceData.ModelRatio,
		OtherRatios:     priceData.OtherRatios,
		OriginModelName: relayInfo.OriginModelName,
		PerCallBilling:  priceData.UsePrice,
	}
	if err := task.Insert(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "server_error",
			},
		})
		return
	}
	service.LogTaskConsumption(c, relayInfo)
	billed = true

	go runPaySubmitTask(publicTaskID, c.GetInt("channel_id"), common.GetContextKeyString(c, constant.ContextKeyChannelKey), bodyBytes)
	c.JSON(http.StatusAccepted, payAcceptedTaskResponse(task))
}

func runPaySubmitTask(publicTaskID string, channelID int, key string, requestBody []byte) {
	ctx := context.Background()
	task, exists, err := model.GetByOnlyTaskId(publicTaskID)
	if err != nil || !exists {
		return
	}
	preStatus := task.Status
	ch, err := model.CacheGetChannel(channelID)
	if err != nil {
		failPayTask(ctx, task, preStatus, fmt.Sprintf("get channel failed: %v", err))
		return
	}
	baseURL := strings.TrimRight(ch.GetBaseURL(), "/")
	if baseURL == "" {
		failPayTask(ctx, task, preStatus, "pay upstream base url is not configured")
		return
	}
	upstreamURL := baseURL + "/v1/pay"

	task.Status = model.TaskStatusInProgress
	task.Progress = "30%"
	task.StartTime = time.Now().Unix()
	_, _ = task.UpdateWithStatus(preStatus)

	req, err := http.NewRequest(http.MethodPost, upstreamURL, bytes.NewReader(requestBody))
	if err != nil {
		failPayTask(ctx, task, model.TaskStatusInProgress, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		failPayTask(ctx, task, model.TaskStatusInProgress, fmt.Sprintf("pay upstream request failed: %s", err.Error()))
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		failPayTask(ctx, task, model.TaskStatusInProgress, err.Error())
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		failPayTask(ctx, task, model.TaskStatusInProgress, fmt.Sprintf("upstream status=%d body=%s", resp.StatusCode, common.LocalLogPreview(string(body))))
		return
	}

	var parsed map[string]any
	if common.Unmarshal(body, &parsed) != nil {
		failPayTask(ctx, task, model.TaskStatusInProgress, "invalid upstream response")
		return
	}
	upstreamTaskID := payUpstreamTaskID(parsed)
	if upstreamTaskID == "" {
		failPayTask(ctx, task, model.TaskStatusInProgress, "upstream task id missing")
		return
	}
	task.PrivateData.UpstreamTaskID = upstreamTaskID
	task.Data = body
	task.Progress = "30%"
	if won, err := task.UpdateWithStatus(model.TaskStatusInProgress); err != nil || !won {
		logger.LogError(ctx, fmt.Sprintf("pay upstream task save failed task=%s err=%v won=%v", publicTaskID, err, won))
	}
}

func failPayTask(ctx context.Context, task *model.Task, from model.TaskStatus, reason string) {
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.FailReason = reason
	won, err := task.UpdateWithStatus(from)
	if err != nil || !won {
		return
	}
	if task.Quota != 0 {
		service.RefundTaskQuota(ctx, task, reason)
	}
}

func PayOrRelayTaskFetch(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		taskID = c.Param("id")
	}
	userID := c.GetInt("id")
	if task, exists, err := model.GetByTaskId(userID, taskID); err == nil && exists && task.Platform == constant.TaskPlatformPay {
		c.JSON(http.StatusOK, payTaskResponse(task))
		return
	}
	ImageOrRelayTaskFetch(c)
}

func payTaskInitialData(body []byte) []byte {
	var m map[string]any
	if common.Unmarshal(body, &m) == nil {
		m["model"] = payModelName
		if b, err := common.Marshal(m); err == nil {
			return b
		}
	}
	return body
}

func payAcceptedTaskResponse(task *model.Task) map[string]any {
	return map[string]any{
		"id":         task.TaskID,
		"task_id":    task.TaskID,
		"object":     "task",
		"type":       constant.PayActionPay,
		"status":     "processing",
		"progress":   task.Progress,
		"created_at": task.SubmitTime,
		"updated_at": time.Now().Unix(),
	}
}

func payTaskResponse(task *model.Task) map[string]any {
	out := payAcceptedTaskResponse(task)
	out["updated_at"] = task.UpdatedAt
	if task.FinishTime > 0 {
		out["finished_at"] = task.FinishTime
	}
	switch task.Status {
	case model.TaskStatusSubmitted, model.TaskStatusQueued, model.TaskStatusNotStart:
		out["status"] = "processing"
	case model.TaskStatusInProgress:
		out["status"] = "processing"
	case model.TaskStatusSuccess:
		out["status"] = "completed"
	case model.TaskStatusFailure:
		out["status"] = "failed"
	}
	if task.Progress != "" {
		out["progress"] = strings.TrimSuffix(strings.TrimSpace(task.Progress), "%")
	}
	if task.FailReason != "" {
		out["error"] = map[string]any{"message": task.FailReason}
	}
	if len(task.Data) > 0 {
		var upstream map[string]any
		if common.Unmarshal(task.Data, &upstream) == nil {
			for k, v := range upstream {
				if k == "id" || k == "task_id" {
					continue
				}
				out[k] = v
			}
		}
	}
	return out
}

func payUpstreamTaskID(m map[string]any) string {
	id := asString(m["id"])
	if id != "" {
		return strings.TrimPrefix(id, "task_")
	}
	return strings.TrimPrefix(asString(m["task_id"]), "task_")
}

type payTaskAdaptor struct{}

func NewPayTaskAdaptor() service.TaskPollingAdaptor { return &payTaskAdaptor{} }

func (a *payTaskAdaptor) Init(info *relaycommon.RelayInfo) {}

func (a *payTaskAdaptor) FetchTask(baseURL, key string, body map[string]any, _ string) (*http.Response, error) {
	taskID := asString(body["task_id"])
	if taskID == "" {
		taskID = asString(body["id"])
	}
	taskID = strings.TrimPrefix(strings.TrimSpace(taskID), "task_")
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/task/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	return http.DefaultClient.Do(req)
}

func (a *payTaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var m map[string]any
	if err := common.Unmarshal(respBody, &m); err != nil {
		return nil, err
	}
	status := strings.ToLower(asString(m["status"]))
	taskInfo := &relaycommon.TaskInfo{TaskID: payUpstreamTaskID(m)}
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
		return nil, fmt.Errorf("unknown pay task status %q", status)
	}
	if p := payProgressString(m["progress"]); p != "" {
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
	taskInfo.Reason = payTaskErrorMessage(m)
	if b, err := common.Marshal(m); err == nil {
		taskInfo.RemoteUrl = string(b)
	}
	return taskInfo, nil
}

func (a *payTaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	return 0
}

func payProgressString(v any) string {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return ""
		}
		if strings.HasSuffix(s, "%") {
			return s
		}
		return s + "%"
	case float64:
		return fmt.Sprintf("%.0f%%", t)
	case int:
		return fmt.Sprintf("%d%%", t)
	case int64:
		return fmt.Sprintf("%d%%", t)
	default:
		return ""
	}
}

func payTaskErrorMessage(m map[string]any) string {
	if errObj, ok := m["error"].(map[string]any); ok {
		if msg := firstString(errObj["message"], errObj["error"]); msg != "" {
			return msg
		}
	}
	return firstString(m["error"], m["message"])
}
