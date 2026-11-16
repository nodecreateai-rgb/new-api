package model

import (
	"os"
	"strings"
	"testing"
)

func TestScnetModelsStayRetired(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	if strings.Contains(body, "ensureScnetRouting()") {
		t.Fatal("minimax-h3-2k, seedance-2.5-720p-c4, and seedance-2.0 must stay unpublished; retireDeadVideoModels strips them")
	}
	for _, price := range []string{
		`"minimax-h3-2k":                      1,`,
		`"seedance-2.5-720p-c4":               4,`,
		`"seedance-2.0":                       2,`,
		`"seedance-2.5-720p-c4":   1.5,`,
	} {
		if strings.Contains(body, price) {
			t.Fatalf("retired model price must not be enforced: %s", price)
		}
	}
	for _, gone := range []string{`"minimax-h3-2k"`, `"seedance-2.5-720p-c4"`, `"seedance-2.0"`} {
		if strings.Count(body, gone) < 1 {
			t.Fatalf("%s must be stripped from stored prices", gone)
		}
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	for _, model := range []string{`"minimax-h3-2k"`, `"seedance-2.5-720p-c4"`, `"seedance-2.0"`} {
		if !strings.Contains(dead, model) {
			t.Fatalf("%s must be in deadVideoModels", model)
		}
	}
	if !strings.Contains(dead, `"Video 2K C4"`) {
		t.Fatal("Video 2K C4 must be in deadVideoChannels")
	}

	ratio, err := os.ReadFile("../setting/ratio_setting/model_ratio.go")
	if err != nil {
		t.Fatal(err)
	}
	defaults := string(ratio)
	for _, price := range []string{
		`"minimax-h3-2k":`,
		`"seedance-2.5-720p-c4":`,
	} {
		if strings.Contains(defaults, price) {
			t.Fatalf("default ModelPrice must not keep %s", price)
		}
	}
}
