package sora

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestConvertToOpenAIVideoNormalizesProcessingStatus(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_public",
		Status:   model.TaskStatusInProgress,
		Progress: "49%",
		Data:     []byte(`{"id":"upstream","task_id":"upstream","object":"video","model":"seedance2-c2","status":"processing","progress":0}`),
		Properties: model.Properties{
			OriginModelName: "sd2-c2",
		},
	}

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	require.Equal(t, "task_public", gjson.GetBytes(body, "id").String())
	require.Equal(t, "task_public", gjson.GetBytes(body, "task_id").String())
	require.Equal(t, "in_progress", gjson.GetBytes(body, "status").String())
	require.Equal(t, int64(49), gjson.GetBytes(body, "progress").Int())
	require.Equal(t, "sd2-c2", gjson.GetBytes(body, "model").String())
}

func TestNormalizeSoraVideoStatus(t *testing.T) {
	require.Equal(t, "in_progress", normalizeSoraVideoStatus("processing"))
	require.Equal(t, "in_progress", normalizeSoraVideoStatus("running"))
	require.Equal(t, "queued", normalizeSoraVideoStatus("pending"))
	require.Equal(t, "completed", normalizeSoraVideoStatus("success"))
	require.Equal(t, "failed", normalizeSoraVideoStatus("cancelled"))
}

func TestTaskSubmitReqToUpstreamVideoBody(t *testing.T) {
	body := taskSubmitReqToUpstreamVideoBody(relaycommon.TaskSubmitReq{
		Prompt:         "hello",
		Size:           "1080x1920",
		Seconds:        "8",
		InputReference: "https://example.com/a.png",
	}, "seedance2-c1")

	require.Equal(t, "seedance2-c1", body["model"])
	require.Equal(t, "hello", body["prompt"])
	require.Equal(t, 8, body["duration"])
	require.Equal(t, "1080x1920", body["size"])
	require.Equal(t, "https://example.com/a.png", body["image_url"])
	require.Equal(t, []string{"https://example.com/a.png"}, body["image_refs"])
}

func TestTaskSubmitReqToUpstreamVideoBodyKeepsRefsAndInfersPortrait(t *testing.T) {
	body := taskSubmitReqToUpstreamVideoBody(relaycommon.TaskSubmitReq{
		Prompt:    "真人实拍，9:16画幅，竖屏短剧。",
		ImageRefs: []string{"https://example.com/1.png", "https://example.com/2.png"},
		Images:    []string{"https://example.com/2.png", "https://example.com/3.png"},
		AudioRefs: []string{"https://example.com/a.mp3"},
	}, "seedance-2.0")

	require.Equal(t, []string{"https://example.com/1.png", "https://example.com/2.png", "https://example.com/3.png"}, body["image_refs"])
	require.Equal(t, []string{"https://example.com/a.mp3"}, body["audio_refs"])
	require.Equal(t, "9:16", body["aspect_ratio"])
	require.Equal(t, "1080x1920", body["size"])
}

func TestUpstreamVideoTaskPrefersJSON(t *testing.T) {
	require.True(t, upstreamVideoTaskPrefersJSON("http://paco-dola2api-er9b9x-dola2api-1:38472"))
	require.False(t, upstreamVideoTaskPrefersJSON("https://api.openai.com"))
}
