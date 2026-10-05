package model

import (
	"os"
	"strings"
	"testing"
)

func TestFlovaRoutingUsesFlovaGateway(t *testing.T) {
	data, err := os.ReadFile("option_flova.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureFlova2apiRouting()`,
		`publicModels := []string{"wan-3.0-prime", "minimax-h3", "sora-2"}`,
		`baseURL = "http://flova2api:38686"`,
		`"wan-3.0-prime":"wan-3.0-prime","minimax-h3":"minimax-h3","sora-2":"sora-2"`,
		`os.Getenv("FLOVA2API_BASE_URL")`,
		`os.Getenv("FLOVA2API_GATEWAY_KEY")`,
		`原生720P，30秒，10图片5视频5音频`,
		`768P，15秒，全参`,
		`12秒，720P，单图，不支持真人`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}

	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureFlova2apiRouting()`,
		`"wan-3.0-prime":                      2`,
		`"minimax-h3":                         0.5`,
		`"sora-2":                             1.5`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
