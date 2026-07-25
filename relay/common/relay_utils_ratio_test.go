package common

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestValidateMultipartTaskRequestAcceptsRatioAlias(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("prompt", "test")
	_ = w.WriteField("model", "seedance-video-standard")
	_ = w.WriteField("ratio", "9:16")
	_ = w.Close()
	c.Request = httptest.NewRequest("POST", "/v1/videos", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())

	req, err := validateMultipartTaskRequest(c, &RelayInfo{}, "textGenerate")
	if err != nil {
		t.Fatalf("validate request: %v", err)
	}
	if req.Ratio != "9:16" || req.AspectRatio != "9:16" {
		t.Fatalf("ratio=%q aspect_ratio=%q", req.Ratio, req.AspectRatio)
	}
}

func TestValidateMultipartTaskRequestAcceptsComplianceParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("prompt", "test")
	_ = w.WriteField("model", "seedance-video-standard")
	_ = w.WriteField("compliance_enabled", "false")
	_ = w.WriteField("compliance_mode", "colored-pencil")
	_ = w.Close()
	c.Request = httptest.NewRequest("POST", "/v1/videos", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())

	req, err := validateMultipartTaskRequest(c, &RelayInfo{}, "textGenerate")
	if err != nil {
		t.Fatalf("validate request: %v", err)
	}
	if req.ComplianceEnabled == nil || *req.ComplianceEnabled {
		t.Fatalf("compliance_enabled=%v", req.ComplianceEnabled)
	}
	if req.ComplianceMode != "colored-pencil" {
		t.Fatalf("compliance_mode=%q", req.ComplianceMode)
	}
}

func TestSupportsAudioReferenceOnlyForPixVerseAliases(t *testing.T) {
	for _, model := range []string{
		"seedance-video-fast",
		"seedance-video-standard",
		"seedance-video-fast-per-second",
		"seedance-video-standard-per-second",
		"seedance-2.0-fast-720p",
		"seedance-2.0-720p",
		"seedance-2.0-1080p",
		"seedance-2.0-4k",
	} {
		if !supportsAudioReference(model) {
			t.Fatalf("expected %q to support audio references", model)
		}
	}
	for _, model := range []string{"sora-2", "sd2-c7", "seedance-2.0-standard", "seedance-2.0", ""} {
		if supportsAudioReference(model) {
			t.Fatalf("expected %q to reject audio references", model)
		}
	}
}

func TestValidateMultipartTaskRequestAspectRatioWins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("prompt", "test")
	_ = w.WriteField("model", "seedance-video-standard")
	_ = w.WriteField("ratio", "9:16")
	_ = w.WriteField("aspect_ratio", "16:9")
	_ = w.Close()
	c.Request = httptest.NewRequest("POST", "/v1/videos", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())

	req, err := validateMultipartTaskRequest(c, &RelayInfo{}, "textGenerate")
	if err != nil {
		t.Fatalf("validate request: %v", err)
	}
	if req.AspectRatio != "16:9" {
		t.Fatalf("aspect_ratio=%q", req.AspectRatio)
	}
}

func TestValidateMultipartDirectKeepsAllImageFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("prompt", "use every image")
	_ = w.WriteField("model", "seedance-video-fast")
	for _, item := range []struct {
		field string
		name  string
		data  string
	}{
		{field: "image", name: "one.png", data: "first-image"},
		{field: "image[]", name: "two.png", data: "second-image"},
		{field: "reference_images", name: "three.png", data: "third-image"},
	} {
		part, err := w.CreateFormFile(item.field, item.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(item.data)); err != nil {
			t.Fatal(err)
		}
	}
	_ = w.Close()
	c.Request = httptest.NewRequest("POST", "/v1/videos", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())

	if taskErr := ValidateMultipartDirect(c, &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}}); taskErr != nil {
		t.Fatalf("validate request: %v", taskErr)
	}
	req, err := GetTaskRequest(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Images) != 3 {
		t.Fatalf("images=%d want=3", len(req.Images))
	}
}

func TestValidateMultipartTaskRequestKeepsAllImageFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("prompt", "use every image")
	for _, item := range []struct {
		field string
		name  string
		data  string
	}{
		{field: "image", name: "one.png", data: "first-image"},
		{field: "image[]", name: "two.png", data: "second-image"},
		{field: "reference_images", name: "three.png", data: "third-image"},
	} {
		part, err := w.CreateFormFile(item.field, item.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(item.data)); err != nil {
			t.Fatal(err)
		}
	}
	_ = w.Close()
	c.Request = httptest.NewRequest("POST", "/v1/videos", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())

	req, err := validateMultipartTaskRequest(c, &RelayInfo{}, "generate")
	if err != nil {
		t.Fatalf("validate request: %v", err)
	}
	if len(req.Images) != 3 {
		t.Fatalf("images=%d want=3", len(req.Images))
	}
	for i, ref := range req.Images {
		if !strings.HasPrefix(ref, "data:") || !strings.Contains(ref, ";base64,") {
			t.Fatalf("image %d is not a data URL: %q", i, ref)
		}
	}
}
