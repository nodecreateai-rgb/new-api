package model

import (
	"os"
	"strings"
	"testing"
)

func TestOpenaifitModelsListedWithPrices(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureOpenaifitRouting()`,
		`"seedance-2.0-fast-720p-c1":          1.8,`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	data, err := os.ReadFile("option_openaifit.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`openaifitChannelName = "Video Fast 720p C1"`,
		`"seedance-2.0-fast-720p-c1":"seedance2.0 fast Deneb"`,
		`https://openai.fit`,
		`¥1.8/次，固定720P，5/10/15秒`,
		`最多9张图片，不支持参考视频和参考音频`,
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
	for _, forbidden := range []string{"openai.fit", "openaifit", "deneb", "playad"} {
		if strings.Contains(desc, forbidden) {
			t.Fatalf("public descriptions must not name the upstream: %s", forbidden)
		}
	}
}

func TestOpenaifitModelSurvivesRestartCleanup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	ensureAt := strings.Index(body, "ensureOpenaifitRouting()")
	retireAt := strings.Index(body, "retireDeadVideoModels()")
	if ensureAt < 0 || retireAt < 0 || ensureAt < retireAt {
		t.Fatal("ensureOpenaifitRouting must run on every start, after retireDeadVideoModels")
	}
	if strings.Contains(body, `"seedance-2.0-fast-720p-c1"`) && strings.Contains(body, "permanentlyRetiredPublicModels") {
		retired := body[strings.Index(body, "var permanentlyRetiredPublicModels"):]
		retired = retired[:strings.Index(retired, "}")]
		if strings.Contains(retired, `"seedance-2.0-fast-720p-c1"`) {
			t.Fatal("seedance-2.0-fast-720p-c1 must not be permanently retired")
		}
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	if strings.Contains(dead, `"seedance-2.0-fast-720p-c1"`) {
		t.Fatal("seedance-2.0-fast-720p-c1 must not be in deadVideoModels")
	}
	if strings.Contains(dead, `"Video Fast 720p C1"`) {
		t.Fatal("Video Fast 720p C1 must not be in deadVideoChannels")
	}
}
