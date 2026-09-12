package middleware

import (
	"os"
	"strings"
	"testing"
)

func TestDistributorHandlesPayRoute(t *testing.T) {
	data, err := os.ReadFile("distributor.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `strings.HasPrefix(c.Request.URL.Path, "/v1/pay")`) {
		t.Fatal("distributor must handle /v1/pay")
	}
	if !strings.Contains(s, `modelRequest.Model = "pay"`) {
		t.Fatal("distributor must map /v1/pay to pay model")
	}
}
