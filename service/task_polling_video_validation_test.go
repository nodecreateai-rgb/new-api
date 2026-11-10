package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestExtractCompletedVideoURLFromOpenAICompatibleTask(t *testing.T) {
	payload := map[string]any{
		"code": "success",
		"data": map[string]any{
			"id":        "task_upstream",
			"task_id":   "task_provider_short",
			"status":    "completed",
			"progress":  100,
			"video_url": "https://cdn.example.com/result.mp4",
		},
	}
	body, err := common.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got := extractCompletedVideoURL(body); got != "https://cdn.example.com/result.mp4" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractCompletedVideoURLRejectsTaskIdentifiers(t *testing.T) {
	body := []byte(`{"data":{"id":"task_upstream","task_id":"task_provider_short","status":"completed"}}`)
	if got := extractCompletedVideoURL(body); got != "" {
		t.Fatalf("task identifier leaked as URL: %q", got)
	}
}

func TestCompletedVideoTimeoutDoesNotBlockFinishedURL(t *testing.T) {
	timeoutErr := fmt.Errorf("%w: read video for validation: context deadline exceeded", errVideoValidationInconclusive)
	if !completedVideoCanFinishWithoutBytes(timeoutErr, "https://cdn.example.com/done.mp4") {
		t.Fatal("finished upstream URL must complete when the download times out")
	}
	if completedVideoCanFinishWithoutBytes(timeoutErr, "") {
		t.Fatal("missing URL must keep retrying")
	}
	if completedVideoCanFinishWithoutBytes(errors.New("zero duration: duration=0 timescale=0"), "https://cdn.example.com/done.mp4") {
		t.Fatal("a parsed invalid video must still fail")
	}
	if completedVideoCanFinishWithoutBytes(timeoutErr, "/outputs/task_public.mp4") {
		t.Fatal("a relative URL is not client-usable until it is resolved")
	}
	if !videoURLNeedsChannelAuth("https://upstream.example/v1/videos/abc/content") {
		t.Fatal("provider content URL must keep the channel key")
	}
	if videoURLNeedsChannelAuth("https://cdn.example.com/done.mp4") {
		t.Fatal("public CDN URL must be fetched without the channel key")
	}
	if !videoURLNeedsChannelAuth("https://xingapi.top/api/v1/media-content/token") {
		t.Fatal("provider media-content URL must be downloaded with the channel key")
	}
	if model.ClientMayReceiveDirectVideoURL("https://xingapi.top/api/v1/media-content/token") {
		t.Fatal("provider media-content URL must not be returned to the client")
	}
	if model.ClientMayReceiveDirectVideoURL("https://xingapi.top/v1/videos/videoprep_x/content") {
		t.Fatal("upstream content URL must not be returned to the client")
	}
	base := "https://xingapi.top"
	ch := &model.Channel{BaseURL: &base}
	if !clientMayKeepUpstreamVideoURL(ch, "https://cdn.example.com/done.mp4") {
		t.Fatal("public CDN URL can be returned")
	}
	if clientMayKeepUpstreamVideoURL(ch, "https://xingapi.top/api/v1/media-content/token") {
		t.Fatal("provider host URL must be stored by the gateway")
	}
	if clientMayKeepUpstreamVideoURL(ch, "https://xingapi.top/api/v1/media-content/token") == clientMayKeepUpstreamVideoURL(ch, "https://cdn.example.com/done.mp4") {
		t.Fatal("provider and public CDN URLs must take different completion paths")
	}
}

func TestSlowFinishedVideoStopsQuickly(t *testing.T) {
	InitHttpClient()
	previous := videoValidationTimeout
	videoValidationTimeout = 200 * time.Millisecond
	t.Cleanup(func() { videoValidationTimeout = previous })

	authSeen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authSeen <- r.Header.Get("Authorization")
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	base := "https://cdn.example.com"
	ch := &model.Channel{Type: constant.ChannelTypeSora, BaseURL: &base, Key: "secret-channel-key"}
	task := &model.Task{TaskID: "task_public", Platform: constant.TaskPlatform("1")}
	result := &relaycommon.TaskInfo{Url: server.URL + "/cdn/done.mp4"}

	started := time.Now()
	_, err := validateCompletedVideo(context.Background(), ch, task, result)
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("validation waited %s; a slow file must not block completion", elapsed)
	}
	if !errors.Is(err, errVideoValidationInconclusive) {
		t.Fatalf("expected inconclusive download, got %v", err)
	}
	if !completedVideoCanFinishWithoutBytes(err, result.Url) {
		t.Fatal("slow CDN URL must still finish the task")
	}
	select {
	case auth := <-authSeen:
		if auth != "" {
			t.Fatalf("CDN download must not send the channel key, got %q", auth)
		}
	default:
		t.Fatal("download never reached the CDN")
	}
}

func TestProviderContentDownloadKeepsChannelKey(t *testing.T) {
	InitHttpClient()
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-a-video"))
	}))
	defer server.Close()

	ch := &model.Channel{Type: constant.ChannelTypeSora, BaseURL: &server.URL, Key: "secret-channel-key"}
	task := &model.Task{TaskID: "task_public", Platform: constant.TaskPlatform("1")}
	result := &relaycommon.TaskInfo{Url: server.URL + "/v1/videos/abc/content"}
	_, _ = validateCompletedVideo(context.Background(), ch, task, result)
	if auth != "Bearer secret-channel-key" {
		t.Fatalf("provider content download auth = %q", auth)
	}
}

func TestValidateCompletedVideoRejectsZeroDurationMP4(t *testing.T) {
	InitHttpClient()
	fixture, err := os.ReadFile("/tmp/task81347.mp4")
	if err != nil {
		t.Skipf("production regression fixture unavailable: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(fixture)
	}))
	defer server.Close()

	ch := &model.Channel{Type: constant.ChannelTypeOpenAI, BaseURL: &server.URL}
	task := &model.Task{TaskID: "task_public", Platform: constant.TaskPlatform("1")}
	result := &relaycommon.TaskInfo{Url: server.URL + "/broken.mp4"}

	if _, err := validateCompletedVideo(context.Background(), ch, task, result); err == nil {
		t.Fatal("expected invalid/zero-duration MP4 to be rejected")
	}
}
