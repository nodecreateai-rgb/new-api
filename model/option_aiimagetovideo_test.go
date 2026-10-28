package model

import (
	"os"
	"strings"
	"testing"
)

func TestAiimagetovideoModelIsRetired(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	if !strings.Contains(body, "var permanentlyRetiredPublicModels = []string{\n\t\"wan-3.0-480p\",") {
		t.Fatal("wan-3.0-480p must stay retired")
	}
	routing, err := os.ReadFile("option_aiimagetovideo.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{`disableChannelByName(aivideoChannelName)`, `retireMarketplaceModels(aivideoPublicModels)`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
