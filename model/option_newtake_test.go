package model

import (
	"os"
	"strings"
	"testing"
)

func TestNewtakeModelStaysOnAfterStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureNewtake2apiRouting()`,
		`ensureVIP13OnEnabledChannels()`,
		`"wan-3.0-c1":                         0.8`,
		`"wan-3.0-c1": 0.5`,
		`"vip13":   1`,
		`"vip13":   "VIP13分组"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(body, `"wan-3.0-c1"}, permanentlyRetiredPublicModels`) {
		t.Fatal("wan-3.0-c1 must not be wiped from ModelPrice on startup")
	}
	publish := strings.Index(body, `ensureNewtake2apiRouting()`)
	vip13 := strings.Index(body, `ensureVIP13OnEnabledChannels()`)
	if publish < 0 || vip13 < publish {
		t.Fatal("vip13 abilities must be applied after the Newtake channel is published")
	}
	routing, err := os.ReadFile("option_newtake.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(routing)
	for _, want := range []string{
		`newtakeChannelName = "Newtake Video"`,
		`http://newtake2api:38687`,
		`NEWTAKE2API_BASE_URL`,
		`{"wan-3.0-c1":"wan3"}`,
		`¥0.8/次，VIP13 ¥0.5/次，原生480P，2-20秒`,
		`画幅 adaptive/16:9/4:3/1:1/3:4/9:16`,
		`最多10张图片、0段视频、5段音频`,
		`vip13`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
