package common

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestValidateMultipartTaskRequestPreservesTextReferenceAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("prompt", "参考图 1 环绕镜头")
	_ = w.WriteField("model", "seedance-2.0-fast-720p")
	_ = w.WriteField("image_url", "https://assets.example/one.png")
	_ = w.WriteField("reference_images", "https://assets.example/two.png")
	_ = w.WriteField("image_refs", "https://assets.example/three.png")
	_ = w.WriteField("duration", "15")
	_ = w.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/videos", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())
	req, err := validateMultipartTaskRequest(c, &RelayInfo{}, "generate")
	if err != nil {
		t.Fatal(err)
	}
	if req.ImageURL != "https://assets.example/one.png" || len(req.ReferenceImages) != 1 || len(req.ImageRefs) != 1 || req.Duration != 15 {
		t.Fatalf("req=%+v", req)
	}
	if !req.HasImage() {
		t.Fatal("text reference aliases must classify request as image-to-video")
	}
}

func TestValidateMultipartTaskRequestPreservesRepeatedSingularImageAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("prompt", "use every repeated image URL")
	_ = w.WriteField("model", "seedance-2.0-720p")
	_ = w.WriteField("image_url", "https://assets.example/one.png")
	_ = w.WriteField("image_url", "https://assets.example/two.png")
	_ = w.WriteField("image_url", "https://assets.example/three.png")
	_ = w.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/videos", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())
	req, err := validateMultipartTaskRequest(c, &RelayInfo{}, "generate")
	if err != nil {
		t.Fatal(err)
	}
	if req.ImageURL != "https://assets.example/one.png" {
		t.Fatalf("image_url=%q", req.ImageURL)
	}
	if len(req.ImageURLs) != 3 {
		t.Fatalf("image_urls=%v want=3", req.ImageURLs)
	}
	collected := collectTaskImageRefsForTest(req)
	if len(collected) != 3 {
		t.Fatalf("collected=%v want=3", collected)
	}
}

func TestValidateMultipartTaskRequestPreservesRepeatedImageField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("prompt", "run")
	_ = w.WriteField("model", "seedance-2.0-720p")
	_ = w.WriteField("image", "https://assets.example/one.png")
	_ = w.WriteField("image", "https://assets.example/two.png")
	_ = w.WriteField("image", "https://assets.example/three.png")
	_ = w.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/videos", &body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())
	req, err := validateMultipartTaskRequest(c, &RelayInfo{}, "generate")
	if err != nil {
		t.Fatal(err)
	}
	if got := collectTaskImageRefsForTest(req); len(got) != 3 {
		t.Fatalf("collected=%v want=3", got)
	}
}

func collectTaskImageRefsForTest(req TaskSubmitReq) []string {
	seen := map[string]bool{}
	var out []string
	add := func(value string) {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	for _, value := range []string{req.Image, req.InputReference, req.ImageURL, req.ReferenceImage} {
		add(value)
	}
	for _, group := range [][]string{req.ImageRefs, req.ImageURLs, req.Images, req.ReferenceImages, req.ExtraImages} {
		for _, value := range group {
			add(value)
		}
	}
	return out
}
