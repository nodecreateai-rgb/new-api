package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

const (
	taskCacheKeyPrefix  = "task:v2:"
	taskCacheMissPrefix = "task:v2:miss:"
	taskCacheMissTTL    = 30 * time.Second
)

type taskCacheRecord struct {
	Task
	PrivateData TaskPrivateData `json:"private_data"`
}

var taskLookupFlight singleflight.Group

func taskCacheKeyUser(userId int, taskId string) string {
	return fmt.Sprintf("%s%d:%s", taskCacheKeyPrefix, userId, taskId)
}

func taskCacheKeyOnly(taskId string) string {
	return fmt.Sprintf("%stid:%s", taskCacheKeyPrefix, taskId)
}

func taskCacheTTL(status TaskStatus) time.Duration {
	switch status {
	case TaskStatusSuccess, TaskStatusFailure:
		sec := constant.TaskCacheTTLDoneSec
		if sec <= 0 {
			sec = 1800
		}
		return time.Duration(sec) * time.Second
	default:
		sec := constant.TaskCacheTTLActiveSec
		if sec <= 0 {
			sec = 60
		}
		return time.Duration(sec) * time.Second
	}
}

func cacheGetTask(key string) (*Task, bool) {
	if !common.RedisEnabled {
		return nil, false
	}
	raw, err := common.RedisGet(key)
	if err != nil || raw == "" {
		return nil, false
	}
	var rec taskCacheRecord
	if err := common.Unmarshal([]byte(raw), &rec); err != nil {
		return nil, false
	}
	task := rec.Task
	task.PrivateData = rec.PrivateData
	return &task, true
}

func cacheSetTask(key string, task *Task) {
	if !common.RedisEnabled || task == nil {
		return
	}
	payload, err := common.Marshal(taskCacheRecord{Task: *task, PrivateData: task.PrivateData})
	if err != nil {
		return
	}
	_ = common.RedisSet(key, string(payload), taskCacheTTL(task.Status))
}

func cacheSetTaskLookup(task *Task) {
	if task == nil || task.TaskID == "" {
		return
	}
	cacheSetTask(taskCacheKeyUser(task.UserId, task.TaskID), task)
	cacheSetTask(taskCacheKeyOnly(task.TaskID), task)
}

// RefreshTaskCache writes the latest task state to Redis so polling clients
// can read progress without hitting ClickHouse after a background update.
func RefreshTaskCache(task *Task) {
	cacheSetTaskLookup(task)
}

func taskCacheMissKey(userId int, taskId string) string {
	if userId <= 0 {
		return taskCacheMissPrefix + "tid:" + taskId
	}
	return fmt.Sprintf("%s%d:%s", taskCacheMissPrefix, userId, taskId)
}

func cacheGetTaskMiss(userId int, taskId string) bool {
	if !common.RedisEnabled || taskId == "" {
		return false
	}
	val, err := common.RedisGet(taskCacheMissKey(userId, taskId))
	return err == nil && val == "1"
}

func cacheSetTaskMiss(userId int, taskId string) {
	if !common.RedisEnabled || taskId == "" {
		return
	}
	_ = common.RedisSet(taskCacheMissKey(userId, taskId), "1", taskCacheMissTTL)
}

func invalidateTaskCache(userId int, taskId string) {
	if !common.RedisEnabled || taskId == "" {
		return
	}
	if userId > 0 {
		_ = common.RedisDel(taskCacheKeyUser(userId, taskId))
	}
	_ = common.RedisDel(taskCacheKeyOnly(taskId))
	_ = common.RedisDel(taskCacheMissKey(userId, taskId))
	_ = common.RedisDel(taskCacheMissKey(0, taskId))
}

func invalidateTaskCaches(taskIds ...string) {
	for _, taskId := range taskIds {
		if taskId == "" {
			continue
		}
		invalidateTaskCache(0, taskId)
	}
}

func taskFilterKey(filters map[string]any) string {
	cols := make([]string, 0, len(filters))
	for col := range filters {
		cols = append(cols, col)
	}
	sort.Strings(cols)
	parts := make([]string, 0, len(cols))
	for _, col := range cols {
		parts = append(parts, fmt.Sprintf("%s=%v", col, filters[col]))
	}
	return strings.Join(parts, "&")
}

// findTaskByFilter loads a single task without GORM First()'s ORDER BY primary key,
// which forces a full sort on ClickHouse even when task_id is unique.
func findTaskByFilter(filters map[string]any, task **Task) error {
	if len(filters) == 0 {
		return fmt.Errorf("empty task filter")
	}
	result, err, _ := taskLookupFlight.Do(taskFilterKey(filters), func() (any, error) {
		// Table() avoids GORM injecting zero-value struct fields (e.g. user_id=0).
		q := DB.Table("tasks")
		for col, val := range filters {
			q = q.Where(col+" = ?", val)
		}
		var rows []Task
		if err := q.Limit(1).Find(&rows).Error; err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, gorm.ErrRecordNotFound
		}
		return &rows[0], nil
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			*task = nil
		}
		return err
	}
	*task = result.(*Task)
	return nil
}
