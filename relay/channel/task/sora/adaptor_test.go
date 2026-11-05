package sora

import (
	"encoding/json"
	"strings"
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

func TestConvertToOpenAIVideoCompletedUsesR2PublicURL(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		Data:       []byte(`{"id":"upstream","status":"completed","progress":100,"url":"/outputs/task_upstream.mp4"}`),
		Properties: model.Properties{OriginModelName: "sd2-c7"},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://oss.domie.studio/videos/2026/10/task_public.mp4",
		},
	}

	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	wantURL := "https://oss.domie.studio/videos/2026/10/task_public.mp4"
	require.Equal(t, wantURL, gjson.GetBytes(body, "url").String())
	require.Equal(t, wantURL, gjson.GetBytes(body, "video_url").String())
	require.Equal(t, wantURL, gjson.GetBytes(body, "metadata.url").String())
}

func TestNormalizeSoraVideoStatus(t *testing.T) {
	require.Equal(t, "in_progress", normalizeSoraVideoStatus("processing"))
	require.Equal(t, "in_progress", normalizeSoraVideoStatus("running"))
	require.Equal(t, "queued", normalizeSoraVideoStatus("pending"))
	require.Equal(t, "completed", normalizeSoraVideoStatus("success"))
	require.Equal(t, "failed", normalizeSoraVideoStatus("cancelled"))
}

func TestParseTaskResultPreservesCompletedRelativeVideoURL(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{
		"id":"upstream",
		"status":"completed",
		"progress":100,
		"video_url":"/outputs/task_upstream.mp4"
	}`))
	require.NoError(t, err)
	require.Equal(t, model.TaskStatusSuccess, result.Status)
	require.Equal(t, "/outputs/task_upstream.mp4", result.Url)
}

func TestTaskSubmitReqToUpstreamVideoBody(t *testing.T) {
	enabled := false
	body := taskSubmitReqToUpstreamVideoBody(relaycommon.TaskSubmitReq{
		Prompt:            "hello",
		Size:              "1080x1920",
		Seconds:           "8",
		InputReference:    "https://example.com/a.png",
		ComplianceEnabled: &enabled,
		ComplianceMode:    "colored-pencil",
	}, "seedance2-c1")

	require.Equal(t, "seedance2-c1", body["model"])
	require.Equal(t, "hello", body["prompt"])
	require.Equal(t, 8, body["duration"])
	require.Equal(t, "1080x1920", body["size"])
	require.Equal(t, "https://example.com/a.png", body["image_url"])
	require.Equal(t, []string{"https://example.com/a.png"}, body["image_refs"])
	require.Equal(t, false, body["compliance_enabled"])
	require.Equal(t, "colored-pencil", body["compliance_mode"])
}

func TestApplyParsedReferencesFillsImagesWhenOnlyAliasPresent(t *testing.T) {
	body := map[string]interface{}{
		"image_refs": []interface{}{"https://cdn.example/a.png", "https://cdn.example/b.png"},
		"audio_refs": []interface{}{"https://cdn.example/voice.mp3"},
	}
	applyParsedReferences(body, relaycommon.TaskSubmitReq{
		ImageRefs: []string{"https://cdn.example/a.png", "https://cdn.example/b.png"},
		AudioRefs: []string{"https://cdn.example/voice.mp3"},
	})
	require.Equal(t, []string{"https://cdn.example/a.png", "https://cdn.example/b.png"}, body["images"])
	require.Equal(t, []string{"https://cdn.example/voice.mp3"}, body["audios"])
}

func TestSanitizeMoyinH3VideoBodyStripsAliasesThatMoyinRejects(t *testing.T) {
	body := map[string]interface{}{
		"model":                     "minimax-h3-syt",
		"prompt":                    "a cat walking",
		"seconds":                   "4",
		"duration":                  4,
		"aspect_ratio":              "16:9",
		"ratio":                     "16:9",
		"size":                      "768p",
		"resolution":                "768p",
		"images":                    []interface{}{"https://cdn.example/a.png"},
		"image_refs":                []interface{}{"https://cdn.example/a.png"},
		"image_url":                 "https://cdn.example/a.png",
		"image_urls":                []interface{}{"https://cdn.example/a.png"},
		"reference_images":          []interface{}{"https://cdn.example/a.png"},
		"audio_refs":                []interface{}{"https://cdn.example/voice.mp3"},
		"compliance_enabled":        true,
		"prompt_compliance_enabled": true,
		"generate_audio":            true,
		"video_refs":                []interface{}{"https://cdn.example/clip.mp4"},
	}
	sanitizeOriginModelUpstreamBody(body, relaycommon.TaskSubmitReq{
		Prompt:      "a cat walking",
		Seconds:     "4",
		Duration:    4,
		AspectRatio: "16:9",
		Images:      []string{"https://cdn.example/a.png"},
		ImageRefs:   []string{"https://cdn.example/a.png"},
		AudioRefs:   []string{"https://cdn.example/voice.mp3"},
	}, "minimax-h3-768p")

	require.Equal(t, "minimax-h3-syt", body["model"])
	require.Equal(t, "a cat walking", body["prompt"])
	require.Equal(t, "4", body["seconds"])
	require.Equal(t, 4, body["duration"])
	require.Equal(t, "16:9", body["aspect_ratio"])
	require.Equal(t, "768p", body["size"])
	require.Equal(t, "768p", body["resolution"])
	require.Equal(t, []string{"https://cdn.example/a.png"}, body["images"])
	require.Equal(t, []string{"https://cdn.example/voice.mp3"}, body["audios"])
	for _, forbidden := range []string{
		"image_refs", "image_url", "image_urls", "reference_images",
		"audio_refs", "video_refs", "ratio",
		"compliance_enabled", "prompt_compliance_enabled", "generate_audio",
	} {
		require.NotContains(t, body, forbidden)
	}
}

func TestSanitizeMoyinH3VideoBodyCopiesImageRefsWhenImagesMissing(t *testing.T) {
	body := map[string]interface{}{
		"model":      "minimax-h3-syt",
		"prompt":     "x",
		"image_refs": []interface{}{"https://cdn.example/a.png"},
	}
	sanitizeOriginModelUpstreamBody(body, relaycommon.TaskSubmitReq{
		ImageRefs: []string{"https://cdn.example/a.png"},
	}, "minimax-h3-768p")
	require.Equal(t, []string{"https://cdn.example/a.png"}, body["images"])
	require.NotContains(t, body, "image_refs")
}

func TestSanitizeWanfeng720pC1ForcesPortraitAndKeepsVideoRefs(t *testing.T) {
	body := map[string]interface{}{
		"model":              "sd2-933-720p-9:16",
		"prompt":             "a cat walking",
		"seconds":            "4",
		"duration":           4,
		"aspect_ratio":       "16:9",
		"ratio":              "16:9",
		"size":               "1280x720",
		"image_refs":         []interface{}{"https://cdn.example/a.png"},
		"video_refs":         []interface{}{"https://cdn.example/clip.mp4"},
		"audio_refs":         []interface{}{"https://cdn.example/voice.mp3"},
		"compliance_enabled": true,
	}
	sanitizeOriginModelUpstreamBody(body, relaycommon.TaskSubmitReq{
		Prompt:      "a cat walking",
		Seconds:     "4",
		Duration:    4,
		AspectRatio: "16:9",
		ImageRefs:   []string{"https://cdn.example/a.png"},
		VideoRefs:   []string{"https://cdn.example/clip.mp4"},
		AudioRefs:   []string{"https://cdn.example/voice.mp3"},
	}, "seedance-2.0-720p-c1")

	require.Equal(t, "sd2-933-720p-9:16", body["model"])
	require.Equal(t, "9:16", body["aspect_ratio"])
	require.Equal(t, "720p", body["resolution"])
	require.Equal(t, []string{"https://cdn.example/a.png"}, body["images"])
	require.Equal(t, []string{"https://cdn.example/clip.mp4"}, body["videos"])
	require.Equal(t, []string{"https://cdn.example/voice.mp3"}, body["audios"])
	require.Equal(t, true, body["generateAudio"])
	for _, forbidden := range []string{"image_refs", "video_refs", "audio_refs", "ratio", "compliance_enabled"} {
		require.NotContains(t, body, forbidden)
	}
}

func TestSanitizeOpenaifitFast720pC1KeepsRatioAndImageURLs(t *testing.T) {
	body := map[string]interface{}{
		"model":              "seedance2.0 fast Deneb",
		"prompt":             "a cat walking",
		"seconds":            "5",
		"duration":           5,
		"aspect_ratio":       "9:16",
		"ratio":              "9:16",
		"size":               "720x1280",
		"generate_audio":     true,
		"generateAudio":      true,
		"image_refs":         []interface{}{"https://cdn.example/a.png"},
		"compliance_enabled": true,
	}
	sanitizeOriginModelUpstreamBody(body, relaycommon.TaskSubmitReq{
		Prompt:      "a cat walking",
		Seconds:     "5",
		Duration:    5,
		AspectRatio: "9:16",
		ImageRefs:   []string{"https://cdn.example/a.png"},
		VideoRefs:   []string{"https://cdn.example/clip.mp4"},
		AudioRefs:   []string{"https://cdn.example/voice.mp3"},
	}, "seedance-2.0-fast-720p-c1")

	require.Equal(t, "seedance2.0 fast Deneb", body["model"])
	require.Equal(t, "9:16", body["ratio"])
	require.Equal(t, "720p", body["resolution"])
	require.Equal(t, "5", body["seconds"])
	require.Equal(t, []string{"https://cdn.example/a.png"}, body["image_urls"])
	require.Equal(t, []string{"https://cdn.example/a.png"}, body["images"])
	require.Equal(t, []string{"https://cdn.example/clip.mp4"}, body["videos"])
	require.Equal(t, []string{"https://cdn.example/voice.mp3"}, body["audios"])
	for _, forbidden := range []string{
		"image_refs", "duration", "aspect_ratio", "size",
		"generate_audio", "generateAudio", "compliance_enabled",
	} {
		require.NotContains(t, body, forbidden)
	}
}

func TestSanitizeXxzFast720pC2DropsVideosKeepsAudio(t *testing.T) {
	body := map[string]interface{}{
		"model":          "521ai-SD-fast-720-xd",
		"prompt":         "a cat walking",
		"seconds":        "8",
		"duration":       8,
		"aspect_ratio":   "9:16",
		"size":           "720x1280",
		"generate_audio": true,
		"image_refs":     []interface{}{"https://cdn.example/a.png"},
		"video_refs":     []interface{}{"https://cdn.example/clip.mp4"},
		"audio_refs":     []interface{}{"https://cdn.example/voice.mp3"},
	}
	sanitizeOriginModelUpstreamBody(body, relaycommon.TaskSubmitReq{
		Prompt:      "a cat walking",
		Seconds:     "8",
		Duration:    8,
		AspectRatio: "9:16",
		ImageRefs:   []string{"https://cdn.example/a.png"},
		VideoRefs:   []string{"https://cdn.example/clip.mp4"},
		AudioRefs:   []string{"https://cdn.example/voice.mp3"},
	}, "seedance-2.0-fast-720p-c2")

	require.Equal(t, "521ai-SD-fast-720-xd", body["model"])
	require.Equal(t, "9:16", body["ratio"])
	require.Equal(t, "720p", body["resolution"])
	require.Equal(t, 8, body["seconds"])
	require.Equal(t, 8, body["duration"])
	require.Equal(t, "a cat walking", body["prompt"])
	content := body["content"].([]map[string]interface{})
	require.Len(t, content, 3)
	require.Equal(t, map[string]interface{}{"type": "text", "text": "a cat walking"}, content[0])
	require.Equal(t, "reference_image", content[1]["role"])
	require.Equal(t, map[string]interface{}{"url": "https://cdn.example/a.png"}, content[1]["image_url"])
	require.Equal(t, "reference_audio", content[2]["role"])
	for _, forbidden := range []string{
		"image_refs", "video_refs", "audio_refs", "aspect_ratio", "size", "videos",
		"images", "image_urls", "input_reference", "audios", "audio_urls", "generate_audio",
	} {
		require.NotContains(t, body, forbidden)
	}
}

func TestResponseTaskAcceptsNumericSeconds(t *testing.T) {
	var task responseTask
	require.NoError(t, json.Unmarshal([]byte(`{"id":"x","status":"queued","seconds":8}`), &task))
	require.Equal(t, jsonString("8"), task.Seconds)
	require.NoError(t, json.Unmarshal([]byte(`{"id":"x","status":"queued","seconds":"5"}`), &task))
	require.Equal(t, jsonString("5"), task.Seconds)
}

func TestSanitizeMoyinH3VideoBodyLeavesOtherModelsUntouched(t *testing.T) {
	body := map[string]interface{}{
		"image_refs":         []string{"https://cdn.example/a.png"},
		"compliance_enabled": true,
	}
	sanitizeOriginModelUpstreamBody(body, relaycommon.TaskSubmitReq{
		ImageRefs: []string{"https://cdn.example/a.png"},
	}, "minimax-h3")
	require.Equal(t, []string{"https://cdn.example/a.png"}, body["image_refs"])
	require.Equal(t, true, body["compliance_enabled"])
}

func TestApplyCanonicalVideoControlsConvertsStringBooleanBeforeUpstream(t *testing.T) {
	enabled := true
	promptCompliance := true
	body := map[string]interface{}{
		"compliance_enabled": "true",
		"eye_mask_enabled":   "true",
		"eye_mask_mode":      "grid",
	}
	applyCanonicalVideoControls(body, relaycommon.TaskSubmitReq{
		PromptComplianceEnabled: &promptCompliance,
		ComplianceEnabled:       &enabled,
		ComplianceMode:          "colored-pencil",
	})
	require.Equal(t, true, body["prompt_compliance_enabled"])
	require.Equal(t, true, body["compliance_enabled"])
	require.Equal(t, "colored-pencil", body["compliance_mode"])
	require.NotContains(t, body, "eye_mask_enabled")
	require.NotContains(t, body, "eye_mask_mode")
}

func TestApplySeedance25C2DurationLimitCapsAtFifteen(t *testing.T) {
	body := map[string]interface{}{}
	applySeedance25C2DurationLimit(body, relaycommon.TaskSubmitReq{Duration: 45})
	require.Equal(t, 15, body["duration"])
	require.Equal(t, "15", body["seconds"])

	body = map[string]interface{}{}
	applySeedance25C2DurationLimit(body, relaycommon.TaskSubmitReq{Seconds: "8"})
	require.Equal(t, 8, body["duration"])
	require.Equal(t, "8", body["seconds"])

	body = map[string]interface{}{}
	applySeedance25C2DurationLimit(body, relaycommon.TaskSubmitReq{Duration: 20, Seconds: "20"})
	require.Equal(t, 15, body["duration"])
	require.Equal(t, "15", body["seconds"])

	body = map[string]interface{}{"duration": 10, "seconds": "10"}
	applySeedance25C2DurationLimit(body, relaycommon.TaskSubmitReq{})
	require.NotContains(t, body, "duration")
	require.NotContains(t, body, "seconds")
}

func TestApplySD2C7DurationLimitPassesFifteenSeconds(t *testing.T) {
	body := map[string]interface{}{"seconds": "15"}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Seconds: "15"}, "sd2-c7")
	require.Equal(t, 15, body["duration"])
	require.Equal(t, "15", body["seconds"])

	body = map[string]interface{}{}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Duration: 20}, "sd2-c7")
	require.Equal(t, 15, body["duration"])
	require.Equal(t, "15", body["seconds"])

	body = map[string]interface{}{"seconds": "10"}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{}, "sd2-c7")
	require.Equal(t, "10", body["seconds"])
	require.NotContains(t, body, "duration")
}

func TestApplySD25DurationLimitUsesSecondsAndCapsAtThirty(t *testing.T) {
	body := map[string]interface{}{"seconds": "25"}
	applySD25DurationLimit(body, relaycommon.TaskSubmitReq{Seconds: "25"})
	require.Equal(t, 25, body["duration"])
	require.Equal(t, "25", body["seconds"])

	body = map[string]interface{}{}
	applySD25DurationLimit(body, relaycommon.TaskSubmitReq{Duration: 5})
	require.Equal(t, 5, body["duration"])
	require.Equal(t, "5", body["seconds"])

	body = map[string]interface{}{}
	applySD25DurationLimit(body, relaycommon.TaskSubmitReq{Duration: 45})
	require.Equal(t, 30, body["duration"])
	require.Equal(t, "30", body["seconds"])

	body = map[string]interface{}{"duration": 10, "seconds": "10"}
	applySD25DurationLimit(body, relaycommon.TaskSubmitReq{})
	require.NotContains(t, body, "duration")
	require.NotContains(t, body, "seconds")

	body = map[string]interface{}{"duration": 10}
	applySD25DurationLimit(body, relaycommon.TaskSubmitReq{Duration: 10, Seconds: "30"})
	require.Equal(t, 30, body["duration"])
	require.Equal(t, "30", body["seconds"])
}

func TestTaskSubmitDurationPrefersSecondsOverStaleDurationDefault(t *testing.T) {
	require.Equal(t, 30, taskSubmitDuration(relaycommon.TaskSubmitReq{Duration: 10, Seconds: "30"}))
	require.Equal(t, 30, relaycommon.ResolveTaskSubmitDuration(10, "30"))
	require.Equal(t, 10, taskSubmitDuration(relaycommon.TaskSubmitReq{Duration: 10, Seconds: "10"}))
}

func TestUpstreamVideoTaskPrefersJSON(t *testing.T) {
	require.True(t, upstreamVideoTaskPrefersJSON("http://paco-dola2api-er9b9x-dola2api-1:38472"))
	require.False(t, upstreamVideoTaskPrefersJSON("https://api.openai.com"))
}

func TestConvertToOpenAIVideoHidesNewtakeError(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_public",
		Status:   model.TaskStatusFailure,
		Progress: "100%",
		Data: []byte(`{
			"id":"upstream",
			"status":"failed",
			"error":{"message":"Get \"https://api2.newtake.tv/api/www/tv/login\": TLS handshake timeout"},
			"remote_url":"https://www.newtake.ai/out.mp4",
			"upstream_task_id":"nt-secret"
		}`),
	}
	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	lower := strings.ToLower(string(body))
	for _, forbidden := range []string{"newtake", "api2.", "remote_url", "nt-secret"} {
		require.NotContains(t, lower, forbidden)
	}
	require.Equal(t, "request failed", gjson.GetBytes(body, "error.message").String())
}

func TestConvertToOpenAIVideoHidesBrambleError(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_public",
		Status:   model.TaskStatusFailure,
		Progress: "100%",
		Data: []byte(`{
			"id":"upstream",
			"status":"failed",
			"error":{"message":"Get \"https://api.bramble.video/v1/media/jobs/abc\": timeout"},
			"remote_url":"https://api.bramble.video/v1/media/assets/xyz",
			"upstream_task_id":"brk-secret"
		}`),
	}
	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	lower := strings.ToLower(string(body))
	for _, forbidden := range []string{"bramble", "api.bramble", "remote_url", "brk-secret"} {
		require.NotContains(t, lower, forbidden)
	}
	require.Equal(t, "request failed", gjson.GetBytes(body, "error.message").String())
}

func TestConvertToOpenAIVideoHidesSekoError(t *testing.T) {
	task := &model.Task{
		TaskID:   "task_public",
		Status:   model.TaskStatusFailure,
		Progress: "100%",
		Data: []byte(`{
			"id":"upstream",
			"status":"failed",
			"error":{"message":"sekotv /seko-api/seko-canvas/v1/canvas-node: Invalid operation: invalid duration"}
		}`),
	}
	body, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)
	lower := strings.ToLower(string(body))
	for _, forbidden := range []string{"seko", "sekotv", "canvas-node"} {
		require.NotContains(t, lower, forbidden)
	}
	require.Equal(t, "request failed", gjson.GetBytes(body, "error.message").String())
}
