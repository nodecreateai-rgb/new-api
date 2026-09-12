package model

import (
	"os"
	"strings"
	"testing"
)

func TestPay2APIRoutingUsesNeutralPersistentAlias(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`const publicModel = "pay"`,
		`baseURL = "http://pay2api:8080"`,
		`{"openai":{"path":"/v1/pay","method":"POST"}}`,
		`Stripe, GPay, GoPay protocol payment`,
		`ensurePay2APIRoutingClickHouse`,
		`ensureNamedVendorID("Pay2API"`,
		`InvalidatePricingCache()`,
		`Where("model = ? AND channel_id <> ?", publicModel, channel.Id)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}

func TestPayModelPriceIsZeroPointFivePerCall(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{`"pay":                           0.5`, `pay=0.5`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
