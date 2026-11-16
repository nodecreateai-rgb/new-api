package model

import (
	"os"
	"strings"
	"testing"
)

func TestSeedanceC8ModelsListedWithPrices(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureSeedanceC8Routing()`,
		`"seedance-2.0-c8":`,
		`"seedance-2.0-mini-c8":`,
		`"seedance-2.0-c8":                    0.3`,
		`"seedance-2.0-mini-c8":               0.2`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	data, err := os.ReadFile("option_seedance_c8.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`seedanceC8ChannelName = "Video Seedance C8"`,
		`"seedance-2.0-c8":"seedance-2.0"`,
		`"seedance-2.0-mini-c8":"seedance-2.0-mini"`,
		`http://172.17.0.1:38698`,
		`¥0.3/次，最长15秒，最多9张图片、0段视频、0段音频，支持原生人脸`,
		`¥0.2/次，最长15秒，最多9张图片、0段视频、0段音频，支持原生人脸`,
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
	if strings.Contains(strings.ToLower(s[descStart:descEnd]), "doubao") || strings.Contains(strings.ToLower(s[descStart:descEnd]), "guest") {
		t.Fatal("public descriptions must not name the upstream")
	}
}

func TestSeedanceC8ModelsSurviveRestartCleanup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	ensureAt := strings.Index(body, "ensureSeedanceC8Routing()")
	retireAt := strings.Index(body, "retireDeadVideoModels()")
	if ensureAt < 0 || retireAt < 0 || ensureAt > retireAt {
		t.Fatal("ensureSeedanceC8Routing must run on every start, before retireDeadVideoModels")
	}
	retired := body[strings.Index(body, "var permanentlyRetiredPublicModels"):]
	retired = retired[:strings.Index(retired, "}")]
	for _, name := range []string{"seedance-2.0-c8", "seedance-2.0-mini-c8"} {
		if strings.Contains(retired, `"`+name+`"`) {
			t.Fatalf("%s must not be permanently retired", name)
		}
	}
	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	for _, name := range []string{`"seedance-2.0-c8"`, `"seedance-2.0-mini-c8"`, `"Video Seedance C8"`} {
		if strings.Contains(dead, name) {
			t.Fatalf("%s must not be withdrawn", name)
		}
	}
}
