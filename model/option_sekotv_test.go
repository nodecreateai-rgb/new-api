package model

import (
	"os"
	"strings"
	"testing"
)

func TestSekotvMiniRoutingUsesSekotvGateway(t *testing.T) {
	data, err := os.ReadFile("option_sekotv.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureSekotvMiniRouting()`,
		`const publicModel = "seedance-2.0-mini-720p"`,
		`const upstreamModel = "seedance-2.0-mini"`,
		`baseURL = "http://sekotv2api:38683"`,
		`{"seedance-2.0-mini-720p":"seedance-2.0-mini"}`,
		`os.Getenv("SEKOTV2API_BASE_URL")`,
		`os.Getenv("SEKOTV2API_GATEWAY_KEY")`,
		`¥1/次`,
		`933，原生720P，最长15秒`,
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
		`ensureSekotvMiniRouting()`,
		`"seedance-2.0-mini-720p":             1,`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
}

func TestRoboneoMiniMarketplaceDescriptions(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureRoboneoMiniRouting()`,
		`retirePermanentlyRemovedModels()`,
		`"seedance-2.0-mini-480p":             0.4`,
		`"seedance-2.0-mini-480p": "Seedance 2.0 Mini 480p 文生/图生视频（异步，¥0.4/次，903，超分720P，最长10秒）"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`"seedance-2.0-mini":                  0.5`,
		`"seedance-2.0-mini":      "Seedance 2.0 Mini 文生/图生视频（异步，¥0.5/次，903，最长12秒，原生720P）"`,
		`¥0.8/次，903，超分720P，最长15秒`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("retired mini listing must stay down: %s", forbidden)
		}
	}
}
