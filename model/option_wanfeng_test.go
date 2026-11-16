package model

import (
	"os"
	"strings"
	"testing"
)

func TestWanfengModelStaysRetired(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	if strings.Contains(body, "ensureWanfengRouting()") {
		t.Fatal("seedance-2.0-720p-c1 must stay unpublished")
	}
	if strings.Contains(body, `"seedance-2.0-720p-c1":               1.5,`) {
		t.Fatal("retired model price must not be enforced")
	}
	if !strings.Contains(body, `"seedance-2.0-720p-c1"`) {
		t.Fatal("seedance-2.0-720p-c1 must be stripped from stored prices")
	}

	retiredVideo, err := os.ReadFile("option_retired_video.go")
	if err != nil {
		t.Fatal(err)
	}
	dead := string(retiredVideo)
	if !strings.Contains(dead, `"seedance-2.0-720p-c1"`) {
		t.Fatal("seedance-2.0-720p-c1 must be in deadVideoModels")
	}
	if !strings.Contains(dead, `"Video 720p C1"`) {
		t.Fatal("Video 720p C1 must be in deadVideoChannels")
	}

	ratio, err := os.ReadFile("../setting/ratio_setting/model_ratio.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ratio), `"seedance-2.0-720p-c1":`) {
		t.Fatal("default ModelPrice must not keep seedance-2.0-720p-c1")
	}
}
