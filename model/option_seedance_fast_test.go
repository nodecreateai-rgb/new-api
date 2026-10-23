package model

import (
	"os"
	"strings"
	"testing"
)

func TestSeedanceFastStaysOnAfterStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureSeedanceFastRouting()`,
		`"seedance-2.0-fast-480p":             1.5`,
		`groupPrices["seedance-2.0-fast-480p"] = 1.5`,
		`"vip10":   "VIP10分组"`,
		`"seedance-2.0-fast-480p": 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`"seedance-2.0-fast-480p":             0.8`,
		`groupPrices["seedance-2.0-fast-480p"] = 0.8`,
		`"seedance-2.0-fast-480p":"seedance-2.0-fast-480p"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("old fast-480p setting must not stay: %s", forbidden)
		}
	}

	data, err := os.ReadFile("option_seedance_fast.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureSeedanceFastRouting()`,
		`seedanceFastChannelName = "Seedance 2.0 Fast"`,
		`http://seedance-fast:38685`,
		`SEEDANCE_FAST_BASE_URL`,
		`{"seedance-2.0-fast-480p":"seedance-2.0-fast","seedance-2.0-720p":"seedance-2.0-720p"}`,
		`¥1.5/次，VIP10 ¥1/次，903，最长15秒，不卡人脸，最多9张图、3段音频`,
		`vip10`,
		seedanceFastPublicModel,
		`seedanceFast720Model`,
		`"seedance-2.0-720p":"seedance-2.0-720p"`,
		`¥3/次，VIP10 ¥2.5/次`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(strings.ToLower(s), "updream") {
		t.Fatal("channel config must not name the upstream")
	}
}
