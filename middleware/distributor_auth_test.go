package middleware

import (
	"os"
	"strings"
	"testing"
)

func TestDistributorMapsAuthPathToOAuth2AuthModel(t *testing.T) {
	data, err := os.ReadFile("distributor.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `strings.HasPrefix(c.Request.URL.Path, "/v1/auth")`) {
		t.Fatal("distributor must handle /v1/auth")
	}
	if !strings.Contains(s, `modelRequest.Model = "oauth2"`) {
		t.Fatal("distributor must map /v1/auth to oauth2 model")
	}
}
