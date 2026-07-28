package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/abema/go-mp4"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/samber/lo"
)

// TaskPollingAdaptor 定义轮询所需的最小适配器接口，避免 service -> relay 的循环依赖
type TaskPollingAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	FetchTask(baseURL string, key string, body map[string]any, proxy string) (*http.Response, error)
	ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error)
	// AdjustBillingOnComplete 在任务到达终态（成功/失败）时由轮询循环调用。
	// 返回正数触发差额结算（补扣/退还），返回 0 保持预扣费金额不变。
	AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int
}

// GetTaskAdaptorFunc 由 main 包注入，用于获取指定平台的任务适配器。
// 打破 service -> relay -> relay/channel -> service 的循环依赖。
var GetTaskAdaptorFunc func(platform constant.TaskPlatform) TaskPollingAdaptor

// sweepTimedOutTasks 在主轮询之前独立清理超时任务。
// 每次最多处理 100 条，剩余的下个周期继续处理。
// 使用 per-task CAS (UpdateWithStatus) 防止覆盖被正常轮询已推进的任务。
func sweepTimedOutTasks(ctx context.Context) {
	if constant.TaskTimeoutMinutes <= 0 {
		return
	}
	cutoff := time.Now().Unix() - int64(constant.TaskTimeoutMinutes)*60
	tasks := model.GetTimedOutUnfinishedTasks(cutoff, 100)
	if len(tasks) == 0 {
		return
	}

	const legacyTaskCutoff int64 = 1740182400 // 2026-02-22 00:00:00 UTC
	reason := fmt.Sprintf("任务超时（%d分钟）", constant.TaskTimeoutMinutes)
	legacyReason := "任务超时（旧系统遗留任务，不进行退款，请联系管理员）"
	now := time.Now().Unix()
	timedOutCount := 0

	for _, task := range tasks {
		isLegacy := task.SubmitTime > 0 && task.SubmitTime < legacyTaskCutoff

		oldStatus := task.Status
		task.Status = model.TaskStatusFailure
		task.Progress = "100%"
		task.FinishTime = now
		if isLegacy {
			task.FailReason = legacyReason
		} else {
			task.FailReason = reason
		}

		won, err := task.UpdateWithStatus(oldStatus)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("sweepTimedOutTasks CAS update error for task %s: %v", task.TaskID, err))
			continue
		}
		if !won {
			logger.LogInfo(ctx, fmt.Sprintf("sweepTimedOutTasks: task %s already transitioned, skip", task.TaskID))
			continue
		}
		timedOutCount++
		if !isLegacy && task.Quota != 0 {
			RefundTaskQuota(ctx, task, reason)
		}
	}

	if timedOutCount > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("sweepTimedOutTasks: timed out %d tasks", timedOutCount))
	}
}

const taskPollingInterval = 3 * time.Second

var taskPollingJobs sync.Map

func startTaskPollingJob(key string, job func()) bool {
	if _, loaded := taskPollingJobs.LoadOrStore(key, struct{}{}); loaded {
		return false
	}
	go func() {
		defer taskPollingJobs.Delete(key)
		job()
	}()
	return true
}

// TaskPollingLoop 主轮询循环，每 3 秒检查一次未完成的任务
func TaskPollingLoop() {
	for {
		time.Sleep(taskPollingInterval)
		common.SysLog("任务进度轮询开始")
		ctx := context.TODO()
		sweepTimedOutTasks(ctx)
		allTasks := model.GetAllUnFinishSyncTasks(constant.TaskQueryLimit)
		platformTask := make(map[constant.TaskPlatform][]*model.Task)
		for _, t := range allTasks {
			platformTask[t.Platform] = append(platformTask[t.Platform], t)
		}
		var platformWG sync.WaitGroup
		for platform, tasks := range platformTask {
			if len(tasks) == 0 {
				continue
			}
			platform, tasks := platform, tasks
			platformWG.Add(1)
			go func() {
				defer platformWG.Done()
				taskChannelM := make(map[int][]string)
				taskM := make(map[string]*model.Task)
				nullTaskIds := make([]int64, 0)
				for _, task := range tasks {
					upstreamID := task.GetUpstreamTaskID()
					if upstreamID == "" {
						// 统计失败的未完成任务
						nullTaskIds = append(nullTaskIds, task.ID)
						continue
					}
					taskM[upstreamID] = task
					taskChannelM[task.ChannelId] = append(taskChannelM[task.ChannelId], upstreamID)
				}
				if len(nullTaskIds) > 0 {
					err := model.TaskBulkUpdateByID(nullTaskIds, map[string]any{
						"status":   "FAILURE",
						"progress": "100%",
					})
					if err != nil {
						logger.LogError(ctx, fmt.Sprintf("Fix null task_id task error: %v", err))
					} else {
						logger.LogInfo(ctx, fmt.Sprintf("Fix null task_id task success: %v", nullTaskIds))
					}
				}
				if len(taskChannelM) == 0 {
					return
				}

				DispatchPlatformUpdate(platform, taskChannelM, taskM)
			}()
		}
		platformWG.Wait()
		common.SysLog("任务进度轮询完成")
	}
}

// DispatchPlatformUpdate 按平台分发轮询更新
func DispatchPlatformUpdate(platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) {
	switch platform {
	case constant.TaskPlatformMidjourney:
		// MJ 轮询由其自身处理，这里预留入口
	case constant.TaskPlatformSuno:
		_ = UpdateSunoTasks(context.Background(), taskChannelM, taskM)
	case constant.TaskPlatformImage:
		if err := UpdateImageTasks(context.Background(), taskChannelM, taskM); err != nil {
			common.SysLog(fmt.Sprintf("UpdateImageTasks fail: %s", err))
		}
	default:
		if err := UpdateVideoTasks(context.Background(), platform, taskChannelM, taskM); err != nil {
			common.SysLog(fmt.Sprintf("UpdateVideoTasks fail: %s", err))
		}
	}
}

// UpdateSunoTasks 按渠道更新所有 Suno 任务
func UpdateSunoTasks(ctx context.Context, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	for channelId, taskIds := range taskChannelM {
		channelId, taskIds := channelId, taskIds
		startTaskPollingJob(fmt.Sprintf("suno:%d", channelId), func() {
			if err := updateSunoTasks(ctx, channelId, taskIds, taskM); err != nil {
				logger.LogError(ctx, fmt.Sprintf("渠道 #%d 更新异步任务失败: %s", channelId, err.Error()))
			}
		})
	}
	return nil
}

func updateSunoTasks(ctx context.Context, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, fmt.Sprintf("渠道 #%d 未完成的任务有: %d", channelId, len(taskIds)))
	if len(taskIds) == 0 {
		return nil
	}
	ch, err := model.CacheGetChannel(channelId)
	if err != nil {
		common.SysLog(fmt.Sprintf("CacheGetChannel: %v", err))
		// Collect DB primary key IDs for bulk update (taskIds are upstream IDs, not task_id column values)
		var failedIDs []int64
		for _, upstreamID := range taskIds {
			if t, ok := taskM[upstreamID]; ok {
				failedIDs = append(failedIDs, t.ID)
			}
		}
		err = model.TaskBulkUpdateByID(failedIDs, map[string]any{
			"fail_reason": fmt.Sprintf("获取渠道信息失败，请联系管理员，渠道ID：%d", channelId),
			"status":      "FAILURE",
			"progress":    "100%",
		})
		if err != nil {
			common.SysLog(fmt.Sprintf("UpdateSunoTask error: %v", err))
		}
		return err
	}
	adaptor := GetTaskAdaptorFunc(constant.TaskPlatformSuno)
	if adaptor == nil {
		return errors.New("adaptor not found")
	}
	proxy := ch.GetSetting().Proxy
	resp, err := adaptor.FetchTask(*ch.BaseURL, ch.Key, map[string]any{
		"ids": taskIds,
	}, proxy)
	if err != nil {
		common.SysLog(fmt.Sprintf("Get Task Do req error: %v", err))
		return err
	}
	if resp.StatusCode != http.StatusOK {
		logger.LogError(ctx, fmt.Sprintf("Get Task status code: %d", resp.StatusCode))
		return fmt.Errorf("Get Task status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		common.SysLog(fmt.Sprintf("Get Suno Task parse body error: %v", err))
		return err
	}
	var responseItems dto.TaskResponse[[]dto.SunoDataResponse]
	err = common.Unmarshal(responseBody, &responseItems)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Get Suno Task parse body error2: %v, body: %s", err, string(responseBody)))
		return err
	}
	if !responseItems.IsSuccess() {
		common.SysLog(fmt.Sprintf("渠道 #%d 未完成的任务有: %d, 成功获取到任务数: %s", channelId, len(taskIds), string(responseBody)))
		return err
	}

	for _, responseItem := range responseItems.Data {
		task := taskM[responseItem.TaskID]
		if !taskNeedsUpdate(task, responseItem) {
			continue
		}

		task.Status = lo.If(model.TaskStatus(responseItem.Status) != "", model.TaskStatus(responseItem.Status)).Else(task.Status)
		task.FailReason = lo.If(responseItem.FailReason != "", responseItem.FailReason).Else(task.FailReason)
		task.SubmitTime = lo.If(responseItem.SubmitTime != 0, responseItem.SubmitTime).Else(task.SubmitTime)
		task.StartTime = lo.If(responseItem.StartTime != 0, responseItem.StartTime).Else(task.StartTime)
		task.FinishTime = lo.If(responseItem.FinishTime != 0, responseItem.FinishTime).Else(task.FinishTime)
		if responseItem.FailReason != "" || task.Status == model.TaskStatusFailure {
			logger.LogInfo(ctx, task.TaskID+" 构建失败，"+task.FailReason)
			task.Progress = "100%"
			RefundTaskQuota(ctx, task, task.FailReason)
		}
		if responseItem.Status == model.TaskStatusSuccess {
			task.Progress = "100%"
		}
		task.Data = responseItem.Data

		err = task.Update()
		if err != nil {
			common.SysLog("UpdateSunoTask task error: " + err.Error())
		}
	}
	return nil
}

// taskNeedsUpdate 检查 Suno 任务是否需要更新
func taskNeedsUpdate(oldTask *model.Task, newTask dto.SunoDataResponse) bool {
	if oldTask.SubmitTime != newTask.SubmitTime {
		return true
	}
	if oldTask.StartTime != newTask.StartTime {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if string(oldTask.Status) != newTask.Status {
		return true
	}
	if oldTask.FailReason != newTask.FailReason {
		return true
	}

	if (oldTask.Status == model.TaskStatusFailure || oldTask.Status == model.TaskStatusSuccess) && oldTask.Progress != "100%" {
		return true
	}

	oldData, _ := common.Marshal(oldTask.Data)
	newData, _ := common.Marshal(newTask.Data)

	sort.Slice(oldData, func(i, j int) bool {
		return oldData[i] < oldData[j]
	})
	sort.Slice(newData, func(i, j int) bool {
		return newData[i] < newData[j]
	})

	if string(oldData) != string(newData) {
		return true
	}
	return false
}

// UpdateImageTasks 按渠道更新所有图片异步任务。
func UpdateImageTasks(ctx context.Context, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	return UpdateVideoTasks(ctx, constant.TaskPlatformImage, taskChannelM, taskM)
}

// UpdateVideoTasks 按渠道更新所有视频任务
func UpdateVideoTasks(ctx context.Context, platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	for channelId, taskIds := range taskChannelM {
		channelId, taskIds := channelId, taskIds
		go func() {
			if err := updateVideoTasks(ctx, platform, channelId, taskIds, taskM); err != nil {
				logger.LogError(ctx, fmt.Sprintf("Channel #%d failed to update video async tasks: %s", channelId, err.Error()))
			}
		}()
	}
	return nil
}

func updateVideoTasks(ctx context.Context, platform constant.TaskPlatform, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, fmt.Sprintf("Channel #%d pending video tasks: %d", channelId, len(taskIds)))
	if len(taskIds) == 0 {
		return nil
	}
	cacheGetChannel, err := model.CacheGetChannel(channelId)
	if err != nil {
		// Collect DB primary key IDs for bulk update (taskIds are upstream IDs, not task_id column values)
		var failedIDs []int64
		for _, upstreamID := range taskIds {
			if t, ok := taskM[upstreamID]; ok {
				failedIDs = append(failedIDs, t.ID)
			}
		}
		errUpdate := model.TaskBulkUpdateByID(failedIDs, map[string]any{
			"fail_reason": fmt.Sprintf("Failed to get channel info, channel ID: %d", channelId),
			"status":      "FAILURE",
			"progress":    "100%",
		})
		if errUpdate != nil {
			common.SysLog(fmt.Sprintf("UpdateVideoTask error: %v", errUpdate))
		}
		return fmt.Errorf("CacheGetChannel failed: %w", err)
	}

	for _, taskId := range taskIds {
		taskId := taskId
		jobKey := fmt.Sprintf("video:%s:%d:%s", platform, channelId, taskId)
		startTaskPollingJob(jobKey, func() {
			adaptor := GetTaskAdaptorFunc(platform)
			if adaptor == nil {
				logger.LogError(ctx, fmt.Sprintf("Failed to update video task %s: video adaptor not found", taskId))
				return
			}
			info := &relaycommon.RelayInfo{}
			info.ChannelMeta = &relaycommon.ChannelMeta{ChannelBaseUrl: cacheGetChannel.GetBaseURL()}
			info.ApiKey = cacheGetChannel.Key
			adaptor.Init(info)
			if err := updateVideoSingleTask(ctx, adaptor, cacheGetChannel, taskId, taskM); err != nil {
				logger.LogError(ctx, fmt.Sprintf("Failed to update video task %s: %s", taskId, err.Error()))
			}
		})
	}
	return nil
}

func updateVideoSingleTask(ctx context.Context, adaptor TaskPollingAdaptor, ch *model.Channel, taskId string, taskM map[string]*model.Task) error {
	baseURL := constant.ChannelBaseURLs[ch.Type]
	if ch.GetBaseURL() != "" {
		baseURL = ch.GetBaseURL()
	}
	proxy := ch.GetSetting().Proxy

	task := taskM[taskId]
	if task == nil {
		logger.LogError(ctx, fmt.Sprintf("Task %s not found in taskM", taskId))
		return fmt.Errorf("task %s not found", taskId)
	}
	key := ch.Key

	privateData := task.PrivateData
	if privateData.Key != "" {
		key = privateData.Key
	}
	resp, err := adaptor.FetchTask(baseURL, key, map[string]any{
		"task_id": task.GetUpstreamTaskID(),
		"action":  task.Action,
	}, proxy)
	if err != nil {
		return fmt.Errorf("fetchTask failed for task %s: %w", taskId, err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("readAll failed for task %s: %w", taskId, err)
	}

	logger.LogDebug(ctx, "updateVideoSingleTask response: %s", responseBody)

	snap := task.Snapshot()

	taskResult := &relaycommon.TaskInfo{}
	// try parse as New API response format
	var responseItems dto.TaskResponse[model.Task]
	if err = common.Unmarshal(responseBody, &responseItems); err == nil && responseItems.IsSuccess() {
		logger.LogDebug(ctx, "updateVideoSingleTask parsed as new api response format: %+v", responseItems)
		t := responseItems.Data
		taskResult.TaskID = t.TaskID
		taskResult.Status = string(t.Status)
		taskResult.Url = t.GetResultURL()
		if taskResult.Url == "" {
			taskResult.Url = extractCompletedVideoURL(responseBody)
		}
		taskResult.Progress = t.Progress
		taskResult.Reason = t.FailReason
		task.Data = t.Data
	} else if taskResult, err = adaptor.ParseTaskResult(responseBody); err != nil {
		return fmt.Errorf("parseTaskResult failed for task %s: %w", taskId, err)
	}

	if task.Platform == constant.TaskPlatformImage {
		responseBody, taskResult.Url = normalizeImageResponseBodyForTask(task.TaskID, responseBody, taskResult.Url)
	}
	task.Data = redactVideoResponseBody(responseBody)

	logger.LogDebug(ctx, "updateVideoSingleTask taskResult: %+v", taskResult)

	now := time.Now().Unix()
	if taskResult.Status == "" {
		//taskResult = relaycommon.FailTaskInfo("upstream returned empty status")
		errorResult := &dto.GeneralErrorResponse{}
		if err = common.Unmarshal(responseBody, &errorResult); err == nil {
			openaiError := errorResult.TryToOpenAIError()
			if openaiError != nil {
				// 返回规范的 OpenAI 错误格式，提取错误信息，判断错误是否为任务失败
				if openaiError.Code == "429" {
					// 429 错误通常表示请求过多或速率限制，暂时不认为是任务失败，保持原状态等待下一轮轮询
					return nil
				}

				// 其他错误认为是任务失败，记录错误信息并更新任务状态
				taskResult = relaycommon.FailTaskInfo("upstream returned error")
			} else {
				// unknown error format, log original response
				logger.LogError(ctx, fmt.Sprintf("Task %s returned empty status with unrecognized error format, response: %s", taskId, string(responseBody)))
				taskResult = relaycommon.FailTaskInfo("upstream returned unrecognized message")
			}
		}
	}

	shouldRefund := false
	shouldSettle := false
	quota := task.Quota

	task.Status = model.TaskStatus(taskResult.Status)
	switch taskResult.Status {
	case model.TaskStatusSubmitted:
		task.Progress = taskcommon.ProgressSubmitted
	case model.TaskStatusQueued:
		task.Progress = taskcommon.ProgressQueued
	case model.TaskStatusInProgress:
		task.Progress = taskcommon.ProgressInProgress
		if task.StartTime == 0 {
			task.StartTime = now
		}
	case model.TaskStatusSuccess:
		if task.Platform != constant.TaskPlatformImage {
			if err := validateCompletedVideo(ctx, ch, task, taskResult); err != nil {
				if errors.Is(err, errVideoValidationInconclusive) {
					return fmt.Errorf("video validation inconclusive for task %s: %w", task.TaskID, err)
				}
				taskResult.Status = string(model.TaskStatusFailure)
				taskResult.Progress = taskcommon.ProgressComplete
				taskResult.Reason = "视频文件无效或时长为0秒"
				task.Status = model.TaskStatusFailure
				task.Progress = taskcommon.ProgressComplete
				task.FailReason = common.MaskSensitiveInfo(taskResult.Reason)
				task.PrivateData.ResultURL = ""
				if task.FinishTime == 0 {
					task.FinishTime = now
				}
				logger.LogWarn(ctx, fmt.Sprintf("Task %s returned invalid video, mark failed and refund: %v", task.TaskID, err))
				if quota != 0 {
					shouldRefund = true
				}
				break
			}
		}
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		// Always expose the stable New-API content endpoint. Keep any direct
		// provider URL private so it can be fetched by VideoProxy without leaking
		// the upstream host or returning an expiring CDN URL to clients.
		task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		if strings.HasPrefix(taskResult.Url, "data:") || taskResult.Url == "" {
			task.PrivateData.UpstreamResultURL = ""
		} else {
			task.PrivateData.UpstreamResultURL = taskResult.Url
		}
		shouldSettle = true
	case model.TaskStatusFailure:
		logger.LogJson(ctx, fmt.Sprintf("Task %s failed", taskId), task)
		task.Status = model.TaskStatusFailure
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		task.FailReason = taskResult.Reason
		logger.LogInfo(ctx, fmt.Sprintf("Task %s failed: %s", task.TaskID, task.FailReason))
		taskResult.Progress = taskcommon.ProgressComplete
		if quota != 0 {
			shouldRefund = true
		}
	default:
		return fmt.Errorf("unknown task status %s for task %s", taskResult.Status, task.TaskID)
	}
	if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}

	isDone := task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure
	if isDone && snap.Status != task.Status {
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("UpdateWithStatus failed for task %s: %s", task.TaskID, err.Error()))
			shouldRefund = false
			shouldSettle = false
		} else if !won {
			logger.LogWarn(ctx, fmt.Sprintf("Task %s already transitioned by another process, skip billing", task.TaskID))
			shouldRefund = false
			shouldSettle = false
		}
	} else if !snap.Equal(task.Snapshot()) {
		if _, err := task.UpdateWithStatus(snap.Status); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Failed to update task %s: %s", task.TaskID, err.Error()))
		}
	} else {
		// No changes, skip update
		logger.LogDebug(ctx, "No update needed for task %s", task.TaskID)
	}

	if shouldSettle {
		settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)
	}
	if shouldRefund {
		RefundTaskQuota(ctx, task, task.FailReason)
	}

	return nil
}

const maxVideoValidationBytes int64 = 64 << 20

var errVideoValidationInconclusive = errors.New("video validation inconclusive")

func extractCompletedVideoURL(body []byte) string {
	var payload any
	if err := common.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return findCompletedVideoURL(payload)
}

func findCompletedVideoURL(payload any) string {
	switch value := payload.(type) {
	case map[string]any:
		for _, key := range []string{"video_url", "videoUrl", "url", "download_url", "downloadUrl", "output_url", "outputUrl", "remote_url", "remoteUrl"} {
			if candidate, ok := value[key].(string); ok {
				candidate = strings.TrimSpace(candidate)
				if strings.HasPrefix(candidate, "https://") || strings.HasPrefix(candidate, "http://") || strings.HasPrefix(candidate, "/") || strings.HasPrefix(candidate, "data:video/") {
					return candidate
				}
			}
		}
		for _, child := range value {
			if candidate := findCompletedVideoURL(child); candidate != "" {
				return candidate
			}
		}
	case []any:
		for _, child := range value {
			if candidate := findCompletedVideoURL(child); candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func validateCompletedVideo(ctx context.Context, ch *model.Channel, task *model.Task, taskResult *relaycommon.TaskInfo) error {
	videoURL := strings.TrimSpace(taskResult.Url)
	if videoURL == "" {
		baseURL := strings.TrimRight(strings.TrimSpace(ch.GetBaseURL()), "/")
		upstreamTaskID := strings.TrimSpace(task.GetUpstreamTaskID())
		if baseURL == "" || upstreamTaskID == "" {
			return errors.New("video URL is empty")
		}
		name := upstreamTaskID
		if !strings.HasPrefix(name, "task_") {
			name = "task_" + name
		}
		videoURL = baseURL + "/outputs/" + url.PathEscape(name) + ".mp4"
	}

	if strings.HasPrefix(videoURL, "data:") {
		return nil
	}
	if strings.HasPrefix(videoURL, "/") {
		baseURL := strings.TrimRight(strings.TrimSpace(ch.GetBaseURL()), "/")
		if baseURL == "" {
			return errors.New("relative video URL without channel base URL")
		}
		videoURL = baseURL + videoURL
	}

	client, err := GetHttpClientWithProxy(ch.GetSetting().Proxy)
	if err != nil {
		return fmt.Errorf("%w: create client: %v", errVideoValidationInconclusive, err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, videoURL, nil)
	if err != nil {
		return fmt.Errorf("create video validation request: %w", err)
	}
	if ch.Type == constant.ChannelTypeOpenAI || ch.Type == constant.ChannelTypeSora {
		key := strings.TrimSpace(task.PrivateData.Key)
		if key == "" {
			key = strings.TrimSpace(ch.Key)
		}
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: download failed: %v", errVideoValidationInconclusive, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: HTTP %d", errVideoValidationInconclusive, resp.StatusCode)
	}
	if resp.ContentLength > maxVideoValidationBytes {
		return fmt.Errorf("video exceeds validation limit: %d bytes", resp.ContentLength)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxVideoValidationBytes+1))
	if err != nil {
		return fmt.Errorf("read video for validation: %w", err)
	}
	if int64(len(body)) > maxVideoValidationBytes {
		return fmt.Errorf("video exceeds validation limit")
	}
	if !hasTopLevelMP4Box(body, "moov") {
		return errors.New("MP4 metadata box moov is missing")
	}
	info, err := mp4.Probe(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("probe MP4: %w", err)
	}
	if info.Timescale == 0 || info.Duration == 0 {
		return fmt.Errorf("zero duration: duration=%d timescale=%d", info.Duration, info.Timescale)
	}
	if info.Duration <= uint64(info.Timescale)/10 {
		return fmt.Errorf("zero duration: duration=%d timescale=%d", info.Duration, info.Timescale)
	}
	return nil
}

func hasTopLevelMP4Box(body []byte, want string) bool {
	for offset := 0; offset+8 <= len(body); {
		size := uint64(binary.BigEndian.Uint32(body[offset : offset+4]))
		headerSize := uint64(8)
		if size == 1 {
			if offset+16 > len(body) {
				return false
			}
			size = binary.BigEndian.Uint64(body[offset+8 : offset+16])
			headerSize = 16
		} else if size == 0 {
			size = uint64(len(body) - offset)
		}
		if size < headerSize || size > uint64(len(body)-offset) {
			return false
		}
		if string(body[offset+4:offset+8]) == want {
			return true
		}
		offset += int(size)
	}
	return false
}

func redactVideoResponseBody(body []byte) []byte {
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return []byte(`{}`)
	}
	scrubVideoResponsePayload(payload)
	b, err := common.Marshal(payload)
	if err != nil {
		return []byte(`{}`)
	}
	return b
}

func scrubVideoResponsePayload(payload map[string]any) {
	for _, key := range []string{
		"parent_email", "account_email", "local_path", "upstream_video_id", "video_id", "remote_task_id",
		"upstream_task_id", "chat_id", "chatId", "log_id", "logId", "conversation_id",
		"url", "video_url", "public_url", "download_url", "no_watermark_url", "watermark_url",
		"remote_url", "output_url", "upstream_video_url", "poster", "thumb", "bytesBase64Encoded",
	} {
		delete(payload, key)
	}
	for key, value := range payload {
		switch typed := value.(type) {
		case string:
			payload[key] = common.MaskSensitiveInfo(typed)
		case map[string]any:
			scrubVideoResponsePayload(typed)
		case []any:
			for _, item := range typed {
				if child, ok := item.(map[string]any); ok {
					scrubVideoResponsePayload(child)
				}
			}
		}
	}
}

func truncateBase64(s string) string {
	const maxKeep = 256
	if len(s) <= maxKeep {
		return s
	}
	return s[:maxKeep] + "..."
}

// settleTaskBillingOnComplete 任务完成时的统一计费调整。
// 优先级：1. adaptor.AdjustBillingOnComplete 返回正数 → 使用 adaptor 计算的额度
//
//  2. taskResult.TotalTokens > 0 → 按 token 重算
//  3. 都不满足 → 保持预扣额度不变
func settleTaskBillingOnComplete(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, taskResult *relaycommon.TaskInfo) {
	// 0. 按次计费的任务不做差额结算
	if bc := task.PrivateData.BillingContext; bc != nil && bc.PerCallBilling {
		logger.LogInfo(ctx, fmt.Sprintf("任务 %s 按次计费，跳过差额结算", task.TaskID))
		return
	}
	// 1. 优先让 adaptor 决定最终额度
	if actualQuota := adaptor.AdjustBillingOnComplete(task, taskResult); actualQuota > 0 {
		RecalculateTaskQuota(ctx, task, actualQuota, "adaptor计费调整")
		return
	}
	// 2. 回退到 token 重算
	if taskResult.TotalTokens > 0 {
		RecalculateTaskQuotaByTokens(ctx, task, taskResult.TotalTokens)
		return
	}
	// 3. 无调整，保持预扣额度
}

func normalizeImageResponseBodyForTask(taskID string, body []byte, currentURL string) ([]byte, string) {
	var payload map[string]any
	if err := common.Unmarshal(body, &payload); err != nil {
		return body, currentURL
	}
	resultURL := currentURL
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
			if u := firstImageString(itemMap["url"], itemMap["output_url"]); u != "" {
				delete(itemMap, "b64_json")
				if resultURL == "" || strings.HasPrefix(resultURL, "data:") {
					resultURL = u
				}
				arr[i] = itemMap
				continue
			}
			if b64 := firstImageString(itemMap["b64_json"]); b64 != "" {
				if u, err := persistImagePollingOutput(taskID, i, b64); err == nil && u != "" {
					itemMap["url"] = u
					delete(itemMap, "b64_json")
					if resultURL == "" || strings.HasPrefix(resultURL, "data:") {
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
	if b, err := common.Marshal(payload); err == nil {
		body = b
	}
	return body, resultURL
}

func firstImageString(vals ...any) string {
	for _, v := range vals {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func persistImagePollingOutput(taskID string, index int, b64 string) (string, error) {
	b64 = strings.TrimSpace(b64)
	if strings.HasPrefix(b64, "data:") {
		if comma := strings.IndexByte(b64, ','); comma >= 0 {
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
	fileName := name + imagePollingOutputExt(data)
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

func imagePollingOutputExt(data []byte) string {
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
