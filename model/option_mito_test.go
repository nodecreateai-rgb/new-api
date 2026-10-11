package model

import (
	"os"
	"strings"
	"testing"
)

func TestMitoRoutingUsesMitoGateway(t *testing.T) {
	data, err := os.ReadFile("option_mito.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureMito2apiRouting()`,
		`publicModels := []string{"seedance-2.5-480p"}`,
		`baseURL = "http://mito2api:38474"`,
		`"seedance-2.5-480p":"seedance-2-5"`,
		`os.Getenv("MITO2API_BASE_URL")`,
		`os.Getenv("MITO2API_GATEWAY_KEY")`,
		`原生人脸，30秒，30图0视10音频，480p，有水印，1.5元一次`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, "seedance-2.0-720p") {
		t.Fatal("seedance-2.0-720p must not be published by mito")
	}

	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureMito2apiRouting()`,
		`retirePermanentlyRemovedModels()`,
		`"seedance-2.0-720p"`,
		`"seedance-2.5-480p":                  1.5`,
		`targetModelGroupPrices[group]["seedance-2.5-480p"] = 1.5`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(body, `"seedance-2.0-fast-720p", "seedance-2.0-720p", "kling-o3"`) {
		t.Fatal("seedance-2.0-720p must leave the adobe classic channel")
	}
	if strings.Contains(body, `"seedance-2.0-720p":                  1`) || strings.Contains(body, `"seedance-2.0-720p":      1`) {
		t.Fatal("seedance-2.0-720p price must not be enforced")
	}
}
