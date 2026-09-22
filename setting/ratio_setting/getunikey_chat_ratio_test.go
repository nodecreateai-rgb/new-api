package ratio_setting

import "testing"

func TestGetunikeyChatTokenRatios(t *testing.T) {
	InitRatioSettings()
	cases := []struct {
		model      string
		input      float64
		completion float64
	}{
		{"gemini-3.5-flash", 0.015, 10.0 / 3.0},
		{"minimax-m3", 0.015, 10.0 / 3.0},
		{"kimi-k3", 0.015, 10.0 / 3.0},
		{"glm-5.3", 0.015, 10.0 / 3.0},
		{"deepseek-v4.1-flash", 0.0005, 1},
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
		displayIn := 2 * tc.input
		displayOut := displayIn * tc.completion
		switch tc.model {
		case "deepseek-v4.1-flash":
			if displayIn != 0.001 || displayOut != 0.001 {
				t.Fatalf("%s plaza %v/%v", tc.model, displayIn, displayOut)
			}
		default:
			if displayIn != 0.03 || displayOut != 0.1 {
				t.Fatalf("%s plaza %v/%v", tc.model, displayIn, displayOut)
			}
		}
	}
}
