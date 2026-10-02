package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/r2"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestPersistGeneratedVideoToR2Disabled(t *testing.T) {
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	got := persistGeneratedVideoToR2(context.Background(), &model.Task{TaskID: "task_abc"}, &relaycommon.TaskInfo{}, []byte("mp4"))
	if got != "" {
		t.Fatalf("disabled R2 should skip upload, got %q", got)
	}
}

func TestPersistGeneratedVideoToR2UsesUploader(t *testing.T) {
	t.Setenv("R2_ACCESS_KEY_ID", "id")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	orig := uploadGeneratedVideo
	t.Cleanup(func() { uploadGeneratedVideo = orig })
	uploadGeneratedVideo = func(ctx context.Context, cfg r2.Config, key string, body []byte) (string, error) {
		if !strings.HasPrefix(key, "videos/") || !strings.HasSuffix(key, "/task_abc.mp4") {
			t.Fatalf("key %q", key)
		}
		if string(body) != "mp4-bytes" {
			t.Fatalf("body %q", body)
		}
		return cfg.PublicURL(key), nil
	}
	got := persistGeneratedVideoToR2(context.Background(), &model.Task{TaskID: "task_abc"}, nil, []byte("mp4-bytes"))
	if !strings.HasPrefix(got, "https://oss.domie.studio/videos/") || !strings.HasSuffix(got, "/task_abc.mp4") {
		t.Fatalf("got %q", got)
	}
}

func TestPersistGeneratedVideoToR2UploadErrorFallsBack(t *testing.T) {
	t.Setenv("R2_ACCESS_KEY_ID", "id")
	t.Setenv("R2_SECRET_ACCESS_KEY", "secret")
	orig := uploadGeneratedVideo
	t.Cleanup(func() { uploadGeneratedVideo = orig })
	uploadGeneratedVideo = func(ctx context.Context, cfg r2.Config, key string, body []byte) (string, error) {
		return "", errors.New("r2 unavailable")
	}
	got := persistGeneratedVideoToR2(context.Background(), &model.Task{TaskID: "task_abc"}, nil, []byte("mp4-bytes"))
	if got != "" {
		t.Fatalf("upload failure should fall back, got %q", got)
	}
}

func TestDecodeVideoDataURL(t *testing.T) {
	got, err := decodeVideoDataURL("data:video/mp4;base64,AAAA")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\x00\x00\x00" {
		t.Fatalf("got %q", got)
	}
}
