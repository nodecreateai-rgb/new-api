package common

import (
	"strings"
	"testing"
)

func TestMaskUpstreamProviderInfoOreate(t *testing.T) {
	got := MaskUpstreamProviderInfo("OreateAI2API generation-upstream cdn.oreateai.com sd2-c13.dopio.cyou")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"oreate", "generation-upstream", "cdn.oreateai.com", "sd2-c13.dopio.cyou"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestMaskUpstreamProviderInfoMediaIO(t *testing.T) {
	got := MaskUpstreamProviderInfo("MediaIO2API mediaio-generation-upstream video-generation-upstream paco-mediaio2api-sjfrbl-mediaio2api-1")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"mediaio", "mediaio-generation-upstream", "paco-mediaio2api"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestMaskUpstreamProviderInfoVideoStudio(t *testing.T) {
	got := MaskUpstreamProviderInfo("ribbi2api ribbi.ai Roboneo Seedance Mini Dola2API Seedance Video")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"ribbi", "roboneo", "dola2"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesTransport(t *testing.T) {
	got := PublicErrorMessage("Post \"http://domiex-ribbi-2t6bpb-ribbi2api-1:38691/v1/videos\": dial tcp: connection refused")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"ribbi", "38691", "dial tcp", "http://"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesSekoPath(t *testing.T) {
	got := PublicErrorMessage("sekotv /seko-api/seko-canvas/v1/canvas-node: Invalid operation: invalid duration")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"seko", "sekotv", "canvas-node"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskUpstreamProviderInfoSeko(t *testing.T) {
	got := MaskUpstreamProviderInfo("Seko Seedance Mini sekotv2api www.sekotv.com /seko-api/seko-canvas")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"seko", "sekotv"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestMaskPublicTaskTextCollapsesSekoError(t *testing.T) {
	got := MaskPublicTaskText("message", "video service /seko-api/seko-canvas/v1/canvas-node: Invalid operation: invalid duration")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestPublicErrorMessageHidesManyverse(t *testing.T) {
	got := PublicErrorMessage("manyverse2api manyverse.app asset.manyverse.io finished without a video")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskUpstreamProviderInfoManyverse(t *testing.T) {
	got := MaskUpstreamProviderInfo("Manyverse manyverse2api asset.manyverse.io www.manyverse.app domiex-manyverse-lgqmy2-manyverse2api-1")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"manyverse", "manyverse2api", "manyverse.app", "manyverse.io"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesFlick(t *testing.T) {
	got := PublicErrorMessage("Flick Video flick2api auth.flick.art could not create video")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskUpstreamProviderInfoFlick(t *testing.T) {
	got := MaskUpstreamProviderInfo("Flick Video flick2api auth.flick.art cdn.flick.art domiex-flick-ulmocy-flick2api-1")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"flick", "flick2api"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesNewtake(t *testing.T) {
	got := PublicErrorMessage("Newtake Video newtake2api www.newtake.ai could not create video")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage(`{"blocked":true,"remainingSeconds":300,"blockDurationSeconds":300} (code 1200000421)`)
	if got != "request failed" {
		t.Fatalf("blocked leak %q", got)
	}
}

func TestMaskUpstreamProviderInfoNewtake(t *testing.T) {
	got := MaskUpstreamProviderInfo("Newtake Video newtake2api www.newtake.ai api2.newtake.tv domiex-newtake-wtqoop-newtake2api-1")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"newtake", "newtake2api"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesVibbit(t *testing.T) {
	got := PublicErrorMessage("Vibbit Video vibbit2api api.vibbit.ai finished without a video")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage(`Post "http://vibbit2api:38688/v1/videos": dial tcp: connection refused`)
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskUpstreamProviderInfoVibbit(t *testing.T) {
	got := MaskUpstreamProviderInfo("Vibbit Video Vibbit vibbit2api api.vibbit.ai app.vibbit.ai")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"vibbit", "vibbit2api"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesPlayad(t *testing.T) {
	got := PublicErrorMessage("playad finished without a video")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage(`Post "http://playad2api:38690/v1/videos": dial tcp: connection refused`)
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskUpstreamProviderInfoPlayad(t *testing.T) {
	got := MaskUpstreamProviderInfo("Playad Video Playad playad2api playad-generation-upstream api.playad.ai autopilot.playad.ai")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"playad", "playad2api"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesAiimagetovideo(t *testing.T) {
	got := PublicErrorMessage("aiimagetovideo2api finished without a video")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskUpstreamProviderInfoAiimagetovideo(t *testing.T) {
	got := MaskUpstreamProviderInfo("Spark Video aiimagetovideo2api aiimagetovideo.pro https://aiimagetovideo.pro/x.mp4")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"aiimagetovideo", "spark video"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesFlova(t *testing.T) {
	got := PublicErrorMessage("flova finished without a video")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestPublicErrorMessageHidesXxz(t *testing.T) {
	got := PublicErrorMessage("521xxz 521ai-SD-fast-720-xd failed")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage(`Post "https://www.521xxz.com/v1/videos": dial tcp: connection refused`)
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskUpstreamProviderInfoXxz(t *testing.T) {
	got := MaskUpstreamProviderInfo("521ai-SD-fast-720-xd www.521xxz.com 521ai 521xxz")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"521xxz", "521ai"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesOpenaifit(t *testing.T) {
	got := PublicErrorMessage("openai.fit seedance2.0 fast Deneb failed")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage(`Post "https://openai.fit/v1/videos": dial tcp: connection refused`)
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage("No available channel for model seedance2.0 fast Deneb under group default")
	if got != "request failed" {
		t.Fatalf("model: %q", got)
	}
}

func TestMaskUpstreamProviderInfoOpenaifit(t *testing.T) {
	got := MaskUpstreamProviderInfo("seedance2.0 fast Deneb openai.fit Openaifit Deneb")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"openai.fit", "openaifit", "deneb"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesXingapi(t *testing.T) {
	got := PublicErrorMessage("xingapi seedance-933 failed")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage(`Post "https://xingapi.top/v1/videos": dial tcp: connection refused`)
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage("No available channel for model seedance2.5-V under group default")
	if got != "request failed" {
		t.Fatalf("model: %q", got)
	}
}

func TestMaskUpstreamProviderInfoXingapi(t *testing.T) {
	got := MaskUpstreamProviderInfo("seedance-933 seedance2.0-933-Q seedance2.5-V xingapi.top 星光API")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"xingapi", "星光", "seedance-933", "933-q", "seedance2.5"} {
		if strings.Contains(got, forbidden) || strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesWanfeng(t *testing.T) {
	got := PublicErrorMessage("晚风 API api.pro666.top sd2-933-720p-9:16 failed")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage(`Post "https://api.pro666.top/v1/videos": dial tcp: connection refused`)
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage("No available channel for model sd2-933-720p-9:16 under group svip: the model is claimed by a task plugin")
	if got != "request failed" {
		t.Fatalf("plugin: %q", got)
	}
}

func TestMaskUpstreamProviderInfoWanfeng(t *testing.T) {
	got := MaskUpstreamProviderInfo("晚风 API Wanfeng api.pro666.top www.pro666.top sd2-933-720p-9:16")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"pro666", "晚风", "wanfeng", "sd2-933"} {
		if strings.Contains(got, forbidden) || strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesBramble(t *testing.T) {
	got := PublicErrorMessage("bramble finished without a video")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage(`Post "http://bramble2api:38710/v1/videos": dial tcp: connection refused`)
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskUpstreamProviderInfoBramble(t *testing.T) {
	got := MaskUpstreamProviderInfo("Bramble Video Bramble bramble2api bramble-generation-upstream api.bramble.video")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"bramble", "bramble2api"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestMaskUpstreamProviderInfoFlova(t *testing.T) {
	got := MaskUpstreamProviderInfo("Flova Video Flova flova2api flova-generation-upstream service.flova.tv")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"flova", "flova2api"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesJimeng(t *testing.T) {
	got := PublicErrorMessage("jimeng /mweb/v1/aigc: ret=1015")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage(`Get "https://jimeng.jianying.com/mweb/v1/x?sign=abc": timeout`)
	if got != "request failed" {
		t.Fatalf("link: %q", got)
	}
	got = PublicErrorMessage("即梦 剪映 dreamina_video_novel_video_generate_fast jimeng2api failed")
	if got != "request failed" {
		t.Fatalf("brand: %q", got)
	}
}

func TestMaskUpstreamProviderInfoJimeng(t *testing.T) {
	got := MaskUpstreamProviderInfo("Jimeng jimeng2api jimeng.jianying.com 即梦 剪映 dreamina capcut.com https://lf3-static.bytednsdoc.com/x.mp4")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"jimeng", "jianying", "即梦", "剪映", "dreamina", "capcut", "bytednsdoc", "jm2api"} {
		if strings.Contains(got, forbidden) || strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesIqiyi(t *testing.T) {
	got := PublicErrorMessage("nadouproapi.iqiyi.com 爱奇艺 纳逗 aqy2api failed")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
	got = PublicErrorMessage(`Post "https://nadoupro.iqiyi.com/apis/task": dial tcp`)
	if got != "request failed" {
		t.Fatalf("link: %q", got)
	}
}

func TestMaskUpstreamProviderInfoIqiyi(t *testing.T) {
	got := MaskUpstreamProviderInfo("iQIYI nadoupro nadouproapi.iqiyi.com aqy2api 爱奇艺 纳逗Pro https://www.iqiyi.com/v.mp4")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"iqiyi", "nadou", "aqy2api", "爱奇艺", "纳逗"} {
		if strings.Contains(got, forbidden) || strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesKsyun(t *testing.T) {
	got := PublicErrorMessage("Ksyun2API kspmas.ksyun.com kingsoft 金山云 failed")
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskUpstreamProviderInfoKsyun(t *testing.T) {
	got := MaskUpstreamProviderInfo("Ksyun Seedance 2.5 ksyun2api kspmas.ksyun.com Kingsoft 金山云")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"ksyun", "kspmas", "kingsoft", "金山云"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestMaskUpstreamProviderInfoFotor(t *testing.T) {
	got := MaskUpstreamProviderInfo("Fotor Video fotor2api fotor-generation-upstream www.fotor.com")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"fotor", "fotor2api"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestMaskUpstreamProviderInfoClaudeChatUpstream(t *testing.T) {
	got := MaskUpstreamProviderInfo("Claude Chat runbios2api api.runbios.ai RunBIOS RB2API")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"runbios", "rb2api", "api.runbios.ai"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestPublicErrorMessageHidesClaudeChatUpstream(t *testing.T) {
	got := PublicErrorMessage(`Post "http://runbios2api:8911/v1/chat/completions": dial tcp: connection refused`)
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"runbios", "8911", "dial tcp", "http://"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestPublicErrorMessageHidesSeedanceFastUpstream(t *testing.T) {
	got := PublicErrorMessage(`updream generate: https://www.updream.cn/api/ai/generate-video/async smsbower biliapi.net/updream/aniforge sed2-fast`)
	if got != "request failed" {
		t.Fatalf("got %q", got)
	}
}

func TestMaskUpstreamProviderInfoSeedanceFastUpstream(t *testing.T) {
	got := MaskUpstreamProviderInfo("Updream updream2api www.updream.cn smsbower boss.biliapi.net aniforge sed2-fast")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"updream", "smsbower", "biliapi", "aniforge", "sed2-fast"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}

func TestMaskUpstreamProviderInfoMyEdit(t *testing.T) {
	got := MaskUpstreamProviderInfo("MyEdit2API myedit-generation-upstream myedit.online CyberLink cyberlink.com")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"myedit", "cyberlink"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}
