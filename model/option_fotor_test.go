package model

import (
	"os"
	"strings"
	"testing"
)

func TestFotor2apiRoutingUsesWan480pC2(t *testing.T) {
	data, err := os.ReadFile("option_fotor.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureFotor2apiRouting()`,
		`const modelsCSV = "wan-3.0-480p-c2"`,
		`{"wan-3.0-480p-c2":"wan-3.0"}`,
		`publicModels := []string{"wan-3.0-480p-c2"}`,
		`baseURL = "http://fotor2api:38684"`,
		`os.Getenv("FOTOR2API_BASE_URL")`,
		`os.Getenv("FOTOR2API_GATEWAY_KEY")`,
		`¥0.5/次`,
		`480P，10图片5视频5音频，最大20秒`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, leftover := range []string{
		`func retireFotor2apiRouting()`,
		`seedance-2.0-c2`,
		`wan-3.0-c2`,
	} {
		if strings.Contains(s, leftover) {
			t.Fatalf("leftover %s", leftover)
		}
	}

	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureFotor2apiRouting()`,
		`"wan-3.0-480p-c2":                    0.5`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(body, "retireFotor2apiRouting()") {
		t.Fatal("fotor routing must be ensured on startup")
	}

	ratio, err := os.ReadFile("../setting/ratio_setting/model_ratio.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ratio), `"wan-3.0-480p-c2":                    0.5`) {
		t.Fatal("missing default price wan-3.0-480p-c2")
	}
}
