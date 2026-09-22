package model

import (
	"os"
	"strings"
	"testing"
)

func TestGetunikeyC2RoutingUsesUniKeyGateway(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureGetunikey2apiSeedanceRouting()`,
		`publicModels := []string{"seedance-2.5-c2"}`,
		`baseURL = "http://getunikey2api:38720"`,
		`"seedance-2.5-c2":"bytedance/seedance-2.5"`,
		`"seedance-2.5-c2":                    1`,
		`targetModelGroupPrices[group]["seedance-2.5-c2"] = 1`,
		`"seedance-2.5-c2": ""`,
		`os.Getenv("GETUNIKEY2API_BASE_URL")`,
		`os.Getenv("GETUNIKEY2API_GATEWAY_KEY")`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, "（UniKey") || strings.Contains(s, "UniKey，") {
		t.Fatal("marketplace copy must not mention UniKey")
	}
	if !strings.Contains(s, `ensureGetunikey2apiSeedanceRouting()`) {
		t.Fatal("C2 routing must run at startup")
	}
}
