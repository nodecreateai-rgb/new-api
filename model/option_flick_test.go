package model

import (
	"os"
	"strings"
	"testing"
)

func TestFlickModelStaysOnAfterStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureFlick2apiRouting()`,
		`"minimax-h3-max":                     0.4`,
		`"minimax-h3-max":     0.3`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(body, `"minimax-h3-max"}, permanentlyRetiredPublicModels`) {
		t.Fatal("minimax-h3-max must not be wiped from ModelPrice on startup")
	}
	routing, err := os.ReadFile("option_flick.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`flickChannelName = "Flick Video"`,
		`http://flick2api:38710`,
		`FLICK2API_BASE_URL`,
		`933，原生768P，5-12秒`,
		`最多9张图、3段视频、3段音频`,
		`minimax-h3-ref-selfhost`,
		`¥0.4/次，VIP9 ¥0.3/次`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
