package model

import (
	"os"
	"strings"
	"testing"
)

func TestMoyinModelsListedWithPrices(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureMoyinRouting()`,
		`"minimax-h3-768p":                    0.5,`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	data, err := os.ReadFile("option_moyin.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`moyinChannelName = "Video H3 768p"`,
		`"minimax-h3-768p":"minimax-h3-syt"`,
		`https://ai.moyinai.com/api`,
		`¥0.5/次，固定768P输出`,
		`最多9张图片、0段视频、3段音频`,
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
	if strings.Contains(strings.ToLower(s[descStart:descEnd]), "moyin") {
		t.Fatal("public descriptions must not name the upstream")
	}
}

func TestMoyinModelSurvivesRestartCleanup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	ensureAt := strings.Index(body, "ensureMoyinRouting()")
	retireAt := strings.Index(body, "retireDeadVideoModels()")
	if ensureAt < 0 || retireAt < 0 || ensureAt > retireAt {
		t.Fatal("ensureMoyinRouting must run on every start, before retireDeadVideoModels")
	}
	if strings.Contains(body, `"minimax-h3-768p"`) && strings.Contains(body, "permanentlyRetiredPublicModels") {
		retired := body[strings.Index(body, "var permanentlyRetiredPublicModels"):]
		retired = retired[:strings.Index(retired, "}")]
		if strings.Contains(retired, `"minimax-h3-768p"`) {
			t.Fatal("minimax-h3-768p must not be permanently retired")
		}
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	if strings.Contains(dead, `"minimax-h3-768p"`) {
		t.Fatal("minimax-h3-768p must not be in deadVideoModels")
	}
	if strings.Contains(dead, `"Video H3 768p"`) {
		t.Fatal("Video H3 768p must not be in deadVideoChannels")
	}
}
