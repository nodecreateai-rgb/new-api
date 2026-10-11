package model

import (
	"os"
	"strings"
	"testing"
)

func TestFotorModelsAreRetiredOnStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`retireFotor2apiRouting()`,
		`retirePermanentlyRemovedModels()`,
		`"Fotor Video"`,
		`"wan-3.0-480p-c2"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`ensureFotor2apiRouting()`,
		`"wan-3.0-480p-c2":                    0.5`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("fotor must not be republished: %s", forbidden)
		}
	}

	data, err := os.ReadFile("option_fotor.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func retireFotor2apiRouting()`,
		`fotorChannelName = "Fotor Video"`,
		`disableChannelByName(fotorChannelName)`,
		`retireMarketplaceModels(fotorPublicModels)`,
		`"wan-3.0-480p-c2"`,
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

	ratio, err := os.ReadFile("../setting/ratio_setting/model_ratio.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ratio), `"wan-3.0-480p-c2"`) {
		t.Fatal("default price must not keep wan-3.0-480p-c2")
	}
}
