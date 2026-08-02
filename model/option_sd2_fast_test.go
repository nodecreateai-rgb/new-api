package model

import (
	"os"
	"strings"
	"testing"
)

func TestSD2FastRoutingUsesRestartSafeInternalAlias(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `baseURL = "http://video-fast-upstream:39763"`) {
		t.Fatal("sd2-fast routing must default to the neutral overlay alias and actual internal listen port")
	}
	if !strings.Contains(s, `os.Getenv("SD2_FAST_BASE_URL")`) {
		t.Fatal("sd2-fast routing must support a deployment override")
	}
	if strings.Contains(s, `187.124.94.7:38474`) {
		t.Fatal("sd2-fast routing must not depend on host-published manager port")
	}
}
