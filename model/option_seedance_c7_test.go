package model

import (
	"os"
	"strings"
	"testing"
)

func TestSeedanceC7ModelsStayRetired(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	if !strings.Contains(body, `ensureSeedanceC7Routing()`) {
		t.Fatal("missing ensureSeedanceC7Routing()")
	}
	for _, forbidden := range []string{
		`failed to enforce Video Seedance C7 gateway routing`,
		`"seedance-2.0-mini-480p-c7":`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("startup must not treat C7 as a published channel or enforce price: %s", forbidden)
		}
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	for _, wantDead := range []string{
		`"seedance-2.0-mini-480p-c7"`,
		`"seedance-2.0-480p-c7"`,
		`"seedance-2.0-fast-480p-c7"`,
		`"seedance-2.0-mini-720p-c7"`,
		`"Video Seedance C7"`,
	} {
		if !strings.Contains(dead, wantDead) {
			t.Fatalf("missing %s in dead video catalog", wantDead)
		}
	}

	routing, err := os.ReadFile("option_seedance_c7.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`seedanceC7ChannelName = "Video Seedance C7"`,
		`retireNamedChannelAndModels(seedanceC7ChannelName, seedanceC7PublicModels)`,
		`"seedance-2.0-mini-480p-c7"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`ChannelStatusEnabled`,
		`¥0.5/次`,
		`http://seele2api:38510`,
		`status = 1`,
	} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("c7 routing must not publish the models: %s", forbidden)
		}
	}
}
