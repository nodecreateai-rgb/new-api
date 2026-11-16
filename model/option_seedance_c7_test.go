package model

import (
	"os"
	"strings"
	"testing"
)

func TestSeedanceC7ModelsListedWithPrices(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureSeedanceC7Routing()`,
		`"seedance-2.0-480p-c7":`,
		`"seedance-2.0-fast-480p-c7":`,
		`"seedance-2.0-mini-480p-c7":`,
		`"seedance-2.0-mini-720p-c7":`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	data, err := os.ReadFile("option_seedance_c7.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`seedanceC7ChannelName = "Video Seedance C7"`,
		`"seedance-2.0-480p-c7":"seedance-2.0"`,
		`"seedance-2.0-fast-480p-c7":"seedance-2.0-fast"`,
		`"seedance-2.0-mini-480p-c7":"seedance-2.0-mini"`,
		`"seedance-2.0-mini-720p-c7":"seedance-2.0-mini-720p"`,
		`http://seele2api:38510`,
		`¥1/次，最长14秒`,
		`¥0.6/次，最长15秒`,
		`¥0.5/次，最长15秒`,
		`¥0.5/次，最长12秒`,
		`最多9张图片、0段视频、3段音频`,
		`不支持原生人脸，可内置人脸，可超分`,
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
	if strings.Contains(strings.ToLower(s[descStart:descEnd]), "seele") {
		t.Fatal("public descriptions must not name the upstream")
	}
}

func TestSeedanceC7ModelsSurviveRestartCleanup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	ensureAt := strings.LastIndex(body, "ensureSeedanceC7Routing()")
	retireAt := strings.LastIndex(body, "retireDeadVideoModels()")
	if ensureAt < 0 || retireAt < 0 || ensureAt < retireAt {
		t.Fatal("ensureSeedanceC7Routing must run on every start, after the final retireDeadVideoModels")
	}
	retired := body[strings.Index(body, "var permanentlyRetiredPublicModels"):]
	retired = retired[:strings.Index(retired, "}")]
	for _, name := range []string{
		"seedance-2.0-480p-c7",
		"seedance-2.0-fast-480p-c7",
		"seedance-2.0-mini-480p-c7",
		"seedance-2.0-mini-720p-c7",
	} {
		if strings.Contains(retired, `"`+name+`"`) {
			t.Fatalf("%s must not be permanently retired", name)
		}
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	for _, name := range []string{
		`"seedance-2.0-480p-c7"`,
		`"seedance-2.0-fast-480p-c7"`,
		`"seedance-2.0-mini-480p-c7"`,
		`"seedance-2.0-mini-720p-c7"`,
		`"Video Seedance C7"`,
	} {
		if strings.Contains(dead, name) {
			t.Fatalf("%s must not be withdrawn", name)
		}
	}
}
