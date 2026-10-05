package model

import (
	"os"
	"strings"
	"testing"
)

func TestVideoStudioModelsAreRetiredOnStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`retireVideoStudioRouting()`,
		`"Video Studio"`,
		`"seedance-2.0"`,
		`"sora-2"`,
		`"wan-3.0"`,
		`"minimax-h3"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(body, "ensureVideoStudioRouting()") {
		t.Fatal("video studio routing must not be re-enabled on startup")
	}

	data, err := os.ReadFile("option_videostudio.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func retireVideoStudioRouting()`,
		`videoStudioChannelName = "Video Studio"`,
		`disableChannelByName(videoStudioChannelName)`,
		`retireMarketplaceModels(videoStudioPublicModels)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`func ensureVideoStudioRouting()`,
		`http://video-studio-upstream:38691`,
		`VIDEO_STUDIO_BASE_URL`,
		`ChannelStatusEnabled`,
	} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("video studio ensure leftover %s", forbidden)
		}
	}
}
