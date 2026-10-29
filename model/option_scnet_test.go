package model

import (
	"os"
	"strings"
	"testing"
)

func TestScnetModelsListedWithPrices(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureScnetRouting()`,
		`"minimax-h3-2k":                      0.6,`,
		`"seedance-2.5-720p-c4":               4,`,
		`"seedance-2.5-720p-c4":   1.5,`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	data, err := os.ReadFile("option_scnet.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`scnetChannelName = "Video 2K C4"`,
		`"minimax-h3-2k":"minimax-h3"`,
		`"seedance-2.5-720p-c4":"seedance-2.5"`,
		`¥0.6/次，固定2K输出`,
		`¥4/次，固定720P`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(strings.ToLower(s[strings.Index(s, "modelDescriptions"):strings.Index(s, "endpoint :=")]), "scnet") {
		t.Fatal("public descriptions must not name the upstream")
	}
}
