package model

import (
	"os"
	"strings"
	"testing"
)

func TestXxzModelsListedWithPrices(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureXxzRouting()`,
		`"seedance-2.0-fast-720p-c2":          2,`,
		`groupPrices["seedance-2.0-fast-720p-c2"] != 2`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	data, err := os.ReadFile("option_xxz.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`xxzChannelName = "Video Fast 720p C2"`,
		`"seedance-2.0-fast-720p-c2":"521ai-SD-fast-720-xd"`,
		`https://www.521xxz.com`,
		`¥2/次，固定720P，4-15秒`,
		`最多9张图片、3段音频，不支持参考视频，原生过人脸`,
		`ChannelStatusEnabled`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	descStart := strings.Index(s, "modelDescriptions")
	descEnd := strings.Index(s, "endpoint :=")
	if descStart < 0 || descEnd <= descStart {
		t.Fatal("missing modelDescriptions block")
	}
	desc := strings.ToLower(s[descStart:descEnd])
	for _, forbidden := range []string{"521xxz", "521ai", "xxz.com"} {
		if strings.Contains(desc, forbidden) {
			t.Fatalf("public descriptions must not name the upstream: %s", forbidden)
		}
	}
}

func TestXxzModelSurvivesRestartCleanup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	ensureAt := strings.Index(body, "ensureXxzRouting()")
	retireAt := strings.Index(body, "retireDeadVideoModels()")
	ensureJm2 := strings.Index(body, "ensureJm2apiRouting()")
	if ensureAt < 0 || retireAt < 0 || ensureJm2 < 0 || ensureAt < retireAt || ensureAt < ensureJm2 {
		t.Fatal("ensureXxzRouting must run after retireDeadVideoModels and ensureJm2apiRouting so c2 stays on Video Fast 720p C2")
	}
	if strings.Contains(body, `"seedance-2.0-fast-720p-c2"`) && strings.Contains(body, "permanentlyRetiredPublicModels") {
		retired := body[strings.Index(body, "var permanentlyRetiredPublicModels"):]
		retired = retired[:strings.Index(retired, "}")]
		if strings.Contains(retired, `"seedance-2.0-fast-720p-c2"`) {
			t.Fatal("seedance-2.0-fast-720p-c2 must not be permanently retired")
		}
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	if strings.Contains(dead, `"seedance-2.0-fast-720p-c2"`) {
		t.Fatal("seedance-2.0-fast-720p-c2 must not be in deadVideoModels")
	}
	if strings.Contains(dead, `"Video Fast 720p C2"`) {
		t.Fatal("Video Fast 720p C2 must not be in deadVideoChannels")
	}

	jm2, err := os.ReadFile("option_jm2api.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(jm2), `"seedance-2.0-fast-720p-c2"`) {
		t.Fatal("jm2api must not keep seedance-2.0-fast-720p-c2 after it returns to Video Fast 720p C2")
	}
}
