package service

import (
	"sync"
	"testing"
	"time"
)

func TestRetryableVideoPollKeepsASlowLookupInProgress(t *testing.T) {
	body := []byte(`{"error":{"message":"task not found","type":"invalid_request_error"}}`)
	if !retryableVideoPoll(404, body, 5*time.Minute) {
		t.Fatal("a fresh task-not-found lookup should be retried")
	}
	if retryableVideoPoll(404, body, 21*time.Minute) {
		t.Fatal("a long-missing task should be allowed to fail")
	}
	if !retryableVideoPoll(200, []byte(`{"error":{"message":"context deadline exceeded"}}`), time.Minute) {
		t.Fatal("a deadline with no generation status should be retried")
	}
	if !retryableVideoPoll(503, []byte(`gateway timeout`), time.Minute) {
		t.Fatal("http 503 should be retried")
	}
	if retryableVideoPoll(200, []byte(`{"status":"failed","error":{"message":"generation timed out"}}`), time.Minute) {
		t.Fatal("a failed generation must not be retried")
	}
	if retryableVideoPoll(200, []byte(`{"status":"processing","progress":40}`), time.Minute) {
		t.Fatal("an in-progress payload is a normal poll result")
	}
}

func TestTaskPollingIntervalDefaultsToTenSeconds(t *testing.T) {
	if got := taskPollingInterval(); got != 10*time.Second {
		t.Fatalf("task polling interval=%s", got)
	}
}

func TestTaskPollingJobsAreUnlimitedAndDeduplicatedByKey(t *testing.T) {
	taskPollingJobs.Range(func(key, _ any) bool {
		taskPollingJobs.Delete(key)
		return true
	})

	entered := make(chan string, 2)
	release := make(chan struct{})
	var done sync.WaitGroup
	job := func(key string) func() {
		done.Add(1)
		return func() {
			defer done.Done()
			entered <- key
			<-release
		}
	}

	if !startTaskPollingJob("task-a", job("task-a")) {
		t.Fatal("first task-a job was rejected")
	}
	if startTaskPollingJob("task-a", func() {}) {
		t.Fatal("duplicate task-a job was admitted")
	}
	if !startTaskPollingJob("task-b", job("task-b")) {
		t.Fatal("independent task-b job was rejected")
	}

	seen := map[string]bool{}
	for range 2 {
		select {
		case key := <-entered:
			seen[key] = true
		case <-time.After(time.Second):
			t.Fatal("independent polling jobs did not run concurrently")
		}
	}
	if !seen["task-a"] || !seen["task-b"] {
		t.Fatalf("started jobs=%v", seen)
	}

	close(release)
	done.Wait()
	deadline := time.Now().Add(time.Second)
	for !startTaskPollingJob("task-a", func() {}) {
		if time.Now().After(deadline) {
			t.Fatal("task-a key was not released after completion")
		}
		time.Sleep(time.Millisecond)
	}
}
