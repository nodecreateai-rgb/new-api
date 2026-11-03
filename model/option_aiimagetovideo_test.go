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
	if strings.Contains(body, "var permanentlyRetiredPublicModels = []string{\n\t\"wan-3.0-480p\",") {
		t.Fatal("wan-3.0-480p is on sale again")
	}
	if !strings.Contains(body, `"wan-3.0-480p":                       1.5,`) {
		t.Fatal("missing wan-3.0-480p price")
	}
	routing, err := os.ReadFile("option_aiimagetovideo.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	if !strings.Contains(s, `disableChannelByName(aivideoChannelName)`) {
		t.Fatal("old channel must stay disabled")
	}
	if strings.Contains(s, `retireMarketplaceModels(aivideoPublicModels)`) {
		t.Fatal("startup must not withdraw wan-3.0-480p")
	}
}
