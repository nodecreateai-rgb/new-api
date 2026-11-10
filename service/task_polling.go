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
	"strconv"
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
// fastTaskTimeoutMinutes caps models whose upstream normally finishes in
// minutes: a task still unfinished past this is stuck upstream, so it is
// failed and refunded instead of hanging for the global 600-minute timeout.
var fastTaskTimeoutMinutes = map[string]int{
	"seedance-2.0-fast-720p-c2": 45,
	"seedance-2.0-fast-720p-c3": 45,
}

// pendingUpstreamSubmitTimeoutMinutes fails local-only video rows whose
// upstream create call never returned an id.
const pendingUpstreamSubmitTimeoutMinutes = 20

func sweepTimedOutTasks(ctx context.Context) {
	sweepFastModelTimeouts(ctx)
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

func isPendingUpstreamSubmit(task *model.Task) bool {
	return task != nil && task.PrivateData.PendingUpstream
}

func pendingUpstreamSubmitExpired(task *model.Task, now int64) bool {
	return isPendingUpstreamSubmit(task) && task.SubmitTime > 0 && now-task.SubmitTime > int64(pendingUpstreamSubmitTimeoutMinutes)*60
}

func failPendingUpstreamTask(ctx context.Context, task *model.Task) {
	if task == nil {
		return
	}
	oldStatus := task.Status
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.PrivateData.PendingUpstream = false
	task.FailReason = fmt.Sprintf("上游提交超时（%d分钟未返回任务）", pendingUpstreamSubmitTimeoutMinutes)
	won, err := task.UpdateWithStatus(oldStatus)
	if err != nil || !won {
		return
	}
	logger.LogInfo(ctx, fmt.Sprintf("failPendingUpstreamTask: %s failed after %d min", task.TaskID, pendingUpstreamSubmitTimeoutMinutes))
	if task.Quota != 0 {
		RefundTaskQuota(ctx, task, task.FailReason)
	}
}

const falseFailedDownloadReason = "视频文件暂时无法下载"

func recoverFalseFailedDownloads(ctx context.Context) {
	since := time.Now().Unix() - 48*3600
	tasks := model.GetFalseFailedDownloadTasks(since, 50)
	if len(tasks) == 0 {
		return
	}
	recovered := 0
	for _, task := range tasks {
		if task == nil || strings.TrimSpace(task.GetUpstreamTaskID()) == "" {
			continue
		}
		ch, err := model.CacheGetChannel(task.ChannelId)
		if err != nil {
			ch, err = model.GetChannelById(task.ChannelId, true)
			if err != nil {
				continue
			}
		}
		adaptor := GetTaskAdaptorFunc(constant.TaskPlatform(strconv.Itoa(ch.Type)))
		if adaptor == nil {
			adaptor = GetTaskAdaptorFunc(task.Platform)
		}
		if adaptor == nil {
			continue
		}
		info := &relaycommon.RelayInfo{}
		info.ChannelMeta = &relaycommon.ChannelMeta{ChannelBaseUrl: ch.GetBaseURL()}
		info.ApiKey = ch.Key
		adaptor.Init(info)
		baseURL := ch.GetBaseURL()
		if baseURL == "" {
			baseURL = constant.ChannelBaseURLs[ch.Type]
		}
		key := strings.TrimSpace(task.PrivateData.Key)
		if key == "" {
			key = strings.TrimSpace(ch.Key)
		}
		resp, err := adaptor.FetchTask(baseURL, key, map[string]any{
			"task_id": task.GetUpstreamTaskID(),
			"action":  task.Action,
		}, ch.GetSetting().Proxy)
		if err != nil || resp == nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			continue
		}
		taskResult, err := adaptor.ParseTaskResult(body)
		if err != nil || taskResult == nil {
			continue
		}
		if taskResult.Url == "" {
			taskResult.Url = extractCompletedVideoURL(body)
		}
		if model.TaskStatus(taskResult.Status) != model.TaskStatusSuccess {
			continue
		}
		readyURL := absoluteUpstreamVideoURL(ch, taskResult.Url)
		if readyURL == "" {
			continue
		}
		oldStatus := task.Status
		now := time.Now().Unix()
		task.Status = model.TaskStatusSuccess
		task.Progress = taskcommon.ProgressComplete
		task.FailReason = ""
		task.FinishTime = now
		task.Data = model.JSONRaw(redactVideoResponseBody(body))
		task.PrivateData.DirectUpstreamVideo = false
		task.PrivateData.UpstreamResultURL = readyURL
		task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		won, err := task.UpdateWithStatus(oldStatus)
		if err != nil || !won {
			continue
		}
		ReclaimTaskQuota(ctx, task, "上游已完成，恢复成功并补扣")
		recovered++
		logger.LogInfo(ctx, fmt.Sprintf("recoverFalseFailedDownloads: restored %s", task.TaskID))
	}
	if recovered > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("recoverFalseFailedDownloads: restored %d tasks", recovered))
	}
}

func sweepFastModelTimeouts(ctx context.Context) {
	now := time.Now().Unix()
	for modelName, minutes := range fastTaskTimeoutMinutes {
		cutoff := now - int64(minutes)*60
		reason := fmt.Sprintf("上游生成超时（%d分钟未完成），已自动退款，请重新提交", minutes)
		for _, task := range model.GetTimedOutUnfinishedTasksByModel(modelName, cutoff, 200) {
			if task.Properties.OriginModelName != modelName {
				continue
			}
			oldStatus := task.Status
			task.Status = model.TaskStatusFailure
			task.Progress = "100%"
			task.FinishTime = now
			task.FailReason = reason
			won, err := task.UpdateWithStatus(oldStatus)
			if err != nil || !won {
				continue
			}
			logger.LogInfo(ctx, fmt.Sprintf("sweepFastModelTimeouts: %s (%s) failed after %d min", task.TaskID, modelName, minutes))
			if task.Quota != 0 {
				RefundTaskQuota(ctx, task, reason)
			}
		}
	}
}

var taskPollingJobs sync.Map
var taskProgressLastWrite sync.Map // task public id -> last ClickHouse write time

func taskPollingInterval() time.Duration {
	sec := constant.TaskPollingIntervalSec
	if sec <= 0 {
		sec = 10
	}
	return time.Duration(sec) * time.Second
}

func taskProgressWriteInterval() time.Duration {
	sec := constant.TaskProgressWriteIntervalSec
	if sec <= 0 {
		sec = 30
	}
	return time.Duration(sec) * time.Second
}

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

// TaskPollingLoop 主轮询循环，按 TASK_POLLING_INTERVAL_SEC 检查未完成任务。
func TaskPollingLoop() {
	for {
		time.Sleep(taskPollingInterval())
		common.SysLog("任务进度轮询开始")
		ctx := context.TODO()
		sweepTimedOutTasks(ctx)
		recoverFalseFailedDownloads(ctx)
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
				now := time.Now().Unix()
				for _, task := range tasks {
					if isPendingUpstreamSubmit(task) {
						if pendingUpstreamSubmitExpired(task, now) {
							failPendingUpstreamTask(ctx, task)
						}
						continue
					}
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
	case constant.TaskPlatformPay:
		if err := UpdatePayTasks(context.Background(), taskChannelM, taskM); err != nil {
			common.SysLog(fmt.Sprintf("UpdatePayTasks fail: %s", err))
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
		task.Data = model.JSONRaw(responseItem.Data)

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

	oldData, _ := common.Marshal([]byte(oldTask.Data))
	newData, _ := common.Marshal([]byte(newTask.Data))

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

// UpdatePayTasks 按渠道更新所有协议支付异步任务。
func UpdatePayTasks(ctx context.Context, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	return UpdateVideoTasks(ctx, constant.TaskPlatformPay, taskChannelM, taskM)
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

// notFoundPollGrace is how long a "task not found" status lookup is treated
// as a glitch. After that the task is failed, so a genuinely missing id
// does not poll until the global task timeout.
const notFoundPollGrace = 20 * time.Minute

func taskPollAge(task *model.Task) time.Duration {
	if task == nil {
		return 0
	}
	stamp := task.SubmitTime
	if stamp <= 0 {
		stamp = task.CreatedAt
	}
	if stamp <= 0 {
		return 0
	}
	return time.Since(time.Unix(stamp, 0))
}

// retryableVideoPoll reports a status response that should be retried.
// A payload that already carries a generation status is never retryable:
// "failed" with a timeout in the message is a real result.
func retryableVideoPoll(statusCode int, body []byte, age time.Duration) bool {
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusRequestTimeout || statusCode >= 500 {
		return true
	}
	var probe struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := common.Unmarshal(body, &probe); err != nil {
		return false
	}
	if strings.TrimSpace(probe.Status) != "" {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(probe.Error.Message))
	if msg == "" {
		msg = strings.ToLower(strings.TrimSpace(probe.Message))
	}
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "task not found") {
		return age < notFoundPollGrace
	}
	for _, marker := range []string{
		"status temporarily unavailable",
		"context deadline exceeded",
		"gateway timeout",
		"bad gateway",
		"timeout",
		"timed out",
		"unexpected eof",
		"bad record mac",
		"connection reset",
		"connection refused",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func updateVideoTasks(ctx context.Context, platform constant.TaskPlatform, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, fmt.Sprintf("Channel #%d pending video tasks: %d", channelId, len(taskIds)))
	if len(taskIds) == 0 {
		return nil
	}
	cacheGetChannel, err := model.CacheGetChannel(channelId)
	if err != nil {
		// A channel-cache miss is infrastructure/cache state, not an upstream
		// terminal result. Fall back to the authoritative database so a transient
		// empty/stale cache cannot permanently fail and refund every in-flight task
		// for the channel.
		cacheErr := err
		cacheGetChannel, err = model.GetChannelById(channelId, true)
		if err != nil {
			return fmt.Errorf("get channel %d failed (cache: %v, db: %w)", channelId, cacheErr, err)
		}
		model.CacheUpdateChannel(cacheGetChannel)
		logger.LogInfo(ctx, fmt.Sprintf("Channel #%d missing from cache; recovered from database for %d pending video tasks", channelId, len(taskIds)))
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
	// A slow or missing status lookup is not a finished generation. Failing
	// here refunds a video that is still rendering. Keep polling instead.
	if retryableVideoPoll(resp.StatusCode, responseBody, taskPollAge(task)) {
		logger.LogInfo(ctx, fmt.Sprintf("Task %s status poll is retryable (http %d); keeping it in progress", taskId, resp.StatusCode))
		return nil
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
	// OpenAI/Sora adaptors leave Url empty on completed; qinnaonao and similar
	// gateways return a CDN url in the JSON instead of serving /outputs/{id}.mp4.
	if taskResult.Url == "" {
		taskResult.Url = extractCompletedVideoURL(responseBody)
	}

	if task.Platform == constant.TaskPlatformImage {
		responseBody, taskResult.Url = normalizeImageResponseBodyForTask(task.TaskID, responseBody, taskResult.Url)
	}
	task.Data = model.JSONRaw(redactVideoResponseBody(responseBody))

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
		var videoBytes []byte
		passthrough := ksyunSeedanceReturnsUpstreamVideo(ch, task)
		if !passthrough && task.Platform != constant.TaskPlatformImage && task.Platform != constant.TaskPlatformPay {
			readyURL := absoluteUpstreamVideoURL(ch, taskResult.Url)
			if readyURL != "" && !clientMayKeepUpstreamVideoURL(ch, readyURL) {
				// Provider content must stay behind our proxy. Do not block the
				// poller on a full local download: queueing here was marking
				// finished upstream jobs as failures.
				clearVideoValidationMiss(task.TaskID)
				videoBytes = nil
				passthrough = false
				taskResult.Url = readyURL
				logger.LogInfo(ctx, fmt.Sprintf("Task %s keeps provider video behind gateway content URL", task.TaskID))
			} else {
				var err error
				videoBytes, err = validateCompletedVideo(ctx, ch, task, taskResult)
				if err != nil {
					if readyURL != "" && errors.Is(err, errVideoValidationInconclusive) {
						logger.LogWarn(ctx, fmt.Sprintf("Task %s upstream video is ready; skip local validation: %v", task.TaskID, err))
						clearVideoValidationMiss(task.TaskID)
						videoBytes = nil
						passthrough = true
						taskResult.Url = readyURL
					} else if errors.Is(err, errVideoValidationInconclusive) {
						misses := noteVideoValidationMiss(task.TaskID)
						if misses < maxVideoValidationMisses {
							return fmt.Errorf("video validation inconclusive for task %s: %w", task.TaskID, err)
						}
						clearVideoValidationMiss(task.TaskID)
						taskResult.Status = string(model.TaskStatusFailure)
						taskResult.Progress = taskcommon.ProgressComplete
						taskResult.Reason = "视频文件暂时无法下载，已自动退款，请重新提交"
						task.Status = model.TaskStatusFailure
						task.Progress = taskcommon.ProgressComplete
						task.FailReason = common.PublicErrorMessage(taskResult.Reason)
						task.PrivateData.ResultURL = ""
						if task.FinishTime == 0 {
							task.FinishTime = now
						}
						logger.LogWarn(ctx, fmt.Sprintf("Task %s gave up fetching the finished video after %d attempts: %v", task.TaskID, misses, err))
						if quota != 0 {
							shouldRefund = true
						}
						break
					} else {
						clearVideoValidationMiss(task.TaskID)
						taskResult.Status = string(model.TaskStatusFailure)
						taskResult.Progress = taskcommon.ProgressComplete
						taskResult.Reason = "视频文件无效或时长为0秒"
						task.Status = model.TaskStatusFailure
						task.Progress = taskcommon.ProgressComplete
						task.FailReason = common.PublicErrorMessage(taskResult.Reason)
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
				} else {
					clearVideoValidationMiss(task.TaskID)
				}
			}
		}
		if up := task.PrivateData.Upscaler; up != nil && !up.Done && task.Status == model.TaskStatusSuccess {
			src := videoBytes
			if len(src) == 0 && up.TaskID == "" {
				if d, derr := decodeVideoDataURL(strings.TrimSpace(taskResult.Url)); derr == nil {
					src = d
				}
			}
			upVideo, wait := handleVideoUpscale(ctx, task, src)
			if wait {
				// Generation finished, upscale running: stay in progress.
				task.Status = model.TaskStatusInProgress
				task.Progress = "90%"
				if task.StartTime == 0 {
					task.StartTime = now
				}
				if err := task.UpdateProgress(snap.Status); err != nil {
					logger.LogError(ctx, fmt.Sprintf("Failed to update upscaling task %s: %s", task.TaskID, err.Error()))
				}
				return nil
			}
			if len(upVideo) > 0 {
				videoBytes = upVideo
				passthrough = false
				taskResult.Url = ""
			}
		}
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		if passthrough {
			// This channel/model is fixed in code: return the provider file URL
			// and never replace it with oss.domie.studio after a restart.
			upstream := absoluteUpstreamVideoURL(ch, taskResult.Url)
			if upstream == "" {
				taskResult.Status = string(model.TaskStatusFailure)
				taskResult.Progress = taskcommon.ProgressComplete
				taskResult.Reason = "视频文件无效或时长为0秒"
				task.Status = model.TaskStatusFailure
				task.Progress = taskcommon.ProgressComplete
				task.FailReason = common.PublicErrorMessage(taskResult.Reason)
				task.PrivateData.ResultURL = ""
				task.PrivateData.DirectUpstreamVideo = false
				if task.FinishTime == 0 {
					task.FinishTime = now
				}
				if quota != 0 {
					shouldRefund = true
				}
				break
			}
			task.PrivateData.DirectUpstreamVideo = true
			task.PrivateData.UpstreamResultURL = upstream
			task.PrivateData.ResultURL = upstream
		} else {
			// Keep any direct provider URL private. Prefer a public R2 object URL when
			// upload succeeds; otherwise expose the stable New-API content proxy.
			task.PrivateData.DirectUpstreamVideo = false
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
			if strings.HasPrefix(taskResult.Url, "data:") || taskResult.Url == "" {
				task.PrivateData.UpstreamResultURL = ""
			} else {
				task.PrivateData.UpstreamResultURL = taskResult.Url
			}
			if task.Platform != constant.TaskPlatformImage && task.Platform != constant.TaskPlatformPay {
				if publicURL := persistGeneratedVideoToR2(ctx, task, taskResult, videoBytes); publicURL != "" {
					task.PrivateData.ResultURL = publicURL
				}
			}
		}
		shouldSettle = true
	case model.TaskStatusFailure:
		logger.LogJson(ctx, fmt.Sprintf("Task %s failed", taskId), task)
		task.Status = model.TaskStatusFailure
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		task.FailReason = common.PublicErrorMessage(taskResult.Reason)
		logger.LogInfo(ctx, fmt.Sprintf("Task %s failed: %s", task.TaskID, taskResult.Reason))
		taskResult.Progress = taskcommon.ProgressComplete
		if quota != 0 {
			shouldRefund = true
		}
	default:
		return fmt.Errorf("unknown task status %s for task %s", taskResult.Status, task.TaskID)
	}
	if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
		task.Progress = taskcommon.ProgressComplete
	} else if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}

	isDone := task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure
	after := task.Snapshot()
	shouldPersist := shouldPersistTaskUpdate(task.TaskID, snap, after, isDone)
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
		} else {
			clearTaskProgressWrite(task.TaskID)
		}
	} else if shouldPersist {
		if err := task.UpdateProgress(snap.Status); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Failed to update task %s: %s", task.TaskID, err.Error()))
		} else {
			markTaskProgressWritten(task.TaskID)
		}
	} else {
		model.RefreshTaskCache(task)
		logger.LogDebug(ctx, "Skip ClickHouse write for task %s; refreshed cache only", task.TaskID)
	}

	if shouldSettle {
		settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)
	}
	if shouldRefund {
		RefundTaskQuota(ctx, task, task.FailReason)
	}

	return nil
}

// 30s 720p/1080p outputs routinely exceed 64MB; keep a generous ceiling.
const maxVideoValidationBytes int64 = 1 << 30

// A public CDN only gets a short local fetch. Waiting minutes for a slow CDN
// is what left completed videos at 30%.
var videoValidationTimeout = 20 * time.Second

// Provider content links expire and reject anonymous downloads. The gateway
// has to store the file itself, so this fetch is allowed to take longer.
var providerVideoDownloadTimeout = 3 * time.Minute

// maxVideoValidationMisses bounds retries when upstream says the video is done
// but no client-usable URL exists. After this, the task fails and is refunded.
const maxVideoValidationMisses = 6

// Keep only a few full-file fetches in flight. Extra completed tasks fall
// through to their upstream URL instead of queueing behind a slow download.
var videoValidationSlots = make(chan struct{}, 4)

var videoValidationMisses sync.Map

var errVideoValidationInconclusive = errors.New("video validation inconclusive")

func noteVideoValidationMiss(taskID string) int {
	n := 1
	if current, ok := videoValidationMisses.Load(taskID); ok {
		if misses, ok := current.(int); ok {
			n = misses + 1
		}
	}
	videoValidationMisses.Store(taskID, n)
	return n
}

func clearVideoValidationMiss(taskID string) {
	videoValidationMisses.Delete(taskID)
}

func acquireVideoValidationSlot() bool {
	select {
	case videoValidationSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func releaseVideoValidationSlot() {
	select {
	case <-videoValidationSlots:
	default:
	}
}

// completedVideoCanFinishWithoutBytes is true when upstream already returned a
// playable URL and the only problem is that this process could not download
// the file. A missing URL still retries; a parsed invalid MP4 does not.
func completedVideoCanFinishWithoutBytes(err error, videoURL string) bool {
	if err == nil || !errors.Is(err, errVideoValidationInconclusive) {
		return false
	}
	videoURL = strings.TrimSpace(videoURL)
	return strings.HasPrefix(videoURL, "https://") || strings.HasPrefix(videoURL, "http://")
}

// videoURLNeedsChannelAuth reports provider API paths that reject anonymous
// downloads. Public CDN links must be fetched without the channel key.
func videoURLNeedsChannelAuth(videoURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(videoURL))
	if err != nil || parsed.Host == "" {
		return false
	}
	path := strings.ToLower(parsed.Path)
	return strings.Contains(path, "/v1/task/") ||
		strings.Contains(path, "/v1/videos/") ||
		strings.Contains(path, "/outputs/") ||
		strings.Contains(path, "/api/v1/") ||
		strings.Contains(path, "/media-content/")
}

// clientMayKeepUpstreamVideoURL is true when the client can fetch the file
// without our channel key. Provider hosts and content endpoints must be
// downloaded and replaced with our own URL.
func clientMayKeepUpstreamVideoURL(ch *model.Channel, raw string) bool {
	if !model.ClientMayReceiveDirectVideoURL(raw) {
		return false
	}
	return !videoURLOnChannelHost(ch, raw)
}

func videoURLOnChannelHost(ch *model.Channel, raw string) bool {
	if ch == nil {
		return false
	}
	base, err := url.Parse(strings.TrimSpace(ch.GetBaseURL()))
	file, err2 := url.Parse(strings.TrimSpace(raw))
	if err != nil || err2 != nil || base.Hostname() == "" || file.Hostname() == "" {
		return false
	}
	return strings.EqualFold(base.Hostname(), file.Hostname())
}

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

// ksyunSeedanceReturnsUpstreamVideo is fixed for the Ksyun channel's seedance-2.5
// model. Startup must not turn this back into an object-storage URL.
func ksyunSeedanceReturnsUpstreamVideo(ch *model.Channel, task *model.Task) bool {
	if ch == nil || task == nil || taskModelName(task) != "seedance-2.5" {
		return false
	}
	base := strings.ToLower(strings.TrimSpace(ch.GetBaseURL()))
	name := strings.ToLower(strings.TrimSpace(ch.Name))
	return strings.Contains(base, "ksyun2api") || strings.Contains(name, "ksyun")
}

func absoluteUpstreamVideoURL(ch *model.Channel, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "data:") || r2PublicURL(raw) {
		return ""
	}
	if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		return raw
	}
	if strings.HasPrefix(raw, "/") && ch != nil {
		base := strings.TrimRight(strings.TrimSpace(ch.GetBaseURL()), "/")
		if base != "" {
			return base + raw
		}
	}
	return ""
}

func r2PublicURL(raw string) bool {
	return strings.Contains(strings.ToLower(raw), "oss.domie.studio")
}

func validateCompletedVideo(ctx context.Context, ch *model.Channel, task *model.Task, taskResult *relaycommon.TaskInfo) ([]byte, error) {
	videoURL := strings.TrimSpace(taskResult.Url)
	if videoURL == "" {
		baseURL := strings.TrimRight(strings.TrimSpace(ch.GetBaseURL()), "/")
		upstreamTaskID := strings.TrimSpace(task.GetUpstreamTaskID())
		if baseURL == "" || upstreamTaskID == "" {
			return nil, errors.New("video URL is empty")
		}
		name := upstreamTaskID
		if !strings.HasPrefix(name, "task_") {
			name = "task_" + name
		}
		videoURL = baseURL + "/outputs/" + url.PathEscape(name) + ".mp4"
	}

	if strings.HasPrefix(videoURL, "data:") {
		return nil, nil
	}
	if strings.HasPrefix(videoURL, "/") {
		baseURL := strings.TrimRight(strings.TrimSpace(ch.GetBaseURL()), "/")
		if baseURL == "" {
			return nil, errors.New("relative video URL without channel base URL")
		}
		videoURL = baseURL + videoURL
	}

	client, err := GetHttpClientWithProxy(ch.GetSetting().Proxy)
	if err != nil {
		return nil, fmt.Errorf("%w: create client: %v", errVideoValidationInconclusive, err)
	}
	if !acquireVideoValidationSlot() {
		return nil, fmt.Errorf("%w: validation busy", errVideoValidationInconclusive)
	}
	defer releaseVideoValidationSlot()
	downloadTimeout := videoValidationTimeout
	storeLocally := !clientMayKeepUpstreamVideoURL(ch, videoURL)
	if storeLocally {
		downloadTimeout = providerVideoDownloadTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, videoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create video validation request: %w", err)
	}
	if storeLocally && (ch.Type == constant.ChannelTypeOpenAI || ch.Type == constant.ChannelTypeSora) {
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
		return nil, fmt.Errorf("%w: download failed: %v", errVideoValidationInconclusive, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", errVideoValidationInconclusive, resp.StatusCode)
	}
	if resp.ContentLength > maxVideoValidationBytes {
		return nil, fmt.Errorf("%w: video exceeds validation limit: %d bytes", errVideoValidationInconclusive, resp.ContentLength)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxVideoValidationBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read video for validation: %v", errVideoValidationInconclusive, err)
	}
	if int64(len(body)) > maxVideoValidationBytes {
		return nil, fmt.Errorf("%w: video exceeds validation limit", errVideoValidationInconclusive)
	}
	if !hasTopLevelMP4Box(body, "moov") {
		return nil, errors.New("MP4 metadata box moov is missing")
	}
	info, err := mp4.Probe(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("probe MP4: %w", err)
	}
	if info.Timescale == 0 || info.Duration == 0 {
		return nil, fmt.Errorf("zero duration: duration=%d timescale=%d", info.Duration, info.Timescale)
	}
	if info.Duration <= uint64(info.Timescale)/10 {
		return nil, fmt.Errorf("zero duration: duration=%d timescale=%d", info.Duration, info.Timescale)
	}
	return body, nil
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
		"remote_url", "output_url", "upstream_video_url", "content_url", "result_url", "poster", "thumb", "bytesBase64Encoded",
		"thumbnail_url", "thumb_url", "file_url", "fileUrl", "file_thumb_url",
	} {
		delete(payload, key)
	}
	for key, value := range payload {
		switch typed := value.(type) {
		case string:
			payload[key] = common.MaskPublicTaskText(key, typed)
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

func shouldPersistTaskUpdate(taskID string, before, after model.TaskSnapshot, terminal bool) bool {
	if before.NeedsPersist(after, terminal) {
		return true
	}
	if terminal || bytes.Equal(before.Data, after.Data) {
		return false
	}
	return allowThrottledProgressWrite(taskID)
}

func allowThrottledProgressWrite(taskID string) bool {
	now := time.Now()
	if last, ok := taskProgressLastWrite.Load(taskID); ok {
		if now.Sub(last.(time.Time)) < taskProgressWriteInterval() {
			return false
		}
	}
	return true
}

func markTaskProgressWritten(taskID string) {
	taskProgressLastWrite.Store(taskID, time.Now())
}

func clearTaskProgressWrite(taskID string) {
	taskProgressLastWrite.Delete(taskID)
}
