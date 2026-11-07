package model

import (
	"os"
	"strings"
	"testing"
)

func TestXxzChannelStaysDisabledAfterC2Move(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureXxzRouting()`,
		`"seedance-2.0-fast-720p-c2":          1.8,`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	ensureXxz := strings.Index(body, "ensureXxzRouting()")
	ensureJm2 := strings.Index(body, "ensureJm2apiRouting()")
	retireAt := strings.Index(body, "retireDeadVideoModels()")
	if ensureXxz < 0 || ensureJm2 < 0 || retireAt < 0 || ensureXxz < retireAt || ensureJm2 < ensureXxz {
		t.Fatal("ensureXxzRouting must disable C2 after retireDeadVideoModels, then ensureJm2apiRouting republishes c2")
	}
	if strings.Contains(body, `"seedance-2.0-fast-720p-c2"`) && strings.Contains(body, "permanentlyRetiredPublicModels") {
		retired := body[strings.Index(body, "var permanentlyRetiredPublicModels"):]
		retired = retired[:strings.Index(retired, "}")]
		if strings.Contains(retired, `"seedance-2.0-fast-720p-c2"`) {
			t.Fatal("seedance-2.0-fast-720p-c2 must not be permanently retired")
		}
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	if strings.Contains(dead, `"seedance-2.0-fast-720p-c2"`) {
		t.Fatal("seedance-2.0-fast-720p-c2 must not be in deadVideoModels")
	}
	if !strings.Contains(dead, `"Video Fast 720p C2"`) {
		t.Fatal("Video Fast 720p C2 must stay in deadVideoChannels")
	}

	routing, err := os.ReadFile("option_xxz.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`xxzChannelName = "Video Fast 720p C2"`,
		`disableChannelByName(xxzChannelName)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`521ai-SD-fast-720-xd`,
		`XXZ_BASE_URL`,
		`ChannelStatusEnabled`,
		`stripModelsFromChannels`,
		`retireMarketplaceModels`,
	} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("xxz seeder must not republish c2: %s", forbidden)
		}
	}
}
