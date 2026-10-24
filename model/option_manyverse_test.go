package model

import (
	"os"
	"strings"
	"testing"
)

func TestSeedance480ChannelStaysRetired(t *testing.T) {
	data, err := os.ReadFile("option_manyverse.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureSeedance480Routing()`,
		`disableChannelByName(seedance480ChannelName)`,
		`retireMarketplaceModels(seedance480PublicModels)`,
		`"seedance-2.0-480p", "seedance-2.5-480p", "wan-3.0-prime-720p"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{`ChannelStatusEnabled`, `INSERT INTO`, `FirstOrCreate`, `"status": 1`} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("480p channel must not be re-published: %s", forbidden)
		}
	}
}
