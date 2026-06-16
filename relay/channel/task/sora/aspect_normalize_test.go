package sora

import "testing"

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
