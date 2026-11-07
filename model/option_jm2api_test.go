package model

import (
	"os"
	"strings"
	"testing"
)

func TestJm2apiModelsListedWithPrices(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureJm2apiRouting()`,
		`ensureVIP15OnEnabledChannels()`,
		`"seedance-2.0-fast-720p-c3":          1.8,`,
		`"seedance-2.0-fast-720p-c3": 1`,
		`"vip15":   1`,
		`"vip15":   "VIP15分组"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	data, err := os.ReadFile("option_jm2api.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`jm2apiChannelName = "Video Fast 720p C3"`,
		`"seedance-2.0-fast-720p-c2":"seedance-2.0-fast"`,
		`"seedance-2.0-fast-720p-c3":"seedance-2.0-fast"`,
		`http://jimeng2api:38695`,
		`JM2API_BASE_URL`,
		`¥1.8/次，固定720P，4-15秒`,
		`最多9张图片、3段音频，不支持参考视频，原生过人脸`,
		`最多9张图片、3段视频、3段音频，原生过人脸`,
		`vip15`,
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
	for _, forbidden := range []string{"jimeng", "jm2api", "即梦"} {
		if strings.Contains(desc, forbidden) {
			t.Fatalf("public descriptions must not name the upstream: %s", forbidden)
		}
	}
}

func TestJm2apiModelSurvivesRestartCleanup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	ensureAt := strings.Index(body, "ensureJm2apiRouting()")
	vip15 := strings.Index(body, "ensureVIP15OnEnabledChannels()")
	retireAt := strings.Index(body, "retireDeadVideoModels()")
	if ensureAt < 0 || vip15 < 0 || retireAt < 0 || ensureAt < retireAt || vip15 < ensureAt {
		t.Fatal("ensureJm2apiRouting must run after retireDeadVideoModels, and vip15 after the channel is published")
	}
	if strings.Contains(body, "permanentlyRetiredPublicModels") {
		retired := body[strings.Index(body, "var permanentlyRetiredPublicModels"):]
		retired = retired[:strings.Index(retired, "}")]
		for _, model := range []string{`"seedance-2.0-fast-720p-c2"`, `"seedance-2.0-fast-720p-c3"`} {
			if strings.Contains(retired, model) {
				t.Fatalf("%s must not be permanently retired", model)
			}
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
	if strings.Contains(dead, `"seedance-2.0-fast-720p-c3"`) {
		t.Fatal("seedance-2.0-fast-720p-c3 must not be in deadVideoModels")
	}
	if strings.Contains(dead, `"Video Fast 720p C3"`) {
		t.Fatal("Video Fast 720p C3 must not be in deadVideoChannels")
	}
}
