package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageRequestResolvePromptFromInput(t *testing.T) {
	var req ImageRequest
	require.NoError(t, req.UnmarshalJSON([]byte(`{"model":"gpt-image-2","input":"a red apple"}`)))
	req.ResolvePrompt()
	require.Equal(t, "a red apple", req.Prompt)
}

func TestImageRequestResolvePromptFromMessages(t *testing.T) {
	var req ImageRequest
	require.NoError(t, req.UnmarshalJSON([]byte(`{
		"model":"nano-banana-pro",
		"messages":[{"role":"user","content":"一只橘猫"}]
	}`)))
	req.ResolvePrompt()
	require.Equal(t, "一只橘猫", req.Prompt)
}

func TestImageRequestResolvePromptKeepsExplicitPrompt(t *testing.T) {
	var req ImageRequest
	require.NoError(t, req.UnmarshalJSON([]byte(`{"model":"gpt-image-2","prompt":"keep me","input":"ignore"}`)))
	req.ResolvePrompt()
	require.Equal(t, "keep me", req.Prompt)
}

func TestApplyImagePromptAliasesFromResponsesInput(t *testing.T) {
	m := map[string]any{
		"model": "gpt-image-2",
		"input": []any{
			map[string]any{"type": "input_text", "text": "draw a lighthouse"},
			map[string]any{"type": "input_image", "image_url": "https://example.com/a.png"},
		},
	}
	ApplyImagePromptAliases(m)
	require.Equal(t, "draw a lighthouse", m["prompt"])
}
