package sora

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoRestartWindowFetchWaitsForListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	var requests atomic.Int32
	go func() {
		time.Sleep(1200 * time.Millisecond)
		listener, listenErr := net.Listen("tcp", addr)
		if listenErr != nil {
			return
		}
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests.Add(1)
			w.WriteHeader(http.StatusOK)
		})}
		_ = server.Serve(listener)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/v1/videos/task_test", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := doRestartWindowFetch(&http.Client{Timeout: time.Second}, req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || requests.Load() != 1 {
		t.Fatalf("status=%d requests=%d", resp.StatusCode, requests.Load())
	}
}
