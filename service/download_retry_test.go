package service

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDoDownloadWithRetryRecoversFromTransientEOF(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt < 3 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("response writer does not support hijacking")
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}
		_, _ = fmt.Fprint(w, "ok")
	}))
	server.Config.ErrorLog = nil
	server.Start()
	defer server.Close()

	resp, err := doDownloadWithRetry(server.Client(), server.URL, 3)
	if err != nil {
		t.Fatalf("expected retry to recover, got %v", err)
	}
	defer resp.Body.Close()

	body, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil && len(body) == 0 {
		t.Fatalf("read response: %v", err)
	}
	if body != "ok" {
		t.Fatalf("body=%q", body)
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempts=%d want=3", got)
	}
}

func TestDoDownloadWithRetryStopsAtAttemptLimit(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		hijacker := w.(http.Hijacker)
		conn, rw, err := hijacker.Hijack()
		if err != nil {
			t.Fatalf("hijack: %v", err)
		}
		_ = rw.Flush()
		_ = conn.Close()
	}))
	server.Start()
	defer server.Close()

	resp, err := doDownloadWithRetry(server.Client(), server.URL, 3)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected final error")
	}
	if got := attempts.Load(); got != 3 {
		t.Fatalf("attempts=%d want=3", got)
	}
}
