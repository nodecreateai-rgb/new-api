package model

import (
	"os"
	"strings"
	"testing"
)

func TestSeedance480StaysOnAfterStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureSeedance480Routing()`,
		`ensureVIP11OnEnabledChannels()`,
		`"seedance-2.0-480p":                  1`,
		`"seedance-2.5-480p":                  1.5`,
		`"vip11":   "VIP11分组"`,
		`"seedance-2.0-480p": 1`,
		`"seedance-2.5-480p": 1`,
		`groupPrices["seedance-2.0-480p"] = 1`,
		`groupPrices["seedance-2.5-480p"] = 1.5`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(strings.ToLower(body), "manyverse") {
		t.Fatal("option.go must not mention the upstream project name")
	}
	publish := strings.Index(body, `ensureSeedance480Routing()`)
	vip11 := strings.Index(body, `ensureVIP11OnEnabledChannels()`)
	if publish < 0 || vip11 < publish {
		t.Fatal("vip11 abilities must be applied after the 480p channel is published")
	}

	data, err := os.ReadFile("option_manyverse.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureSeedance480Routing()`,
		`seedance480ChannelName = "Seedance 480p Video"`,
		`http://manyverse2api:38748`,
		`SEEDANCE480_BASE_URL`,
		`{"seedance-2.0-480p":"seedance-2.0","seedance-2.5-480p":"seedance-2.5","wan-3.0-prime-720p":"wan-3.0-prime"}`,
		`¥1/次，原生480P，最长15秒，全参，卡脸`,
		`¥1.5/次，VIP11 ¥1/次，原生480P，最长30秒，全参，卡脸`,
		`channel_id NOT IN`,
		`model NOT IN`,
		`vip11`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
