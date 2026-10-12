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

func TestSD25RoutingUsesNeutralPersistentAlias(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{`const publicModel = "sd2.5"`, `const upstreamModel = "seedance2.5-c1"`, `baseURL = "http://dola2api:38472"`, `Where("model = ? AND channel_id <> ?", publicModel, channel.Id)`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}

func TestSD25AndSD2C7PriceIsHalfYuanPerCall(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`"sd2.5":                              0.5`,
		`"sd2-c7":                             0.5`,
		`targetModelGroupPrices[group]["sd2.5"] = 0.5`,
		`targetModelGroupPrices[group]["sd2-c7"] = 0.5`,
		`sd2.5=0.5 per call`,
		`sd2-c7=0.5 per call`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{`"sd2.5":                              1.5`, `"sd2-c7": 0.3`, `"sd2.5":  0.3`, `"sd2.5":                  1`} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("old price still enforced: %s", forbidden)
		}
	}
}

func TestDolaRoutingUsesNeutralPersistentAlias(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`os.Getenv("DOLA2API_BASE_URL")`,
		`baseURL = "http://dola2api:38472"`,
		`const neutralName = "Dola2API Seedance Video"`,
		`"sd2-c7":"seedance2-c1"`,
		`"sd2-c6":"seedance2-c2"`,
		`"sd2-c6": "最大支持10秒，9图，fast模型"`,
		`"sd2-c7": "最大支持15秒，10图，fast模型"`,
		`const description = "最大支持30秒，10图"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
