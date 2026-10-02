package relay

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestTaskModel2DtoRedactsPrivateVideoTask(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		FailReason: "OreateAI failed for user@example.com at https://cdn.oreateai.com/x.mp4",
		Data:       []byte(`{"id":"upstream-id","video_url":"https://cdn.oreateai.com/x.mp4","local_path":"/app/x.mp4","nested":{"chat_id":"secret"}}`),
	}
	dto := TaskModel2Dto(task)
	combined := strings.ToLower(dto.FailReason + dto.ResultURL + string(dto.Data))
	for _, forbidden := range []string{"oreate", "cdn.oreateai.com", "example.com", "local_path", "chat_id", "upstream-id"} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("leaked %q in %s", forbidden, combined)
		}
	}
	if dto.ResultURL == "" || !strings.Contains(dto.ResultURL, "/v1/videos/task_public/content") {
		t.Fatalf("result_url=%q", dto.ResultURL)
	}
}

func TestTaskModel2DtoHidesChannel(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		ChannelId:  51,
		FailReason: "Roboneo Seedance Mini failed via ribbi.ai",
	}
	dto := TaskModel2Dto(task)
	if dto.ChannelId != 0 {
		t.Fatalf("channel_id=%d", dto.ChannelId)
	}
	lower := strings.ToLower(dto.FailReason)
	for _, forbidden := range []string{"roboneo", "ribbi"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, dto.FailReason)
		}
	}
}

func TestTaskModel2DtoRedactsSekoVideoTask(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_liPjh1CAc6NItGvylLCdwURyFLRu3sDO",
		Status:     model.TaskStatusFailure,
		FailReason: "sekotv /seko-api/seko-canvas/v1/canvas-node: Invalid operation: invalid duration",
		Data:       []byte(`{"id":"upstream-id","error":{"message":"video service /seko-api/seko-canvas/v1/canvas-node: Invalid operation: invalid duration"},"account_email":"user@example.com"}`),
	}
	dto := TaskModel2Dto(task)
	combined := strings.ToLower(dto.FailReason + dto.ResultURL + string(dto.Data))
	for _, forbidden := range []string{"seko", "sekotv", "example.com", "upstream-id", "account_email"} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("leaked %q in %s", forbidden, combined)
		}
	}
	if dto.FailReason != "request failed" {
		t.Fatalf("fail_reason=%q", dto.FailReason)
	}
}

func TestTaskModel2DtoRedactsMyEditVideoTask(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		FailReason: "MyEdit2API failed at https://myedit.online/x.mp4 via CyberLink",
		Data:       []byte(`{"id":"upstream-id","video_url":"https://myedit.online/x.mp4","account_email":"user@example.com","message":"CyberLink MyEdit failure"}`),
	}
	dto := TaskModel2Dto(task)
	combined := strings.ToLower(dto.FailReason + dto.ResultURL + string(dto.Data))
	for _, forbidden := range []string{"myedit", "cyberlink", "example.com", "video_url", "upstream-id"} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("leaked %q in %s", forbidden, combined)
		}
	}
	if dto.ResultURL == "" || !strings.Contains(dto.ResultURL, "/v1/videos/task_public/content") {
		t.Fatalf("result_url=%q", dto.ResultURL)
	}
}

func TestTaskModel2DtoReturnsR2PublicURL(t *testing.T) {
	task := &model.Task{
		TaskID: "task_public",
		Status: model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://oss.domie.studio/videos/2026/10/task_public.mp4",
		},
	}
	dto := TaskModel2Dto(task)
	if dto.ResultURL != "https://oss.domie.studio/videos/2026/10/task_public.mp4" {
		t.Fatalf("result_url=%q", dto.ResultURL)
	}
}
