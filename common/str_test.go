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

func TestPublicErrorMessageHidesFlova(t *testing.T) {
	got := PublicErrorMessage("flova finished without a video")
	if got != "request failed" {
		t.Fatalf("got %q", got)
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

func TestMaskUpstreamProviderInfoMyEdit(t *testing.T) {
	got := MaskUpstreamProviderInfo("MyEdit2API myedit-generation-upstream myedit.online CyberLink cyberlink.com")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"myedit", "cyberlink"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}
