package controller

import (
	"os"
	"strings"
	"testing"
)

func TestRelayAuthUsesOAuth2AuthModelName(t *testing.T) {
	if oauth2AuthModelName != "oauth2" {
		t.Fatalf("unexpected model name: %s", oauth2AuthModelName)
	}
}

func TestRelayAuthSourceContainsUpstreamOAuthPath(t *testing.T) {
	data, err := os.ReadFile("auth_relay.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `baseURL + "/v1/oauth"`) {
		t.Fatal("auth relay must proxy to upstream /v1/oauth")
	}
}
