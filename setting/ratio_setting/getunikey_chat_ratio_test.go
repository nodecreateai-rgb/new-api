package ratio_setting

import "testing"

func TestGetunikeyChatTokenRatios(t *testing.T) {
	InitRatioSettings()
	cases := []struct {
		model      string
		input      float64
		completion float64
	}{
		{"gemini-3.5-flash", 0.06, 4},
		{"gpt-6-astra", 0.25, 6},
		{"claude-fable-5", 0.40, 5},
		{"claude-opus-5", 0.90, 5},
	}
	for _, tc := range cases {
		got, ok, _ := GetModelRatio(tc.model)
		if !ok || got != tc.input {
			t.Fatalf("%s input ratio %v ok=%v", tc.model, got, ok)
		}
		if GetCompletionRatio(tc.model) != tc.completion {
			t.Fatalf("%s completion %v", tc.model, GetCompletionRatio(tc.model))
		}
		if price, priced := GetModelPrice(tc.model, false); priced {
			t.Fatalf("%s must not use per-call ModelPrice %v", tc.model, price)
		}
	}
}
