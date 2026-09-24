package model

import (
	"os"
	"strings"
	"testing"
)

func TestLaihuaSeedanceRoutingUsesLaihuaGateway(t *testing.T) {
	data, err := os.ReadFile("option_laihua.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureLaihua2apiSeedanceRouting()`,
		`publicModels := []string{"seedance-2.5-480p", "seedance-2.5-720p", "seedance-2.5-1080p"}`,
		`baseURL = "http://laihua2api:38693"`,
		`"seedance-2.5-480p":"seedance-2.5-480p","seedance-2.5-720p":"seedance-2.5-720p","seedance-2.5-1080p":"seedance-2.5-1080p"`,
		`os.Getenv("LAIHUA2API_BASE_URL")`,
		`os.Getenv("LAIHUA2API_GATEWAY_KEY")`,
		`¥3/次`,
		`¥4/次`,
		`¥5/次`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}

	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureLaihua2apiSeedanceRouting()`,
		`"seedance-2.5-480p":                  3`,
		`"seedance-2.5-720p":                  4`,
		`"seedance-2.5-1080p":                 5`,
		`targetModelGroupPrices[group]["seedance-2.5-480p"] = 3`,
		`targetModelGroupPrices[group]["seedance-2.5-720p"] = 4`,
		`targetModelGroupPrices[group]["seedance-2.5-1080p"] = 5`,
		`"seedance-2.5-480p":  2`,
		`"seedance-2.5-720p":  3`,
		`"seedance-2.5-1080p": 4`,
		`if group != "vip9"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
