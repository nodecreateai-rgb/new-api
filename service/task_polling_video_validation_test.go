package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

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
