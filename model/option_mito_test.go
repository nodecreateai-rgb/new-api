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
		`ensureSeedance480Routing()`,
		`"Mito Video"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	mito := strings.Index(body, `retireMito2apiRouting()`)
	publish := strings.Index(body, `ensureSeedance480Routing()`)
	if mito < 0 || publish < 0 || publish < mito {
		t.Fatal("480p catalog must be published again after the Mito channel is retired")
	}
	for _, forbidden := range []string{
		`ensureMito2apiRouting()`,
		`"seedance-2.5-480p":                  1.5`,
		`targetModelGroupPrices[group]["seedance-2.5-480p"] = 1.5`,
		`"seedance-2.5-480p":  1.5`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("old mito 480p price must not return: %s", forbidden)
		}
	}
}
