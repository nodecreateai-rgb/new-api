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
		`"gemini-3.5-flash": ""`,
		`"gpt-6-astra":      ""`,
		`"claude-fable-5":   ""`,
		`"claude-opus-5":    ""`,
		`"claude-opus-4-8":  ""`,
		`"gpt-5.6-sol":      ""`,
		`"minimax-m3":       ""`,
		`"kimi-k3":          ""`,
		`"claude-opus-4-8":"claude-opus-4-8"`,
		`"gpt-5.6-sol":"gpt-5.6-sol"`,
		`"minimax-m3":"minimax/minimax-m3"`,
		`"kimi-k3":"kimi-k3"`,
		`ensureGetunikey2apiChatRouting()`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, "输入 ¥") || strings.Contains(s, "（UniKey") {
		t.Fatal("chat marketplace copy must stay empty")
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
