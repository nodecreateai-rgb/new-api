package model

import (
	"os"
	"strings"
	"testing"
)

func TestXingapiModelsListedWithPrices(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureXingapiRouting()`,
		`ensureXingapi25Routing()`,
		`"seedance-2.0-720p-c2":               2.4,`,
		`"seedance-2.0-720p-c3":               1.5,`,
		`"seedance-2.5-720p-c5":               4.5,`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	data, err := os.ReadFile("option_xingapi.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`xingapiChannelName = "Video 720p C2"`,
		`xingapi25ChannelName = "Video 720p C5"`,
		`"seedance-2.0-720p-c2":"seedance-933"`,
		`"seedance-2.0-720p-c3":"seedance-933"`,
		`"seedance-2.5-720p-c5":"seedance2.5-V"`,
		`XINGAPI_V_GATEWAY_KEY`,
		`XINGAPI_V_API_KEY`,
		`https://xingapi.top`,
		`¥2.4/次，固定720P，4-15秒`,
		`最多9张图片、3段音频，不支持参考视频，原生过人脸`,
		`¥1.5/次，固定720P，仅9:16`,
		`最多9张图片、3段视频、3段音频，原生过人脸`,
		`¥4.5/次，固定720P，5-30秒`,
		`最多30张图片、10段视频、10段音频，原生过人脸`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, `"seedance-2.0-720p-c2,seedance-2.0-720p-c3,seedance-2.5-720p-c5"`) {
		t.Fatal("2.5-V must not share the 933 channel key")
	}
	if strings.Contains(s, "seedance2.0-933-Q") {
		t.Fatal("720p c3 must use the discounted 933 upstream, not 933-Q")
	}
	if strings.Contains(s, "sk-") {
		t.Fatal("channel key must come from env, not a hardcoded secret")
	}
	descStart := strings.Index(s, "modelDescriptions")
	descEnd := strings.Index(s, "endpoint :=")
	if descStart < 0 || descEnd <= descStart {
		t.Fatal("missing modelDescriptions block")
	}
	desc := strings.ToLower(s[descStart:descEnd])
	for _, forbidden := range []string{"xingapi", "星光", "933-q", "933-c", "seedance2.5-v", "seedance-933"} {
		if strings.Contains(desc, forbidden) {
			t.Fatalf("public descriptions must not name the upstream: %s", forbidden)
		}
	}
}

func TestXingapiModelsSurviveRestartCleanup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	ensureAt := strings.Index(body, "ensureXingapiRouting()")
	ensure25At := strings.Index(body, "ensureXingapi25Routing()")
	retireAt := strings.Index(body, "retireDeadVideoModels()")
	if ensureAt < 0 || ensure25At < 0 || retireAt < 0 || ensureAt < retireAt || ensure25At < ensureAt {
		t.Fatal("xingapi ensure must run on every start, after retireDeadVideoModels, with 2.5-V after 933")
	}
	for _, model := range []string{"seedance-2.0-720p-c2", "seedance-2.0-720p-c3", "seedance-2.5-720p-c5"} {
		if strings.Contains(body, `"`+model+`"`) && strings.Contains(body, "permanentlyRetiredPublicModels") {
			retired := body[strings.Index(body, "var permanentlyRetiredPublicModels"):]
			retired = retired[:strings.Index(retired, "}")]
			if strings.Contains(retired, `"`+model+`"`) {
				t.Fatalf("%s must not be permanently retired", model)
			}
		}
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	for _, needle := range []string{
		`"seedance-2.0-720p-c2"`,
		`"seedance-2.0-720p-c3"`,
		`"seedance-2.5-720p-c5"`,
		`"Video 720p C2"`,
		`"Video 720p C5"`,
	} {
		if strings.Contains(dead, needle) {
			t.Fatalf("%s must not be in the dead video catalog", needle)
		}
	}
}
