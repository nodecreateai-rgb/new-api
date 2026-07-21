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
