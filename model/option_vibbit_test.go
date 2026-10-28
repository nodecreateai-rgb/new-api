package model

import (
	"os"
	"strings"
	"testing"
)

func TestVibbitModelStaysOnAfterStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureVibbit2apiRouting()`,
		`ensureAiimagetovideo2apiRouting()`,
		`"wan-3.0-480p":                       1.5`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(body, `"wan-3.0-480p"}, permanentlyRetiredPublicModels`) {
		t.Fatal("wan-3.0-480p must not be wiped from ModelPrice on startup")
	}
	routing, err := os.ReadFile("option_vibbit.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`vibbitChannelName = "Vibbit Video"`,
		`disableChannelByName(vibbitChannelName)`,
		`Spark Video`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, `{"wan-3.0-480p":"wan3"}`) {
		t.Fatal("Vibbit Video must not keep wan-3.0-480p mapping")
	}
}
