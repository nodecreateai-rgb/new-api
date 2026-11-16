package perfmetrics

import "testing"

func TestSuccessWindowsBucketMembership(t *testing.T) {
	now := int64(1_700_000_000)
	window := &successWindows{}
	window.add(now, now-2*3600, 10, 9)
	window.add(now, now-5*3600, 10, 5)
	window.add(now, now-20*3600, 10, 1)
	window.add(now, now-successWindow3h, 4, 4)
	window.add(now, now-successWindow3h-1, 100, 0)
	window.add(now, now-successWindow24h, 2, 2)
	window.add(now, now-successWindow24h-1, 50, 50)
	window.add(now, now+10, 7, 7)
	window.add(now, now-100, 0, 0)

	if window.h3.requests != 14 || window.h3.success != 13 {
		t.Fatalf("3h = %+v, want requests 14 success 13", window.h3)
	}
	if window.h12.requests != 124 || window.h12.success != 18 {
		t.Fatalf("12h = %+v, want requests 124 success 18", window.h12)
	}
	if window.h24.requests != 136 || window.h24.success != 21 {
		t.Fatalf("24h = %+v, want requests 136 success 21", window.h24)
	}
}

func TestSuccessWindowSummaryOmitsEmptyWindows(t *testing.T) {
	now := int64(1_700_000_000)
	window := &successWindows{}
	window.add(now, now-20*3600, 4, 3)

	summary := window.summary("gpt-test")
	if summary.ModelName != "gpt-test" {
		t.Fatalf("model = %s", summary.ModelName)
	}
	assertRate(t, summary.SuccessRate24h, 75)
	if summary.SuccessRate12h != nil || summary.SuccessRate3h != nil {
		t.Fatalf("shorter windows = %v %v, want nil", summary.SuccessRate12h, summary.SuccessRate3h)
	}
}

func TestRateFromCountsKeepsWindowsIndependent(t *testing.T) {
	// 3 successes and 5 failures in 3h is 37.5%.
	// Older successes raise the 12h and 24h rates without changing 3h.
	assertRate(t, rateFromCounts(3, 5), 37.5)
	assertRate(t, rateFromCounts(106, 20), 84.13)
	assertRate(t, rateFromCounts(446, 87), 83.68)
	if rateFromCounts(0, 0) != nil {
		t.Fatal("a window with no finished tasks should be empty")
	}
}

func TestRoundedSuccessRate(t *testing.T) {
	if roundedSuccessRate(windowCounts{}) != nil {
		t.Fatal("empty window should be nil")
	}
	assertRate(t, roundedSuccessRate(windowCounts{requests: 3, success: 1}), 33.33)
	assertRate(t, roundedSuccessRate(windowCounts{requests: 10, success: 0}), 0)
	assertRate(t, roundedSuccessRate(windowCounts{requests: 8, success: 8}), 100)
}

func assertRate(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("rate = nil, want %v", want)
	}
	if *got != want {
		t.Fatalf("rate = %v, want %v", *got, want)
	}
}
