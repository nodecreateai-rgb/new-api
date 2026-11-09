package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestInspectComplianceCountsDistinctImagesAndPrice(t *testing.T) {
	body := []byte(`{
		"model":"seedance-2.0",
		"prompt":"walk",
		"compliance_enabled":"true",
		"compliance_mode":"colored_pencil",
		"image_refs":["https://cdn.example/a.png","https://cdn.example/a.png"],
		"image_url":{"url":"https://cdn.example/b.png"}
	}`)
	opt, n, err := InspectCompliance(body, "application/json")
	if err != nil {
		t.Fatal(err)
	}
	if !opt.Enabled || opt.Mode != "colored-pencil" || n != 2 {
		t.Fatalf("opt=%+v images=%d", opt, n)
	}
	if got := opt.Quota(n, 1); got != 2*int(common.QuotaPerUnit) {
		t.Fatalf("quota=%d", got)
	}
	if opt.Quota(n, 0) != 0 {
		t.Fatal("zero group ratio must not charge")
	}
}

func TestInspectComplianceReadsMetadataAndRejectsMode(t *testing.T) {
	body := []byte(`{"metadata":{"compliance_enabled":true,"compliance_mode":"nope"},"image":"https://cdn.example/a.png"}`)
	if _, _, err := InspectCompliance(body, "application/json"); err == nil || !strings.Contains(err.Error(), "invalid compliance_mode") {
		t.Fatalf("err=%v", err)
	}
	off := []byte(`{"compliance_enabled":false,"image_refs":["https://cdn.example/a.png"]}`)
	opt, n, err := InspectCompliance(off, "application/json")
	if err != nil || opt.Enabled || n != 0 {
		t.Fatalf("opt=%+v n=%d err=%v", opt, n, err)
	}
}

func TestApplyComplianceMaterialsRewritesImagesAndStripsFlags(t *testing.T) {
	var posts int
	var gotModel, gotPrompt, gotImage string
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/images/generations":
			posts++
			raw, _ := io.ReadAll(r.Body)
			var req map[string]any
			if err := common.Unmarshal(raw, &req); err != nil {
				t.Errorf("request json: %v", err)
			}
			gotModel, _ = req["model"].(string)
			gotPrompt, _ = req["prompt"].(string)
			gotImage, _ = req["image_url"].(string)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"img1","status":"processing"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/task/img1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"img1","status":"completed","image_url":"` + srvURL(r) + `/outputs/a.png"}`))
		case r.URL.Path == "/outputs/a.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("AICRON2API_BASE_URL", srv.URL)
	t.Setenv("AICRON_COMPLIANCE_MODEL", "")

	body := []byte(`{
		"model":"seedance-2.0",
		"prompt":"keep me",
		"compliance_enabled":true,
		"compliance_mode":"grid",
		"prompt_compliance_enabled":true,
		"image_refs":["https://cdn.example/a.png","https://cdn.example/a.png"],
		"image_url":{"url":"https://cdn.example/a.png"}
	}`)
	out, ct, err := ApplyComplianceMaterials(context.Background(), body, "application/json", ComplianceMaterial{Enabled: true, Mode: "grid"})
	if err != nil {
		t.Fatal(err)
	}
	if ct != "application/json" {
		t.Fatalf("content type %s", ct)
	}
	if posts != 1 {
		t.Fatalf("posts=%d", posts)
	}
	if gotModel != complianceImageModel || gotImage != "https://cdn.example/a.png" || !strings.Contains(gotPrompt, "square grid") {
		t.Fatalf("model=%s image=%s prompt=%s", gotModel, gotImage, gotPrompt)
	}
	var payload map[string]any
	if err := common.Unmarshal(out, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["compliance_enabled"]; ok {
		t.Fatalf("compliance flag still present: %#v", payload)
	}
	if _, ok := payload["compliance_mode"]; ok {
		t.Fatal("compliance mode still present")
	}
	prompt, _ := payload["prompt"].(string)
	if payload["prompt_compliance_enabled"] != true || !strings.Contains(prompt, "keep me") || !strings.HasPrefix(prompt, "【最高优先级·首帧前强制去网格】") || !strings.Contains(prompt, "【生成视频】keep me") {
		t.Fatalf("prompt was not carried for grid mode: %#v", payload["prompt"])
	}
	refs, _ := payload["image_refs"].([]any)
	if len(refs) != 2 || !strings.HasPrefix(refs[0].(string), "data:image/png;base64,") || refs[0] != refs[1] {
		t.Fatalf("refs=%#v", refs)
	}
	imageURL, _ := payload["image_url"].(map[string]any)
	if imageURL["url"] != refs[0] {
		t.Fatalf("image_url=%#v", imageURL)
	}
}

func TestApplyComplianceMaterialsLeavesPlainRequestsUntouched(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("AICRON2API_BASE_URL", srv.URL)
	body := []byte(`{"prompt":"walk","image_refs":["https://cdn.example/a.png"],"compliance_enabled":false}`)
	out, _, err := ApplyComplianceMaterials(context.Background(), body, "application/json", ComplianceMaterial{})
	if err != nil {
		t.Fatal(err)
	}
	if posts != 0 || string(out) != string(body) {
		t.Fatalf("posts=%d out=%s", posts, out)
	}
}

func srvURL(r *http.Request) string {
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}
