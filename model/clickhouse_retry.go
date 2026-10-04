package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const clickHouseReadRetry = 3

func isClickHouseTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unexpected packet") ||
		strings.Contains(msg, "driver: bad connection") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset")
}

// retryClickHouseErr retries a ClickHouse-backed read when the native
// protocol connection is left in a bad state by concurrent inserts/updates.
func retryClickHouseErr(op func() error) error {
	if !common.UsingClickHouse {
		return op()
	}
	var err error
	for attempt := 1; attempt <= clickHouseReadRetry; attempt++ {
		err = op()
		if err == nil || !isClickHouseTransientError(err) {
			return err
		}
		if attempt < clickHouseReadRetry {
			common.SysLog(fmt.Sprintf("clickhouse transient error, retry %d/%d: %v", attempt, clickHouseReadRetry-1, err))
			time.Sleep(time.Duration(attempt*20) * time.Millisecond)
		}
	}
	return err
}
