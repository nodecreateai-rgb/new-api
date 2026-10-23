package model

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// ClickHouse lightweight UPDATEs land as patch parts. Every SELECT has to
// merge them on the fly until a regular merge applies them; on the tasks
// table (tens of updates per second from the task poller) they piled up and
// the unfinished-task scan went from ~50ms to 40s+, past the 30s query
// timeout, so polling stopped and tasks sat at 0%. This sweeper materializes
// patches in the background whenever that scan turns slow.
const (
	clickHousePatchSweepEvery = time.Minute
	// The unfinished-task scan normally takes ~50ms; above this the patches
	// are hurting reads and get materialized.
	clickHousePatchSlowScan = 3 * time.Second
)

func StartClickHousePatchSweeper() {
	if !common.UsingClickHouse || DB == nil {
		return
	}
	go func() {
		for {
			time.Sleep(clickHousePatchSweepEvery)
			sweepClickHousePatches()
		}
	}()
}

func sweepClickHousePatches() {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("clickhouse patch sweep panic: %v", r))
		}
	}()
	start := time.Now()
	var n int64
	err := DB.Raw("SELECT count() FROM tasks WHERE status IN ? SETTINGS max_execution_time = 60", []TaskStatus{
		TaskStatusNotStart, TaskStatusSubmitted, TaskStatusQueued, TaskStatusInProgress, TaskStatusUnknown,
	}).Scan(&n).Error
	took := time.Since(start)
	if err == nil && took < clickHousePatchSlowScan {
		return
	}
	common.SysLog(fmt.Sprintf("clickhouse tasks scan slow (%s, err=%v): applying patches", took.Round(time.Millisecond), err))
	start = time.Now()
	if err := DB.Exec("ALTER TABLE tasks APPLY PATCHES SETTINGS mutations_sync = 2, max_execution_time = 0").Error; err != nil {
		common.SysError("clickhouse apply patches tasks: " + err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("clickhouse apply patches tasks done in %s", time.Since(start).Round(time.Second)))
}
