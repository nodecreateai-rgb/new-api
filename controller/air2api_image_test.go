package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestAir2APIImageModelsForceAsync(t *testing.T) {
	for _, model := range []string{"gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst", "nano-banana-2", "nano-banana-2-lite", "nano-banana-pro"} {
		if !shouldForceAir2APIAsync(&dto.ImageRequest{Model: model}) {
			t.Fatalf("expected %s to force air2api async", model)
		}
	}
}

func TestEnsureAsyncPayloadForAir2APIUsesNativeTaskEndpoint(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"a cat","async":true,"return_task_id":true}`)
	out, contentType, err := ensureAsyncPayload("application/json", body, false, true)
	if err != nil {
		t.Fatalf("ensureAsyncPayload failed: %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("contentType=%q", contentType)
	}
	var payload map[string]any
	if err := common.Unmarshal(out, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload["model"] != "gpt-image-2" || payload["prompt"] != "a cat" {
		t.Fatalf("payload fields not preserved: %#v", payload)
	}
	for _, key := range []string{"async", "async_task", "return_task_id"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("air2api payload must not include %s: %#v", key, payload)
		}
	}
}

func TestImageTaskAdaptorParsesAir2APIProgressAndURL(t *testing.T) {
	adaptor := NewImageTaskAdaptor()
	body := []byte(`{"task_id":"task_abc","object":"task","status":"completed","progress":100,"image_url":"https://example.com/out.png"}`)
	info, err := adaptor.ParseTaskResult(body)
	if err != nil {
		t.Fatalf("ParseTaskResult failed: %v", err)
	}
	if info.Progress != "100%" {
		t.Fatalf("progress=%q", info.Progress)
	}
	if info.Url != "https://example.com/out.png" {
		t.Fatalf("url=%q", info.Url)
	}
}
