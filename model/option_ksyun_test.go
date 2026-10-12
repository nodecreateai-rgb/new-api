package model

import (
	"os"
	"strings"
	"testing"
)

func TestSeedance25AndImageModelsStayRetired(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	startup := body[:strings.Index(body, "func ensureRoboneoMiniRouting")]
	if strings.Contains(startup, "ensureKsyunSeedanceRouting()") {
		t.Fatal("startup must not republish seedance-2.5 after the permanent retire step")
	}
	if strings.Contains(startup, "ensureAir2APIImageRouting()") {
		t.Fatal("startup must not republish image models")
	}
	for _, want := range []string{
		`"seedance-2.5",`,
		`"gpt-image-2",`,
		`"gpt-image-2.5-flare",`,
		`"gpt-image-2.5-sunburst",`,
		`"nano-banana-2",`,
		`"nano-banana-2-lite",`,
		`"nano-banana-2.1",`,
		`"nano-banana-pro",`,
		`"Image Omni"`,
		`"Air Image"`,
		`"Ksyun Seedance 2.5"`,
		`"Seedance 2.5 Video"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`"seedance-2.5":                       7`,
		`targetModelGroupPrices[group]["seedance-2.5"] = 7`,
		`groupPrices["seedance-2.5"] != 7`,
		`air2apiImagePrice`,
		`image-omni=0.05`,
		`seedance-2.5=7`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("retired model is still priced or republished: %s", forbidden)
		}
	}
}
