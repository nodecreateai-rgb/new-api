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
		`ensureXingapi111Routing()`,
		`ensureXingapiFastMRouting()`,
		`"seedance-2.0-720p-c2":               2.8,`,
		`"seedance-2.0-720p-c3":               1.8,`,
		`"seedance-2.5-720p-c5":               5,`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbiddenPrice := range []string{
		`"seedance-2.0-fast-720p-c5":`,
		`"seedance-2.5-480p-c5":`,
		`"seedance-2.5-flash-480p-c5":`,
		`"seedance-2.5-flash-720p-c5":`,
	} {
		if strings.Contains(body, forbiddenPrice) {
			t.Fatalf("retired model price must not be enforced: %s", forbiddenPrice)
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
		`xingapi111ChannelName = "Video 720p C6"`,
		`xingapiFastMChannelName = "Video Fast 720p C5"`,
		`retireXingapiChannel(xingapi111ChannelName, xingapi111PublicModels)`,
		`retireXingapiChannel(xingapiFastMChannelName, xingapiFastMPublicModels)`,
		`disableChannelByName(channelName)`,
		`retireMarketplaceModels(publicModels)`,
		`stripModelsFromChannels(publicModels)`,
		`"seedance-2.0-720p-c2":"seedance-933"`,
		`"seedance-2.0-720p-c3":"seedance-933"`,
		`"seedance-2.5-720p-c5":"seedance2.5-V"`,
		`XINGAPI_111_GATEWAY_KEY`,
		`https://xingapi.top`,
		`¥2.8/次，固定720P，4-15秒`,
		`最多9张图片、3段音频，不支持参考视频，原生过人脸`,
		`¥1.8/次，固定720P，仅9:16`,
		`最多9张图片、3段视频、3段音频，原生过人脸`,
		`¥5/次，固定720P，5-30秒`,
		`最多30张图片、10段视频、10段音频，原生过人脸`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, withdrawn := range []string{
		`"seedance-2.5-720p-c6"`,
		`"seedance-2.5-480p-c5"`,
		`seedance2.5-M`,
		`seedance2.5-flash-M`,
		`seedance2.0-fast-M`,
		`XINGAPI_FAST_M_GATEWAY_KEY`,
	} {
		if strings.Contains(s, withdrawn) {
			t.Fatalf("%s must stay withdrawn", withdrawn)
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
	var descriptions []string
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "异步") {
			descriptions = append(descriptions, line)
		}
	}
	if len(descriptions) == 0 {
		t.Fatal("missing model descriptions")
	}
	desc := strings.ToLower(strings.Join(descriptions, "\n"))
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
	ensure111At := strings.Index(body, "ensureXingapi111Routing()")
	ensureFastAt := strings.Index(body, "ensureXingapiFastMRouting()")
	retireAt := strings.Index(body, "retireDeadVideoModels()")
	if ensureAt < 0 || ensure25At < 0 || ensure111At < 0 || ensureFastAt < 0 || retireAt < 0 || ensureAt < retireAt || ensure25At < ensureAt || ensure111At < ensure25At || ensureFastAt < ensure111At {
		t.Fatal("xingapi ensure must run on every start, after retireDeadVideoModels, with 2.5-V after 933, the 111 channel after that, and Fast-M after the 111 channel")
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
	for _, wantDead := range []string{
		`"seedance-2.5-720p-c6"`,
		`"seedance-2.0-fast-720p-c5"`,
		`"seedance-2.5-480p-c5"`,
		`"seedance-2.5-flash-480p-c5"`,
		`"seedance-2.5-flash-720p-c5"`,
		`"Video 720p C6"`,
		`"Video Fast 720p C5"`,
	} {
		if !strings.Contains(dead, wantDead) {
			t.Fatalf("%s must be in the dead video catalog", wantDead)
		}
	}
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

	ratio, err := os.ReadFile("../setting/ratio_setting/model_ratio.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{
		`"seedance-2.0-fast-720p-c5":`,
		`"seedance-2.5-480p-c5":`,
		`"seedance-2.5-flash-480p-c5":`,
		`"seedance-2.5-flash-720p-c5":`,
	} {
		if strings.Contains(string(ratio), gone) {
			t.Fatalf("default ModelPrice must not keep %s", gone)
		}
	}

	compose, err := os.ReadFile("../docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(compose), "XINGAPI_FAST_M_GATEWAY_KEY") {
		t.Fatal("docker-compose.yml must not keep XINGAPI_FAST_M_GATEWAY_KEY startup wiring")
	}
}
