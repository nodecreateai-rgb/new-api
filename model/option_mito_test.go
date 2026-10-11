package model

import (
	"os"
	"strings"
	"testing"
)

func TestMito480pIsRetiredOnStartup(t *testing.T) {
	data, err := os.ReadFile("option_mito.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func retireMito2apiRouting()`,
		`mitoChannelName = "Mito Video"`,
		`disableChannelByName(mitoChannelName)`,
		`retireMarketplaceModels([]string{"seedance-2.5-480p"})`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`func ensureMito2apiRouting()`,
		`http://mito2api:38474`,
		`ChannelStatusEnabled`,
	} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("mito ensure leftover %s", forbidden)
		}
	}

	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`retireMito2apiRouting()`,
		`"seedance-2.5-480p"`,
		`"Mito Video"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`ensureMito2apiRouting()`,
		`"seedance-2.5-480p":                  1.5`,
		`targetModelGroupPrices[group]["seedance-2.5-480p"] = 1.5`,
		`"seedance-2.5-480p":  1.5`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("seedance-2.5-480p must stay retired: %s", forbidden)
		}
	}
}
