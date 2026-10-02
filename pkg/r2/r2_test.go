package r2

import (
	"testing"
	"time"
)

func TestObjectKey(t *testing.T) {
	at := time.Date(2026, 10, 2, 15, 4, 5, 0, time.UTC)
	got, err := ObjectKey("task_abc", at)
	if err != nil {
		t.Fatal(err)
	}
	if got != "videos/2026/10/task_abc.mp4" {
		t.Fatalf("got %q", got)
	}
	got, err = ObjectKey("abc", at)
	if err != nil {
		t.Fatal(err)
	}
	if got != "videos/2026/10/task_abc.mp4" {
		t.Fatalf("got %q", got)
	}
	if _, err := ObjectKey("../secret", at); err == nil {
		t.Fatal("expected invalid task id")
	}
}

func TestPublicURLAndRecognition(t *testing.T) {
	cfg := Config{PublicBaseURL: "https://oss.domie.studio"}
	got := cfg.PublicURL("videos/2026/10/task_abc.mp4")
	want := "https://oss.domie.studio/videos/2026/10/task_abc.mp4"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if !cfg.IsPublicURL(want) {
		t.Fatalf("expected public url %q", want)
	}
	if cfg.IsPublicURL("https://cdn.oreateai.com/x.mp4") {
		t.Fatal("upstream url must not be treated as public R2")
	}
	if cfg.IsPublicURL("http://localhost:3000/v1/videos/task_abc/content") {
		t.Fatal("proxy url must not be treated as public R2")
	}
	if cfg.IsPublicURL("https://oss.domie.studio.evil.com/videos/x.mp4") {
		t.Fatal("suffix host must not match")
	}
}

func TestEnabledRequiresCredentials(t *testing.T) {
	cfg := LoadConfig()
	cfg.AccessKeyID = ""
	cfg.SecretAccessKey = ""
	if cfg.Enabled() {
		t.Fatal("empty credentials must disable R2")
	}
	cfg.AccessKeyID = "id"
	cfg.SecretAccessKey = "secret"
	if !cfg.Enabled() {
		t.Fatal("expected enabled with credentials")
	}
}
