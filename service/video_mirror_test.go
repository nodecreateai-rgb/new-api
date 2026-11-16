package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/r2"
)

func TestProviderVideoNeedsMirror(t *testing.T) {
	base := "https://xingapi.top"
	ch := &model.Channel{BaseURL: &base}
	task := &model.Task{
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			UpstreamResultURL: "https://xingapi.top/api/v1/media-content/token",
			ResultURL:         "https://llm.example/v1/videos/task_abc/content",
		},
	}
	if !providerVideoNeedsMirror(ch, task) {
		t.Fatal("provider media URL must be copied before it expires")
	}
	task.PrivateData.DirectUpstreamVideo = true
	if providerVideoNeedsMirror(ch, task) {
		t.Fatal("direct upstream videos stay on the provider URL")
	}
	task.PrivateData.DirectUpstreamVideo = false
	task.PrivateData.ResultURL = "https://oss.domie.studio/videos/2026/11/task_abc.mp4"
	if providerVideoNeedsMirror(ch, task) {
		t.Fatal("object storage URL is already durable")
	}
	task.PrivateData.ResultURL = "https://llm.example/v1/videos/task_abc/content"
	task.PrivateData.UpstreamResultURL = "https://cdn.example.com/done.mp4"
	if providerVideoNeedsMirror(ch, task) || providerVideoNeedsMirror(nil, task) {
		t.Fatal("a public CDN file does not need a copy")
	}
	task.Platform = constant.TaskPlatformImage
	task.PrivateData.UpstreamResultURL = "https://xingapi.top/api/v1/media-content/token"
	if providerVideoNeedsMirror(ch, task) {
		t.Fatal("image tasks are not video files")
	}
}

func TestMirrorProviderVideoTaskStoresFile(t *testing.T) {
	t.Setenv("R2_ACCESS_KEY_ID", "id")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	var mediaHits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/videos/videoprep_x":
			if got := r.Header.Get("Authorization"); got != "Bearer secret" {
				t.Errorf("task lookup auth %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"video_url":"` + serverURL(r) + `/api/v1/media-content/fresh"}`))
		case r.URL.Path == "/api/v1/media-content/fresh":
			mediaHits++
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("signed media link must be fetched without the channel key, got %q", got)
			}
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	origUpload := uploadGeneratedVideo
	origSave := saveMirroredVideoResult
	t.Cleanup(func() {
		uploadGeneratedVideo = origUpload
		saveMirroredVideoResult = origSave
	})
	var saved string
	uploadGeneratedVideo = func(ctx context.Context, cfg r2.Config, key string, body []byte) (string, error) {
		if !strings.Contains(string(body), "ftyp") {
			t.Fatalf("body %q", body)
		}
		return cfg.PublicURL(key), nil
	}
	saveMirroredVideoResult = func(task *model.Task, publicURL string) error {
		saved = publicURL
		task.PrivateData.ResultURL = publicURL
		return nil
	}

	base := server.URL
	task := &model.Task{
		TaskID: "task_abc",
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			Key:               "secret",
			UpstreamTaskID:    "videoprep_x",
			UpstreamResultURL: server.URL + "/api/v1/media-content/expired",
			ResultURL:         "https://llm.example/v1/videos/task_abc/content",
		},
	}
	ch := &model.Channel{Key: "channel", BaseURL: &base}
	publicURL, err := mirrorProviderVideoTask(context.Background(), task, ch)
	if err != nil {
		t.Fatal(err)
	}
	if publicURL != saved {
		t.Fatalf("returned %q saved %q", publicURL, saved)
	}
	if mediaHits != 1 {
		t.Fatalf("media hits %d", mediaHits)
	}
	if !strings.HasPrefix(saved, "https://oss.domie.studio/videos/") || !strings.HasSuffix(saved, "/task_abc.mp4") {
		t.Fatalf("saved %q", saved)
	}
}

func TestMirrorProviderVideoTaskExpired(t *testing.T) {
	t.Setenv("R2_ACCESS_KEY_ID", "id")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/videos/videoprep_x" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"video_url":"` + serverURL(r) + `/api/v1/media-content/fresh"}`))
			return
		}
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"error":{"code":"VIDEO_CONTENT_EXPIRED"}}`))
	}))
	defer server.Close()

	origSave := saveMirroredVideoResult
	t.Cleanup(func() { saveMirroredVideoResult = origSave })
	saveMirroredVideoResult = func(task *model.Task, publicURL string) error {
		t.Fatal("expired video must not be stored")
		return nil
	}
	base := server.URL
	task := &model.Task{
		TaskID: "task_old",
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID:    "videoprep_x",
			UpstreamResultURL: server.URL + "/api/v1/media-content/old",
			ResultURL:         "https://llm.example/v1/videos/task_old/content",
		},
	}
	_, err := mirrorProviderVideoTask(context.Background(), task, &model.Channel{Key: "secret", BaseURL: &base})
	if !errors.Is(err, errProviderVideoExpired) {
		t.Fatalf("got %v", err)
	}
}

func TestMirrorProviderVideoTaskFallsBackToContentURL(t *testing.T) {
	t.Setenv("R2_ACCESS_KEY_ID", "id")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	var contentHits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/videos/videoprep_x":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"completed"}`))
		case strings.Contains(r.URL.Path, "/media-content/"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<!doctype html><html></html>"))
		case r.URL.Path == "/v1/videos/videoprep_x/content":
			contentHits++
			if got := r.Header.Get("Authorization"); got != "Bearer secret" {
				t.Errorf("content auth %q", got)
			}
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	origUpload := uploadGeneratedVideo
	origSave := saveMirroredVideoResult
	t.Cleanup(func() {
		uploadGeneratedVideo = origUpload
		saveMirroredVideoResult = origSave
	})
	uploadGeneratedVideo = func(ctx context.Context, cfg r2.Config, key string, body []byte) (string, error) {
		return cfg.PublicURL(key), nil
	}
	var saved string
	saveMirroredVideoResult = func(task *model.Task, publicURL string) error {
		saved = publicURL
		task.PrivateData.ResultURL = publicURL
		return nil
	}

	base := server.URL
	task := &model.Task{
		TaskID: "task_html",
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			Key:               "secret",
			UpstreamTaskID:    "videoprep_x",
			UpstreamResultURL: server.URL + "/api/v1/media-content/page",
			ResultURL:         "https://llm.example/v1/videos/task_html/content",
		},
	}
	publicURL, err := mirrorProviderVideoTask(context.Background(), task, &model.Channel{Key: "channel", BaseURL: &base})
	if err != nil {
		t.Fatal(err)
	}
	if contentHits != 1 {
		t.Fatalf("content hits %d", contentHits)
	}
	if publicURL == "" || publicURL != saved {
		t.Fatalf("public %q saved %q", publicURL, saved)
	}
}

func TestStoreProviderVideoOnceSharesOneDownload(t *testing.T) {
	t.Setenv("R2_ACCESS_KEY_ID", "id")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var contentHits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/videos/videoprep_share":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"completed"}`))
		case strings.Contains(r.URL.Path, "/media-content/"):
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<!doctype html>"))
		case r.URL.Path == "/v1/videos/videoprep_share/content":
			contentHits++
			once.Do(func() { close(started) })
			<-release
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	origUpload := uploadGeneratedVideo
	origSave := saveMirroredVideoResult
	t.Cleanup(func() {
		uploadGeneratedVideo = origUpload
		saveMirroredVideoResult = origSave
		providerVideoPullAfter.Delete("task_share")
		providerVideoMirrorStop.Delete("task_share")
	})
	uploadGeneratedVideo = func(ctx context.Context, cfg r2.Config, key string, body []byte) (string, error) {
		return cfg.PublicURL(key), nil
	}
	saveMirroredVideoResult = func(task *model.Task, publicURL string) error {
		task.PrivateData.ResultURL = publicURL
		return nil
	}

	base := server.URL
	newTask := func() *model.Task {
		return &model.Task{
			TaskID: "task_share",
			Status: model.TaskStatusSuccess,
			PrivateData: model.TaskPrivateData{
				Key:               "secret",
				UpstreamTaskID:    "videoprep_share",
				UpstreamResultURL: server.URL + "/api/v1/media-content/page",
				ResultURL:         "https://llm.example/v1/videos/task_share/content",
			},
		}
	}
	ch := &model.Channel{Key: "channel", BaseURL: &base}
	var wg sync.WaitGroup
	urls := make([]string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			urls[i] = StoreProviderVideoOnce(context.Background(), newTask(), ch)
		}(i)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("download did not start")
	}
	close(release)
	wg.Wait()
	if contentHits != 1 {
		t.Fatalf("content hits %d", contentHits)
	}
	if urls[0] == "" || urls[0] != urls[1] {
		t.Fatalf("urls %#v", urls)
	}
}

func TestScheduleProviderVideoMirrorSkipsWithoutStorage(t *testing.T) {
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	if ScheduleProviderVideoMirror("task_abc") {
		t.Fatal("mirror must not start when object storage is disabled")
	}
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
}
