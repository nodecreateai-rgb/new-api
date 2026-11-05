package model

import (
	"os"
	"strings"
	"testing"
)

func TestWanfengModelsListedWithPrices(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureWanfengRouting()`,
		`"seedance-2.0-720p-c1":               1.5,`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	data, err := os.ReadFile("option_wanfeng.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`wanfengChannelName = "Video 720p C1"`,
		`"seedance-2.0-720p-c1":"sd2-933-720p-9:16"`,
		`https://api.pro666.top`,
		`¥1.5/次，固定720P，仅9:16`,
		`最多9张图片、3段视频、3段音频`,
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
	for _, forbidden := range []string{"wanfeng", "pro666", "晚风", "sd2-933"} {
		if strings.Contains(desc, forbidden) {
			t.Fatalf("public descriptions must not name the upstream: %s", forbidden)
		}
	}
}

func TestWanfengModelSurvivesRestartCleanup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	ensureAt := strings.Index(body, "ensureWanfengRouting()")
	retireAt := strings.Index(body, "retireDeadVideoModels()")
	if ensureAt < 0 || retireAt < 0 || ensureAt < retireAt {
		t.Fatal("ensureWanfengRouting must run on every start, after retireDeadVideoModels")
	}
	if strings.Contains(body, `"seedance-2.0-720p-c1"`) && strings.Contains(body, "permanentlyRetiredPublicModels") {
		retired := body[strings.Index(body, "var permanentlyRetiredPublicModels"):]
		retired = retired[:strings.Index(retired, "}")]
		if strings.Contains(retired, `"seedance-2.0-720p-c1"`) {
			t.Fatal("seedance-2.0-720p-c1 must not be permanently retired")
		}
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	if strings.Contains(dead, `"seedance-2.0-720p-c1"`) {
		t.Fatal("seedance-2.0-720p-c1 must not be in deadVideoModels")
	}
	if strings.Contains(dead, `"Video 720p C1"`) {
		t.Fatal("Video 720p C1 must not be in deadVideoChannels")
	}
}
