package model

import (
	"os"
	"strings"
	"testing"
)

func TestClaudeChatRoutingUsesNeutralName(t *testing.T) {
	data, err := os.ReadFile("option_claude_chat.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`func ensureClaudeChatRouting()`,
		`claudeChatNeutralName = "Claude Chat"`,
		`claudeOpus55PublicModel = "anthropic/claude-opus-5-5"`,
		`{"anthropic/claude-opus-5-5":"anthropic/claude-opus-5-5"}`,
		`os.Getenv("CLAUDE_CHAT_BASE_URL")`,
		`ensureNamedOpenAIChatRouting(`,
		`func renameLegacyClaudeChatChannels()`,
		`/v1/chat/completions`,
		`func ensureLLMVipOnEnabledChannels()`,
		`"llm-vip"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(s, `neutralName = "`) && strings.Contains(s, "runbios") {
		t.Fatal("channel name must stay Claude Chat")
	}
	if strings.Contains(s, claudeChatNeutralName) && strings.Contains(strings.ToLower(claudeChatNeutralName), "runbios") {
		t.Fatal("neutral channel name leaked vendor")
	}
}

func TestClaudeChatPricingPersistsOnRestart(t *testing.T) {
	option, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(option)
	for _, want := range []string{
		`"anthropic/claude-opus-5-5":          10000000,`,
		`"llm-vip": 1,`,
		`"llm-vip": "LLM VIP分组"`,
		`"anthropic/claude-opus-5-5": 0,`,
		`ensureClaudeChatRouting()`,
		`ensureLLMVipOnEnabledChannels()`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if strings.Contains(strings.ToLower(body), "runbios") {
		t.Fatal("option.go must not mention the upstream project name")
	}
}

func TestDefaultModelPriceIncludesClaudeOpus55(t *testing.T) {
	data, err := os.ReadFile("../setting/ratio_setting/model_ratio.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"anthropic/claude-opus-5-5":          10000000,`) {
		t.Fatal("default ModelPrice must include anthropic/claude-opus-5-5")
	}
}
