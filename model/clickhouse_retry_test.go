package model

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func TestIsClickHouseTransientError(t *testing.T) {
	if !isClickHouseTransientError(errors.New("code: 101, message: Unexpected packet Query received from client")) {
		t.Fatal("expected protocol error to be transient")
	}
	if isClickHouseTransientError(gorm.ErrRecordNotFound) {
		t.Fatal("record not found must not be retried")
	}
	if isClickHouseTransientError(nil) {
		t.Fatal("nil must not be transient")
	}
}

func TestRetryClickHouseErrRetriesTransient(t *testing.T) {
	common.UsingClickHouse = true
	t.Cleanup(func() { common.UsingClickHouse = false })

	var calls atomic.Int32
	err := retryClickHouseErr(func() error {
		if calls.Add(1) < 3 {
			return errors.New("code: 101, message: Unexpected packet Query received from client")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls.Load())
	}
}

func TestRetryClickHouseErrDoesNotRetryPermanent(t *testing.T) {
	common.UsingClickHouse = true
	t.Cleanup(func() { common.UsingClickHouse = false })

	var calls atomic.Int32
	permanent := errors.New("record not found")
	err := retryClickHouseErr(func() error {
		calls.Add(1)
		return permanent
	})
	if !errors.Is(err, permanent) {
		t.Fatalf("expected permanent error, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 attempt, got %d", calls.Load())
	}
}

func TestRetryClickHouseErrSkippedWithoutClickHouse(t *testing.T) {
	common.UsingClickHouse = false
	var calls atomic.Int32
	err := retryClickHouseErr(func() error {
		calls.Add(1)
		return errors.New("code: 101, message: Unexpected packet Query received from client")
	})
	if err == nil {
		t.Fatal("expected error to pass through")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected 1 attempt without ClickHouse, got %d", calls.Load())
	}
}
