package model

import (
	"os"
	"strings"
	"testing"
)

func TestAiimagetovideoModelStaysOnAfterStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureAiimagetovideo2apiRouting()`,
		`"wan-3.0-480p":                       1.5`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(body, `"wan-3.0-480p"}, permanentlyRetiredPublicModels`) {
		t.Fatal("wan-3.0-480p must not be wiped from ModelPrice on startup")
	}
	routing, err := os.ReadFile("option_aiimagetovideo.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`aivideoChannelName = "Spark Video"`,
		`http://aiimagetovideo2api:38691`,
		`AIIMAGETOVIDEO2API_BASE_URL`,
		`{"wan-3.0-480p":"wan3.0-spark"}`,
		`¥1.5/次，固定480P，2-30秒`,
		`画幅 16:9/9:16/1:1/4:3/3:4`,
		`最多10张图片、0段视频、5段音频`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
