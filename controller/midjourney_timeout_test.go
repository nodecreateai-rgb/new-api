package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestMidjourneyTaskTimedOutUsesConfiguredMinutes(t *testing.T) {
	previous := constant.TaskTimeoutMinutes
	t.Cleanup(func() { constant.TaskTimeoutMinutes = previous })

	constant.TaskTimeoutMinutes = 120
	if midjourneyTaskTimedOut(60*60*1000, "50%") {
		t.Fatal("one-hour task must not time out when configured for 120 minutes")
	}
	if midjourneyTaskTimedOut(119*60*1000, "50%") {
		t.Fatal("119-minute task must not time out when configured for 120 minutes")
	}
	if !midjourneyTaskTimedOut(121*60*1000, "50%") {
		t.Fatal("121-minute task must time out when configured for 120 minutes")
	}
	if midjourneyTaskTimedOut(121*60*1000, "100%") {
		t.Fatal("completed task must not be overwritten by timeout")
	}
}

func TestMidjourneyTaskTimeoutCanBeDisabled(t *testing.T) {
	previous := constant.TaskTimeoutMinutes
	t.Cleanup(func() { constant.TaskTimeoutMinutes = previous })

	constant.TaskTimeoutMinutes = 0
	if midjourneyTaskTimedOut(24*60*60*1000, "50%") {
		t.Fatal("zero timeout must disable the timeout rule")
	}
}
