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
		`¥0.8/次`,
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
		`"seedance-2.0-mini-720p":             0.8`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
