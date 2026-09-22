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
		`ensureGetunikey2apiChatRouting()`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, "输入 ¥") || strings.Contains(s, "（UniKey") {
		t.Fatal("chat marketplace copy must stay empty")
	}
	for _, retired := range []string{
		`"gpt-6-astra":"gpt-6-astra"`,
		`"claude-fable-5":"claude-fable-5"`,
		`"claude-opus-5":"claude-opus-5"`,
		`"minimax/minimax-m3"`,
	} {
		if strings.Contains(s, retired) {
			t.Fatalf("retired unikey mapping still present: %s", retired)
		}
	}
}

func TestWorkbuddyChatRouting(t *testing.T) {
	data, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureWorkbuddy2apiChatRouting()`,
		`neutralName = "WorkBuddy Chat"`,
		`baseURL = "http://workbuddy2api:8788"`,
		`os.Getenv("WORKBUDDY2API_BASE_URL")`,
		`"minimax-m3":"minimax-m3"`,
		`"glm-5.3":"glm-5.3"`,
		`"deepseek-v4.1-flash":"deepseek-v4.1-flash"`,
		`"kimi-k3":"kimi-k3"`,
		`retiredUnikeyChatModels`,
		`ensureWorkbuddy2apiChatRouting()`,
		`retireMarketplaceModels(retiredUnikeyChatModels)`,
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
