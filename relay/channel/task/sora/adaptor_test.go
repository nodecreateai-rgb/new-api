package sora

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
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
