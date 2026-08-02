package model

import (
	"os"
	"strings"
	"testing"
)

func TestSD2FastRoutingHasDeploymentOverrideAndWorkerReachableFallback(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `os.Getenv("SD2_FAST_BASE_URL")`) {
		t.Fatal("sd2-fast routing must support a deployment override")
	}
	if !strings.Contains(s, `baseURL = "http://187.124.94.7:38474"`) {
		t.Fatal("sd2-fast routing must fall back to the worker-verified published route")
	}
}
