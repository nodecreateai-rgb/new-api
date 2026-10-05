package model

import (
	"os"
	"strings"
	"testing"
)

func TestFotor2apiModelsAreRetiredOnStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`retireFotor2apiRouting()`,
		`"Fotor Video"`,
		`"seedance-2.0-c2"`,
		`"seedance-2.0-480p-c2"`,
		`"seedance-2.0-fast-c2"`,
		`"seedance-2.0-fast-480p-c2"`,
		`"seedance-2.0-mini-c2"`,
		`"wan-3.0-c2"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(body, "ensureFotor2apiRouting()") {
		t.Fatal("fotor routing must not be re-enabled on startup")
	}

	data, err := os.ReadFile("option_fotor.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func retireFotor2apiRouting()`,
		`fotor2apiChannelName = "Fotor Video"`,
		`disableChannelByName(fotor2apiChannelName)`,
		`retireMarketplaceModels(fotor2apiPublicModels)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`func ensureFotor2apiRouting()`,
		`http://fotor2api:38684`,
		`FOTOR2API_BASE_URL`,
		`ChannelStatusEnabled`,
	} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("fotor ensure leftover %s", forbidden)
		}
	}
}
