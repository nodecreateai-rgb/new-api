package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolvePossiblyRelativeVideoURL(t *testing.T) {
	t.Run("relative output path uses channel base URL", func(t *testing.T) {
		got := resolvePossiblyRelativeVideoURL("/outputs/task_abc.mp4", "http://provider:38472")
		want := "http://provider:38472/outputs/task_abc.mp4"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("absolute URL unchanged", func(t *testing.T) {
		got := resolvePossiblyRelativeVideoURL("https://cdn.example.com/task.mp4", "http://provider:38472")
		want := "https://cdn.example.com/task.mp4"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("data URL unchanged", func(t *testing.T) {
		got := resolvePossiblyRelativeVideoURL("data:video/mp4;base64,AAAA", "http://provider:38472")
		want := "data:video/mp4;base64,AAAA"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}

func TestUpstreamVideoOutputURL(t *testing.T) {
	got := upstreamVideoOutputURL("http://paco-dola2api-er9b9x-dola2api-1:38472", "1311ad37-7251-4fad-8c08-4fafd9bebe96")
	want := "http://paco-dola2api-er9b9x-dola2api-1:38472/outputs/task_1311ad37-7251-4fad-8c08-4fafd9bebe96.mp4"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := upstreamVideoOutputURL("https://api.openai.com", "abc"); got != "" {
		t.Fatalf("openai base should not map to outputs URL, got %q", got)
	}
}

func TestVideoOutputCacheBypass(t *testing.T) {
	baseURL := "https://sd2-c7.dopio.cyou"
	videoURL := "https://sd2-c7.dopio.cyou/outputs/task_abc.mp4"
	if !shouldRetryVideoWithCacheBypass(videoURL, baseURL) {
		t.Fatal("trusted /outputs video URL should be retried with cache bypass")
	}
	if shouldRetryVideoWithCacheBypass("https://other.example/outputs/task_abc.mp4", baseURL) {
		t.Fatal("untrusted video URL must not be retried with cache bypass")
	}
	got := addVideoCacheBypass(videoURL, 1783756372)
	want := "https://sd2-c7.dopio.cyou/outputs/task_abc.mp4?_cb=1783756372"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveUpstreamTaskVideoURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/videos/task_upstream" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected authorization %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"task_upstream","status":"completed","video_url":"https://cdn.example.com/result.mp4"}`)
	}))
	defer server.Close()

	got := resolveUpstreamTaskVideoURL(context.Background(), server.Client(), server.URL, "secret", "task_upstream")
	if got != "https://cdn.example.com/result.mp4" {
		t.Fatalf("got %q", got)
	}
}

func TestPrivateVideoContentURL(t *testing.T) {
	got := privateVideoContentURL("http://video-generation-upstream:38983", "task_abc")
	want := "http://video-generation-upstream:38983/v1/videos/task/task_abc/content"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := privateVideoContentURL("http://other-upstream:38983", "task_abc"); got != "" {
		t.Fatalf("non-mediaio upstream must use the existing output route, got %q", got)
	}
}
