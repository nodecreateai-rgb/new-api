package model

import (
	"os"
	"strings"
	"testing"
)

func TestFlovaModelsStayOnAfterStartup(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureFlova2apiRouting()`,
		`"wan-3.0"`,
		`"minimax-h3"`,
		`"sora-2"`,
		`"wan-3.0":                            1,`,
		`"minimax-h3":                         0.5`,
		`"sora-2":                             1`,
		`"wan-3.0":            0.8`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, forbidden := range []string{
		`retireFlova2apiRouting()`,
		`"wan-3.0-prime":                      1.5`,
		`"sora-2":                             1.5`,
		`"wan-3.0-prime":      1`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("retired flova setting must not return: %s", forbidden)
		}
	}

	data, err := os.ReadFile("option_flova.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureFlova2apiRouting()`,
		`flovaChannelName = "Flova Video"`,
		`http://flova2api:38686`,
		`FLOVA2API_BASE_URL`,
		`ChannelStatusEnabled`,
		`"wan-3.0"`,
		`"minimax-h3"`,
		`flovaRetiredModels = []string{"sora-2", "wan-3.0-prime"}`,
		`¥1/次，VIP9 ¥0.8/次，原生480P，2-20秒`,
		`¥0.5/次，原生768P，4-15秒`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, `func retireFlova2apiRouting()`) {
		t.Fatal("flova routing must not be retired on startup")
	}
}
