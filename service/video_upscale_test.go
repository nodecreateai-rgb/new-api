package service

import "testing"

func TestParseUpscalerOption(t *testing.T) {
	cases := map[string]string{
		`{"upscaler":true}`:          "720P/general/precise",
		`{"upscaler":"4k"}`:          "4K/general/precise",
		`{"upscaler":"1080p-anime"}`: "1080P/anime/precise",
		`{"upscaler":{"quality":"2k","model":"portrait","style":"creative"}}`: "2K/portrait/creative",
		`{"metadata":{"upscaler":"720p creative"}}`:                           "720P/general/creative",
		`{"upscaler":false}`:  "",
		`{"upscaler":"none"}`: "",
		`{"prompt":"x"}`:      "",
	}
	for in, want := range cases {
		got := ""
		if o := ParseUpscalerOption([]byte(in), nil); o != nil {
			got = o.Quality + "/" + o.Model + "/" + o.Style
		}
		if got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
	if o := ParseUpscalerOption(nil, func(k string) string {
		if k == "upscaler" {
			return "1080p"
		}
		return ""
	}); o == nil || o.Quality != "1080P" {
		t.Errorf("form: %+v", o)
	}
}
