package sora

import (
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestNormalizeOpenAIVideoAspectValues(t *testing.T) {
	cases := []struct {
		name       string
		aspectIn   string
		sizeIn     string
		wantAspect string
		wantSize   string
	}{
		{name: "ratio in size", sizeIn: "9:16", wantAspect: "9:16", wantSize: "1080x1920"},
		{name: "portrait dimensions", sizeIn: "1080x1920", wantAspect: "9:16", wantSize: "1080x1920"},
		{name: "explicit aspect", aspectIn: "9:16", wantAspect: "9:16", wantSize: "1080x1920"},
		{name: "landscape dimensions", sizeIn: "1920x1080", wantAspect: "16:9", wantSize: "1920x1080"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotAspect, gotSize := normalizeOpenAIVideoAspectValues(tc.aspectIn, tc.sizeIn)
			if gotAspect != tc.wantAspect || gotSize != tc.wantSize {
				t.Fatalf("got aspect=%q size=%q want aspect=%q size=%q", gotAspect, gotSize, tc.wantAspect, tc.wantSize)
			}
		})
	}
}

func TestNormalizeOpenAIVideoAspectBody(t *testing.T) {
	body := map[string]interface{}{"model": "sd2-c1", "size": "1080x1920"}
	normalizeOpenAIVideoAspectBody(body)
	if body["aspect_ratio"] != "9:16" || body["size"] != "1080x1920" {
		t.Fatalf("body=%v", body)
	}
}

func TestTaskSubmitReqToUpstreamVideoBodyAcceptsRatioAlias(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Model: "seedance-video-standard", Ratio: "9:16"}
	body := taskSubmitReqToUpstreamVideoBody(req, "seedance-2.0-standard")
	normalizeOpenAIVideoAspectBody(body)
	if body["aspect_ratio"] != "9:16" || body["size"] != "1080x1920" {
		t.Fatalf("body=%v", body)
	}
}

func TestTaskSubmitReqToUpstreamVideoBodyAcceptsResolutionAlias(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Model: "seedance-video-fast", Resolution: "1920x1080"}
	body := taskSubmitReqToUpstreamVideoBody(req, "seedance-2.0-fast")
	normalizeOpenAIVideoAspectBody(body)
	if body["aspect_ratio"] != "16:9" || body["size"] != "1920x1080" {
		t.Fatalf("body=%v", body)
	}
}

func TestTaskSubmitReqToUpstreamVideoBodyAcceptsAspectAlias(t *testing.T) {
	req := relaycommon.TaskSubmitReq{Model: "seedance-video-fast", Aspect: "16:9"}
	body := taskSubmitReqToUpstreamVideoBody(req, "seedance-2.0-fast")
	normalizeOpenAIVideoAspectBody(body)
	if body["aspect_ratio"] != "16:9" || body["size"] != "1920x1080" {
		t.Fatalf("body=%v", body)
	}
}

func TestNormalizeOpenAIVideoAspectBodyAcceptsAliases(t *testing.T) {
	tests := []struct {
		name string
		body map[string]interface{}
	}{
		{name: "aspect alias", body: map[string]interface{}{"aspect": "16:9"}},
		{name: "ratio alias", body: map[string]interface{}{"ratio": "16:9"}},
		{name: "resolution alias", body: map[string]interface{}{"resolution": "1920x1080"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			normalizeOpenAIVideoAspectBody(tc.body)
			if tc.body["aspect_ratio"] != "16:9" || tc.body["size"] != "1920x1080" {
				t.Fatalf("body=%v", tc.body)
			}
		})
	}
}

func TestSanitizeOpenAIVideoTaskDataRecursive(t *testing.T) {
	data := []byte(`{"id":"upstream-id","video_url":"https://cdn.oreateai.com/x.mp4","local_path":"/app/outputs/x.mp4","nested":{"account_email":"user@example.com","chat_id":"secret","message":"OreateAI failed at https://cdn.oreateai.com/x"},"videos":[{"url":"https://cdn.oreateai.com/x.mp4","poster":"https://cdn.oreateai.com/p.jpg"}]}`)
	got, err := sanitizeOpenAIVideoTaskData(data)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(got))
	for _, forbidden := range []string{"oreate", "cdn.oreateai.com", "local_path", "account_email", "chat_id", "poster", "video_url"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %s", forbidden, got)
		}
	}
}

func TestTaskSubmitReqToUpstreamVideoBodyAcceptsAudioRefs(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		AudioURL:        "https://assets.example/audio.mp3",
		ReferenceAudios: []string{"https://assets.example/audio2.wav"},
	}
	body := taskSubmitReqToUpstreamVideoBody(req, "seedance-2.0-standard")
	audios, ok := body["audio_refs"].([]string)
	if !ok || len(audios) != 2 || body["audio_url"] != audios[0] || body["reference_audio"] != audios[0] {
		t.Fatalf("audios body=%v", body)
	}
	for _, key := range []string{"audio_urls", "audios", "reference_audios"} {
		refs, ok := body[key].([]string)
		if !ok || len(refs) != 2 {
			t.Fatalf("%s body=%v", key, body)
		}
	}
}

func TestTaskSubmitReqToUpstreamVideoBodyForSD25FansOutAllAudioAliases(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		AudioURL:        "https://assets.example/audio.mp3",
		AudioRefs:       []string{"https://assets.example/audio.mp3"},
		ReferenceAudios: []string{"https://assets.example/audio2.wav"},
	}
	body := taskSubmitReqToUpstreamVideoBody(req, "seedance-2.5-omni")
	audios, ok := body["audio_refs"].([]string)
	if !ok || len(audios) != 2 {
		t.Fatalf("sd2.5 audio_refs body=%v", body)
	}
	for _, key := range []string{"audio_urls", "audios", "reference_audios"} {
		refs, ok := body[key].([]string)
		if !ok || len(refs) != 2 {
			t.Fatalf("sd2.5 %s body=%v", key, body)
		}
	}
	if body["audio_url"] != audios[0] || body["reference_audio"] != audios[0] {
		t.Fatalf("sd2.5 single audio aliases body=%v", body)
	}
}

func TestTaskSubmitReqToUpstreamVideoBodyAcceptsImageAndVideoRefs(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		ImageURL:        "https://assets.example/image.png",
		ReferenceImages: []string{"https://assets.example/image2.png"},
		VideoURL:        "https://assets.example/video.mp4",
		ReferenceVideos: []string{"https://assets.example/video2.mp4"},
	}
	body := taskSubmitReqToUpstreamVideoBody(req, "seedance-2.0")
	images, ok := body["image_refs"].([]string)
	if !ok || len(images) != 2 {
		t.Fatalf("images body=%v", body)
	}
	if _, exists := body["image_url"]; exists {
		t.Fatalf("multi-reference body must not duplicate the first image in image_url: %v", body)
	}
	videos, ok := body["video_refs"].([]string)
	if !ok || len(videos) != 2 || body["video_url"] != videos[0] || body["reference_video"] != videos[0] {
		t.Fatalf("videos body=%v", body)
	}
	for _, key := range []string{"video_urls", "videos", "reference_videos"} {
		refs, ok := body[key].([]string)
		if !ok || len(refs) != 2 {
			t.Fatalf("%s body=%v", key, body)
		}
	}
}
