package service

import "testing"

func TestParseUpscalerOption(t *testing.T) {
	cases := map[string]string{
		`{"upscaler":"general"}`:            "general",
		`{"upscaler":"anime"}`:              "anime",
		`{"upscaler":"Portrait"}`:           "portrait",
		`{"upscaler":true}`:                 "general",
		`{"metadata":{"upscaler":"anime"}}`: "anime",
		`{"upscaler":false}`:                "",
		`{"upscaler":"none"}`:               "",
		`{"upscaler":"xyz"}`:                "",
		`{"prompt":"x"}`:                    "",
	}
	for in, want := range cases {
		got := ""
		if o := ParseUpscalerOption([]byte(in), nil); o != nil {
			got = o.Model
		}
		if got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}
