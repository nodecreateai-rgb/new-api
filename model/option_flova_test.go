package model

import (
	"os"
	"strings"
	"testing"
)

func TestFlovaModelsAreRetiredOnStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`retireFlova2apiRouting()`,
		`retirePermanentlyRemovedModels()`,
		`"Flova Video"`,
		`"wan-3.0-prime"`,
		`"minimax-h3"`,
		`"sora-2"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`"wan-3.0-prime":                      2`,
		`"minimax-h3":                         0.5`,
		`"sora-2":                             1.5`,
		`"wan-3.0-prime":      1.5`,
		`if group == "vip9" && model == "wan-3.0-prime"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("retired model price must not be enforced: %s", forbidden)
		}
	}
	if strings.Contains(body, "ensureFlova2apiRouting()") {
		t.Fatal("flova routing must not be re-enabled on startup")
	}

	data, err := os.ReadFile("option_flova.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func retireFlova2apiRouting()`,
		`flovaChannelName = "Flova Video"`,
		`disableChannelByName(flovaChannelName)`,
		`retireMarketplaceModels(flovaPublicModels)`,
		`"wan-3.0-prime"`,
		`"minimax-h3"`,
		`"sora-2"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`func ensureFlova2apiRouting()`,
		`http://flova2api:38686`,
		`FLOVA2API_BASE_URL`,
		`ChannelStatusEnabled`,
	} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("flova ensure leftover %s", forbidden)
		}
	}
}
