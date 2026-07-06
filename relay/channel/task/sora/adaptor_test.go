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

func TestConvertToOpenAIVideoCompletedUsesAuthenticatedContentProxy(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_public",
		Status:   model.TaskStatusSuccess,
		Progress: "100%",
		Data: []byte(`{
			"id":"upstream",
			"task_id":"upstream",
			"object":"video",
			"model":"seedance2-c1",
			"status":"completed",
			"progress":100,
			"url":"/outputs/task_upstream.mp4",
			"video_url":"/outputs/task_upstream.mp4",
			"videos":[{"url":"/outputs/task_upstream.mp4"}]
		}`),
		Properties: model.Properties{OriginModelName: "sd2-c7"},
	}

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	wantURL := "http://localhost:3000/v1/videos/task_public/content"
	require.Equal(t, wantURL, gjson.GetBytes(body, "url").String())
	require.Equal(t, wantURL, gjson.GetBytes(body, "video_url").String())
	require.Equal(t, wantURL, gjson.GetBytes(body, "metadata.url").String())
	require.Equal(t, wantURL, gjson.GetBytes(body, "videos.0.url").String())
	require.Equal(t, "sd2-c7", gjson.GetBytes(body, "model").String())
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

func TestUpstreamVideoTaskPrefersJSON(t *testing.T) {
	require.True(t, upstreamVideoTaskPrefersJSON("http://paco-dola2api-er9b9x-dola2api-1:38472"))
	require.False(t, upstreamVideoTaskPrefersJSON("https://api.openai.com"))
}
