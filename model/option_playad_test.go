package model

import (
	"os"
	"strings"
	"testing"
)

func TestPlayadModelStaysOnAfterStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensurePlayad2apiRouting()`,
		`"seedance-2.0-fast-720p-c1":          1.8`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(body, `"seedance-2.0-fast-720p-c1"}, permanentlyRetiredPublicModels`) {
		t.Fatal("seedance-2.0-fast-720p-c1 must not be wiped from ModelPrice on startup")
	}
	routing, err := os.ReadFile("option_playad.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`playadChannelName = "Playad Video"`,
		`http://playad2api:38690`,
		`PLAYAD2API_BASE_URL`,
		`{"seedance-2.0-fast-720p-c1":"seedance-2.0-fast"}`,
		`¥1.8/次，原生720P，4-15秒，默认带声音且不可关闭`,
		`画幅 16:9/9:16/1:1/4:3/3:4/21:9`,
		`最多9张图片、3段视频、3段音频`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
