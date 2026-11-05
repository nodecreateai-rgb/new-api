package model

import (
	"os"
	"strings"
	"testing"
)

func TestPlayadChannelStaysRetired(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	if !strings.Contains(body, `ensurePlayad2apiRouting()`) {
		t.Fatal("missing ensurePlayad2apiRouting()")
	}
	routing, err := os.ReadFile("option_playad.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`playadChannelName = "Playad Video"`,
		`disableChannelByName(playadChannelName)`,
		`retireMarketplaceModels(playadPublicModels)`,
		`stripModelsFromChannels(playadPublicModels)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, `PLAYAD2API_BASE_URL`) || strings.Contains(s, `¥1.8/次`) {
		t.Fatal("playad routing must not publish the model")
	}
	if strings.Contains(s, `"seedance-2.0-fast-720p-c1"`) {
		t.Fatal("playad seeder must not keep stripping seedance-2.0-fast-720p-c1")
	}
}
