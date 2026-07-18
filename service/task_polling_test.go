package service

import (
	"sync"
	"testing"
	"time"
)

func TestTaskPollingIntervalIsThreeSeconds(t *testing.T) {
	if taskPollingInterval != 3*time.Second {
		t.Fatalf("task polling interval=%s", taskPollingInterval)
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
