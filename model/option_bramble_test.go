package model

import (
	"os"
	"strings"
	"testing"
)

func TestBrambleChannelStaysRetired(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	if !strings.Contains(body, `ensureBramble2apiRouting()`) {
		t.Fatal("missing ensureBramble2apiRouting()")
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	for _, want := range []string{
		`"wan3.0-video-prime"`,
		`"seedance-2.5"`,
		`"Bramble Video"`,
	} {
		if !strings.Contains(dead, want) {
			t.Fatalf("missing %s in dead video catalog", want)
		}
	}

	routing, err := os.ReadFile("option_bramble.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`brambleChannelName = "Bramble Video"`,
		`disableChannelByName(brambleChannelName)`,
		`retireMarketplaceModels(bramblePublicModels)`,
		`stripModelsFromChannels(bramblePublicModels)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, `BRAMBLE2API_BASE_URL`) || strings.Contains(s, `¥4/次`) || strings.Contains(s, `ChannelStatusEnabled`) {
		t.Fatal("bramble routing must not publish the models")
	}
}
