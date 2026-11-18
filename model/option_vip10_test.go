package model

import (
	"os"
	"strings"
	"testing"
)

func TestVip10PriceForSkipsAlreadyDiscountedFast480(t *testing.T) {
	if got := vip10PriceFor("seedance-2.0-fast-480p", 1.5); got != 1.4 {
		t.Fatalf("fast-480p already discounted: got %v", got)
	}
	if got := vip10PriceFor("seedance-2.0-fast-480p", 1.4); got != 1.4 {
		t.Fatalf("fast-480p must not drop again from 1.4: got %v", got)
	}
}

func TestVip10PriceForSubtractsOneTenthFromSticker(t *testing.T) {
	cases := map[string][2]float64{
		"seedance-2.5":               {4, 3.9},
		"seedance-2.0-480p":          {1.6, 1.5},
		"seedance-2.0-c8":            {0.3, 0.2},
		"seedance-2.0-mini-c8":       {0.2, 0.1},
		"sd2.5":                      {0.5, 0.4},
		"sd2-c7":                     {0.5, 0.4},
		"seedance-2.0-fast-720p-c5":  {1.5, 1.4},
		"seedance-2.0-720p-c2":       {2.8, 2.7},
		"seedance-2.0-720p-c3":       {1.8, 1.7},
		"seedance-2.5-480p-c5":       {4, 3.9},
		"seedance-2.5-flash-480p-c5": {3, 2.9},
		"seedance-2.5-flash-720p-c5": {4, 3.9},
		"seedance-2.0-720p":          {3, 2.9},
		"oauth2":                     {0.3, 0.2},
	}
	for model, pair := range cases {
		if got := vip10PriceFor(model, pair[0]); got != pair[1] {
			t.Fatalf("%s: sticker %v → vip10 %v, got %v", model, pair[0], pair[1], got)
		}
	}
}

func TestVip10PriceFor720pIsStickerMinusOneTenth(t *testing.T) {
	if got := vip10PriceFor("seedance-2.0-720p", 3); got != 2.9 {
		t.Fatalf("720p sticker 3 → vip10 2.9, got %v", got)
	}
}

func TestApplyVip10DiscountIsRestartSafe(t *testing.T) {
	sticker := map[string]float64{
		"seedance-2.5":              4,
		"seedance-2.0-fast-480p":    1.5,
		"seedance-2.0-fast-480p-c1": 1.5,
		"seedance-2.0-fast-720p-c5": 1.5,
		"seedance-2.0-720p":         3,
		"sd2.5":                     0.5,
		"gemini-3.5-flash":          0.015,
	}
	groupPrices := map[string]map[string]float64{
		"default": cloneFloatMap(sticker),
		"vip10":   cloneFloatMap(sticker),
	}
	groupPrices["vip10"]["seedance-2.0-fast-480p"] = 1.4

	simulateStartup := func() {
		// Startup recopies sticker prices into every group, then reapplies
		// the vip10 specials, matching ensureDopioRMBPricing.
		groupPrices["vip10"] = cloneFloatMap(sticker)
		groupPrices["vip10"]["seedance-2.0-fast-480p"] = 1.4
		if !applyVip10ModelPriceDiscount(groupPrices, sticker) {
			t.Fatal("expected vip10 prices to change from sticker")
		}
	}

	simulateStartup()
	assertVip10 := func() {
		vip10 := groupPrices["vip10"]
		want := map[string]float64{
			"seedance-2.5":              3.9,
			"seedance-2.0-fast-480p":    1.4,
			"seedance-2.0-fast-480p-c1": 1.4,
			"seedance-2.0-fast-720p-c5": 1.4,
			"seedance-2.0-720p":         2.9,
			"sd2.5":                     0.4,
		}
		for model, price := range want {
			if vip10[model] != price {
				t.Fatalf("%s: want %v, got %v", model, price, vip10[model])
			}
		}
		if _, ok := vip10["gemini-3.5-flash"]; ok {
			t.Fatal("token-billed chat models must not get a vip10 per-call price")
		}
		if groupPrices["default"]["seedance-2.5"] != 4 {
			t.Fatal("default group must keep sticker prices")
		}
	}
	assertVip10()

	simulateStartup()
	assertVip10()

	if applyVip10ModelPriceDiscount(groupPrices, sticker) {
		t.Fatal("second pass without a sticker reset must be a no-op")
	}
	assertVip10()
}

func TestVip10DiscountWiredIntoStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`applyVip10ModelPriceDiscount(modelGroupPrices, prices)`,
		`"seedance-2.0-fast-480p": 1.4`,
		`"seedance-2.0-720p":      2.9`,
		`vip10 sticker-0.1 except seedance-2.0-fast-480p=1.4`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
}

func cloneFloatMap(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
