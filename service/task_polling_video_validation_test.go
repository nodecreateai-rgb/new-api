package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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

	if err := validateCompletedVideo(context.Background(), ch, task, result); err == nil {
		t.Fatal("expected invalid/zero-duration MP4 to be rejected")
	}
}
