package controller

import (
	"os"
	"strings"
	"testing"
)

func TestRelayPayUsesPayModelName(t *testing.T) {
	if payModelName != "pay" {
		t.Fatalf("unexpected model name: %s", payModelName)
	}
}

func TestRelayPaySourceContainsUpstreamPayPath(t *testing.T) {
	data, err := os.ReadFile("pay_relay.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `baseURL + "/v1/pay"`) {
		t.Fatal("pay relay must proxy to upstream /v1/pay")
	}
	if !strings.Contains(string(data), `"/v1/task/"`) {
		t.Fatal("pay relay must poll upstream /v1/task/{id}")
	}
}
