package common

import "testing"

func TestResolveTaskSubmitDurationPrefersLargerSeconds(t *testing.T) {
	if got := ResolveTaskSubmitDuration(10, "30"); got != 30 {
		t.Fatalf("got=%d want=30", got)
	}
	if got := ResolveTaskSubmitDuration(30, "10"); got != 30 {
		t.Fatalf("got=%d want=30", got)
	}
	if got := ResolveTaskSubmitDuration(0, "30"); got != 30 {
		t.Fatalf("got=%d want=30", got)
	}
	if got := ResolveTaskSubmitDuration(10, ""); got != 10 {
		t.Fatalf("got=%d want=10", got)
	}
}
