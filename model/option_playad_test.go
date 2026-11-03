package model

import (
	"os"
	"strings"
	"testing"
)

func TestPlayadFast720Retired(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	if !strings.Contains(body, `ensurePlayad2apiRouting()`) {
		t.Fatal("missing ensurePlayad2apiRouting()")
	}
	if !strings.Contains(body, `"seedance-2.0-fast-720p-c1",`) {
		t.Fatal("seedance-2.0-fast-720p-c1 must stay in permanentlyRetiredPublicModels")
	}
	if strings.Contains(body, `"seedance-2.0-fast-720p-c1":          1.8`) {
		t.Fatal("retired model must not be written back into ModelPrice")
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
}
