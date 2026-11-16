package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
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

func TestProviderMediaExpired(t *testing.T) {
	err := fmt.Errorf("upstream returned status 410 for https://xingapi.top/api/v1/media-content/token")
	if !providerMediaExpired(err) {
		t.Fatal("signed media 410 means the provider file is gone")
	}
	if providerMediaExpired(fmt.Errorf("upstream returned status 404 for https://xingapi.top/outputs/task_x.mp4")) {
		t.Fatal("a missing speculative path is not an expired file")
	}
	if providerMediaExpired(nil) {
		t.Fatal("nil error is not expired")
	}
}

func TestIsLikelyPlayableVideoResponse(t *testing.T) {
	html := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}}
	if isLikelyPlayableVideoResponse(html) {
		t.Fatal("SPA HTML must not count as playable video")
	}
	mp4 := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"video/mp4"}}}
	if !isLikelyPlayableVideoResponse(mp4) {
		t.Fatal("video/mp4 must count as playable")
	}
	octet := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/octet-stream"}}}
	if !isLikelyPlayableVideoResponse(octet) {
		t.Fatal("octet-stream must count as playable")
	}
}

func TestFetchFirstAvailableVideoSkipsHTMLOutputs(t *testing.T) {
	var hits []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		switch r.URL.Path {
		case "/outputs/task_abc.mp4":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("ETag", `"spa"`)
			_, _ = io.WriteString(w, "<!doctype html><html></html>")
		case "/api/v1/media-content/abc":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte{0x00, 0x00, 0x00, 0x20, 'f', 't', 'y', 'p'})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodGet, "http://example/", nil)
	if err != nil {
		t.Fatal(err)
	}
	c.Request = req
	task := &model.Task{TaskID: "task_local"}
	err = fetchFirstAvailableVideo(c, req, server.Client(), context.Background(), task, server.URL, "secret", []string{
		server.URL + "/outputs/task_abc.mp4",
		server.URL + "/api/v1/media-content/abc",
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(hits) < 2 || hits[0] != "/outputs/task_abc.mp4" || hits[1] != "/api/v1/media-content/abc" {
		t.Fatalf("unexpected hit order %#v", hits)
	}
	if ct := recorder.Header().Get("Content-Type"); ct != "video/mp4" {
		t.Fatalf("content-type %q", ct)
	}
	if body := recorder.Body.Bytes(); len(body) < 8 || string(body[4:8]) != "ftyp" {
		t.Fatalf("body %#v", body)
	}
}

func TestUniqueNonEmptyURLs(t *testing.T) {
	got := uniqueNonEmptyURLs([]string{
		"",
		"/outputs/task_abc.mp4",
		"/outputs/task_abc.mp4",
		"http://dola2api:38472/v1/task/task_abc/content",
	})
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
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
