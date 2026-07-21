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

func TestMaskUpstreamProviderInfoMyEdit(t *testing.T) {
	got := MaskUpstreamProviderInfo("MyEdit2API myedit-generation-upstream myedit.online CyberLink cyberlink.com")
	lower := strings.ToLower(got)
	for _, forbidden := range []string{"myedit", "cyberlink"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("leaked %q in %q", forbidden, got)
		}
	}
}
