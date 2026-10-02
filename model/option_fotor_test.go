package model

import (
	"os"
	"strings"
	"testing"
)

func TestFotor2apiRoutingUsesFotorGateway(t *testing.T) {
	data, err := os.ReadFile("option_fotor.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureFotor2apiRouting()`,
		`const neutralName = "Fotor Video"`,
		`baseURL = "http://fotor2api:38684"`,
		`{"seedance-2.0-c2":"seedance-2.0","seedance-2.0-480p-c2":"seedance-2.0-480p","seedance-2.0-fast-c2":"seedance-2.0-fast","seedance-2.0-fast-480p-c2":"seedance-2.0-fast-480p","seedance-2.0-mini-c2":"seedance-mini","wan-3.0-c2":"wan-3.0"}`,
		`os.Getenv("FOTOR2API_BASE_URL")`,
		`os.Getenv("FOTOR2API_GATEWAY_KEY")`,
		`¥0.8/次`,
		`¥0.7/次`,
		`¥0.6/次`,
		`¥1.5/次`,
		`不超分`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}

	ratio, err := os.ReadFile("../setting/ratio_setting/model_ratio.go")
	if err != nil {
		t.Fatal(err)
	}
	ratioBody := string(ratio)
	for _, want := range []string{
		`"seedance-2.0-c2":                    0.8`,
		`"seedance-2.0-480p-c2":               0.8`,
		`"seedance-2.0-fast-c2":               0.7`,
		`"seedance-2.0-fast-480p-c2":          0.7`,
		`if price, ok := defaultModelPrice[name]; ok`,
	} {
		if !strings.Contains(ratioBody, want) {
			t.Fatalf("missing default price %s", want)
		}
	}

	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureFotor2apiRouting()`,
		`"seedance-2.0-c2":                    0.8`,
		`"seedance-2.0-480p-c2":               0.8`,
		`"seedance-2.0-fast-c2":               0.7`,
		`"seedance-2.0-fast-480p-c2":          0.7`,
		`"seedance-2.0-mini-c2":               0.6`,
		`"wan-3.0-c2":                         1.5`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
