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
