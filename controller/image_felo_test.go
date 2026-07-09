package controller

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func TestShouldRouteImageRequestToFeloOnlyGPTImage2WithReferences(t *testing.T) {
	if shouldRouteImageRequestToFelo(&dto.ImageRequest{Model: "gpt-image-2", ImageURL: []byte(`"https://example.com/ref.png"`)}) != true {
		t.Fatalf("expected gpt-image-2 with image_url to route to felo2api")
	}
	if shouldRouteImageRequestToFelo(&dto.ImageRequest{Model: "gpt-image-2"}) != false {
		t.Fatalf("text-only gpt-image-2 must stay on the existing channel")
	}
	if shouldRouteImageRequestToFelo(&dto.ImageRequest{Model: "nano-banana-2", ImageURL: []byte(`"https://example.com/ref.png"`)}) != false {
		t.Fatalf("non gpt-image-2 image model must not route to felo2api")
	}
}

func TestEnsureAsyncPayloadForFeloJSONMapsImageURLToReferenceImages(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"改成赛博朋克","image_url":{"url":"https://example.com/ref.png"},"size":"1536x864","quality":"high"}`)
	out, contentType, err := ensureAsyncPayload("application/json", body, true)
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
	if payload["model"] != "gpt-image-2" || payload["prompt"] != "改成赛博朋克" || payload["size"] != "1536x864" || payload["quality"] != "high" {
		t.Fatalf("important fields not preserved: %#v", payload)
	}
	refs, ok := payload["reference_images"].([]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("reference_images should be an array for felo2api: %#v", payload["reference_images"])
	}
	if ref, ok := refs[0].(map[string]any); !ok || ref["url"] != "https://example.com/ref.png" {
		t.Fatalf("reference image object not preserved inside array: %#v", refs[0])
	}
	if _, ok := payload["image_url"]; ok {
		t.Fatalf("image_url should be normalized away for felo2api: %#v", payload)
	}
	if payload["async"] != true || payload["return_task_id"] != true || payload["response_format"] != "url" {
		t.Fatalf("async fields not set: %#v", payload)
	}
}

func TestEnsureAsyncPayloadForFeloMultipartConvertsFilesToReferenceImages(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", "gpt-image-2"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("prompt", "按参考图改风格"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("image", "ref.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("fake image bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	out, contentType, err := ensureAsyncPayload(writer.FormDataContentType(), body.Bytes(), true)
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
	refs, ok := payload["reference_images"].([]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("reference_images not a single-element array: %#v", payload["reference_images"])
	}
	ref, _ := refs[0].(string)
	if ref == "" || !bytes.HasPrefix([]byte(ref), []byte("data:")) || !bytes.Contains([]byte(ref), []byte(";base64,")) {
		t.Fatalf("reference image was not encoded as data URL: %q", ref)
	}
	if mt := http.DetectContentType([]byte("fake image bytes")); mt == "" {
		t.Fatalf("sanity: detect content type returned empty")
	}
}
