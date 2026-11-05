package model

import (
	"os"
	"strings"
	"testing"
)

func TestOpenaifitChannelStaysRetired(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	if !strings.Contains(body, `ensureOpenaifitRouting()`) {
		t.Fatal("missing ensureOpenaifitRouting()")
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	for _, want := range []string{
		`"seedance-2.0-fast-720p-c1"`,
		`"Video Fast 720p C1"`,
	} {
		if !strings.Contains(dead, want) {
			t.Fatalf("missing %s in dead video catalog", want)
		}
	}

	routing, err := os.ReadFile("option_openaifit.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`openaifitChannelName = "Video Fast 720p C1"`,
		`disableChannelByName(openaifitChannelName)`,
		`retireMarketplaceModels(openaifitPublicModels)`,
		`stripModelsFromChannels(openaifitPublicModels)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, `OPENAIFIT_BASE_URL`) || strings.Contains(s, `¥1.8/次`) || strings.Contains(s, `ChannelStatusEnabled`) {
		t.Fatal("fast 720p c1 routing must not publish the model")
	}
}
