package model

import (
	"os"
	"strings"
	"testing"
)

func TestOAuth2APIRoutingUsesNeutralPersistentAlias(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`const publicModel = "oauth2"`,
		`baseURL = "http://google2api:39181"`,
		`{"openai":{"path":"/v1/auth","method":"POST"}}`,
		`ensureOAuth2APIRoutingClickHouse`,
		`ensureNamedVendorID("OAuth2API"`,
		`description := "谷歌协议登录"`,
		`InvalidatePricingCache()`,
		`Where("model = ? AND channel_id <> ?", publicModel, channel.Id)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}

func TestOAuth2AuthPriceIsZeroPointTwoPerCall(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{`"oauth2":                        0.2`, `oauth2=0.2`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
