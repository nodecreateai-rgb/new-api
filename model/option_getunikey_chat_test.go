package model

import (
	"os"
	"strings"
	"testing"
)

func TestGetunikeyChatRoutingUsesOpenAIChannel(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureGetunikey2apiChatRouting()`,
		`neutralName = "UniKey Chat"`,
		`"gemini-3.5-flash":"google/gemini-3.5-flash"`,
		`constant.ChannelTypeOpenAI`,
		`/v1/chat/completions`,
		`输入 ¥0.12/M`,
		`输入 ¥0.50/M`,
		`输入 ¥0.80/M`,
		`输入 ¥1.80/M`,
		`输入 ¥0.20/M`,
		`输入 ¥1.00/M`,
		`"minimax-m3":"minimax/minimax-m3"`,
		`"kimi-k3":"kimi-k3"`,
		`ensureGetunikey2apiChatRouting()`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
}

func TestMergeJSONFloatMap(t *testing.T) {
	next, ok := mergeJSONFloatMap(`{"gpt-4":15}`, map[string]float64{"gemini-3.5-flash": 0.06})
	if !ok || !strings.Contains(next, `"gemini-3.5-flash"`) || !strings.Contains(next, `"gpt-4"`) {
		t.Fatalf("merge %q ok=%v", next, ok)
	}
	_, ok = mergeJSONFloatMap(next, map[string]float64{"gemini-3.5-flash": 0.06})
	if ok {
		t.Fatal("unchanged map should not rewrite")
	}
}
