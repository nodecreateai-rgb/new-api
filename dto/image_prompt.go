package dto

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

var imagePromptAliasKeys = []string{
	"input",
	"text",
	"query",
	"prompt_text",
	"instruction",
	"instructions",
	"messages",
	"contents",
}

// ResolvePrompt fills Prompt from common client aliases (input/text/messages/...).
func (i *ImageRequest) ResolvePrompt() {
	if i == nil {
		return
	}
	payload := map[string]any{"prompt": i.Prompt}
	for key, raw := range i.Extra {
		var value any
		if err := common.Unmarshal(raw, &value); err == nil {
			payload[key] = value
			continue
		}
		payload[key] = string(raw)
	}
	if s := PromptFromPayload(payload); s != "" {
		i.Prompt = s
	}
}

// ApplyImagePromptAliases writes a non-empty prompt into m["prompt"] using aliases.
func ApplyImagePromptAliases(m map[string]any) {
	if m == nil {
		return
	}
	if s := PromptFromPayload(m); s != "" {
		m["prompt"] = s
	}
}

// PromptFromPayload returns prompt, or a value extracted from alias fields.
func PromptFromPayload(m map[string]any) string {
	if m == nil {
		return ""
	}
	if s := strings.TrimSpace(anyPromptString(m["prompt"])); s != "" {
		return s
	}
	for _, key := range imagePromptAliasKeys {
		if s := PromptFromAny(m[key]); s != "" {
			return s
		}
	}
	if params, ok := m["parameters"].(map[string]any); ok {
		if s := PromptFromPayload(params); s != "" {
			return s
		}
	}
	return PromptFromAny(m["content"])
}

func PromptFromAny(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := PromptFromAny(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		return promptFromObject(t)
	default:
		return ""
	}
}

func promptFromObject(obj map[string]any) string {
	if obj == nil {
		return ""
	}
	typ := strings.ToLower(anyPromptString(obj["type"]))
	if strings.Contains(typ, "image") {
		if s := strings.TrimSpace(anyPromptString(obj["text"])); s != "" {
			return s
		}
		if s := strings.TrimSpace(anyPromptString(obj["input_text"])); s != "" {
			return s
		}
		return ""
	}
	for _, key := range []string{"text", "input_text", "prompt", "content", "input"} {
		if _, ok := obj[key]; !ok {
			continue
		}
		if s := PromptFromAny(obj[key]); s != "" {
			return s
		}
	}
	if _, ok := obj["parts"]; ok {
		return PromptFromAny(obj["parts"])
	}
	return ""
}

func anyPromptString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
