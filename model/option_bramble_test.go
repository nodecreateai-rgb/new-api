package model

import (
	"os"
	"strings"
	"testing"
)

func TestBrambleModelsStayOnAfterStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureBramble2apiRouting()`,
		`"wan3.0-video-prime":                 2,`,
		`"seedance-2.5":                       4,`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(body, `"seedance-2.5"}, permanentlyRetiredPublicModels`) {
		t.Fatal("seedance-2.5 must not be wiped from ModelPrice on startup")
	}
	publish := strings.Index(body, `ensureBramble2apiRouting()`)
	retire := strings.Index(body, `retireDeadVideoModels()`)
	if publish < 0 || retire < 0 || publish < retire {
		t.Fatal("bramble models must be published after the last catalog retire step")
	}

	routing, err := os.ReadFile("option_bramble.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`brambleChannelName = "Bramble Video"`,
		`http://bramble2api:38710`,
		`BRAMBLE2API_BASE_URL`,
		`{"wan3.0-video-prime":"wan3.0-video-prime","seedance-2.5":"seedance-2.5"}`,
		`¥2/次，原生720P，1-30秒`,
		`¥4/次，原生480P，1-30秒`,
		`最多10张图片、0段视频、5段音频`,
		`不卡人脸，最多30张图片、0段视频、10段音频`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	desc := s[strings.Index(s, "modelDescriptions"):strings.Index(s, "endpoint :=")]
	if strings.Contains(strings.ToLower(desc), "bramble") {
		t.Fatal("public descriptions must not name the upstream")
	}
}
