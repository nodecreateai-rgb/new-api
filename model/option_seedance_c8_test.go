package model

import (
	"os"
	"strings"
	"testing"
)

func TestSeedanceC8ModelsStayRetired(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	if !strings.Contains(body, `ensureSeedanceC8Routing()`) {
		t.Fatal("missing ensureSeedanceC8Routing()")
	}
	if strings.Contains(body, `failed to enforce Video Seedance C8 gateway routing`) {
		t.Fatal("startup must not treat C8 as a published channel")
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	for _, want := range []string{
		`"seedance-2.0-c8"`,
		`"seedance-2.0-mini-c8"`,
		`"Video Seedance C8"`,
	} {
		if !strings.Contains(dead, want) {
			t.Fatalf("missing %s in dead video catalog", want)
		}
	}

	routing, err := os.ReadFile("option_seedance_c8.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`seedanceC8ChannelName = "Video Seedance C8"`,
		`disableChannelByName(seedanceC8ChannelName)`,
		`retireMarketplaceModels(seedanceC8PublicModels)`,
		`stripModelsFromChannels(seedanceC8PublicModels)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`ChannelStatusEnabled`,
		`¥0.3/次`,
		`¥0.2/次`,
		`GUESTDOUBAO_BASE_URL`,
		`status = 1`,
	} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("c8 routing must not publish the models: %s", forbidden)
		}
	}
}
