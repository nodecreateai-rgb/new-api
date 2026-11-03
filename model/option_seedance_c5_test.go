package model

import (
	"os"
	"strings"
	"testing"
)

func TestVideoSeedanceC5ListedWithPrices(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`ensureVideoSeedanceC5Routing()`,
		`"seedance-2.0-mini-720p":             1,`,
		`"seedance-2.0-fast-480p-c1":          1.5,`,
		`groupPrices["seedance-2.0-fast-480p-c1"] = priceFast480`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	data, err := os.ReadFile("option_seedance_c5.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`seedanceC5ChannelName = "Video Seedance C5"`,
		`"seedance-2.0-mini-720p":"seedance-2-0-mini"`,
		`"seedance-2.0-fast-480p-c1":"seedance-2-0-fast"`,
		`"wan-3.0-480p":"wan-3-0"`,
		`"seedance-2.0-fast-720p":"seedance-2-0-fast"`,
		`"seedance-2.0-480p":"seedance-2-0"`,
		`原生人脸，最多9张图片、0段视频、3段音频。`,
		`最多10张图片、0段视频、5段音频`,
		`原生人脸，最多9张图片、0段视频、3段音频`,
		`¥1/次，固定720P`,
		`VIP10 ¥1.4/次，其他分组 ¥1.5/次`,
		`不卡人脸，最多9张图片、0段视频、3段音频`,
		`http://aqy2api:38693`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	desc := s[strings.Index(s, "modelDescriptions"):strings.Index(s, "endpoint :=")]
	for _, forbidden := range []string{"iqiyi", "nadou", "aqy", "爱奇艺", "纳逗"} {
		if strings.Contains(strings.ToLower(desc), forbidden) {
			t.Fatalf("public description names the upstream: %s", forbidden)
		}
	}
}
