package common

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
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
