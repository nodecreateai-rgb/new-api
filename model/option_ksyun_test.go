package model

import (
	"os"
	"strings"
	"testing"
)

func TestKsyunSeedanceStaysOnAtSevenYuan(t *testing.T) {
	data, err := os.ReadFile("option_ksyun.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureKsyunSeedanceRouting()`,
		`const publicModel = "seedance-2.5"`,
		`neutralName = "Ksyun Seedance 2.5"`,
		`baseURL = "http://ksyun2api:38922"`,
		`{"seedance-2.5":"seedance-2.5"}`,
		`os.Getenv("KSYUN2API_BASE_URL")`,
		`os.Getenv("KSYUN2API_GATEWAY_KEY")`,
		`¥7/次，最长30秒`,
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
	retire := body[strings.Index(body, "func retirePermanentlyRemovedModels"):]
	ksyunCall := strings.Index(body, "ensureKsyunSeedanceRouting()")
	retireCall := strings.Index(body, "retirePermanentlyRemovedModels()")
	if ksyunCall < 0 || retireCall < 0 || ksyunCall < retireCall {
		t.Fatal("ksyun seedance-2.5 must be restored after the permanent retire step")
	}
	for _, want := range []string{
		`ensureKsyunSeedanceRouting()`,
		`"seedance-2.5":                       7`,
		`targetModelGroupPrices[group]["seedance-2.5"] = 7`,
		`groupPrices["seedance-2.5"] != 7`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(retire, `"seedance-2.5",`) || strings.Contains(body, `"seedance-2.5":                       3`) || strings.Contains(body, `"seedance-2.5":       2`) {
		t.Fatal("seedance-2.5 must not be retired or repriced away from ¥7")
	}
}
