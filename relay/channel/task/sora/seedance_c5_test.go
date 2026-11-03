package sora

import (
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestOriginModelMediaLimitSeedanceC5(t *testing.T) {
	if err := originModelMediaLimit("seedance-2.0-mini-720p", relaycommon.TaskSubmitReq{}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("seedance-2.0-fast-480p-c1", relaycommon.TaskSubmitReq{
		VideoURL: "https://example.com/a.mp4",
	}); err == nil || !strings.Contains(err.Error(), "0 reference videos") {
		t.Fatalf("video ref: %v", err)
	}
	if err := originModelMediaLimit("seedance-2.0-480p", relaycommon.TaskSubmitReq{
		VideoURL: "https://example.com/b.mp4",
	}); err == nil || !strings.Contains(err.Error(), "0 reference videos") {
		t.Fatalf("480p video ref: %v", err)
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p", relaycommon.TaskSubmitReq{
		VideoURL: "https://example.com/a.mp4",
	}); err == nil || !strings.Contains(err.Error(), "0 reference videos") {
		t.Fatalf("fast 720p video ref: %v", err)
	}
	images := make([]string, 10)
	for i := range images {
		images[i] = "https://example.com/" + string(rune('a'+i)) + ".jpg"
	}
	if err := originModelMediaLimit("seedance-2.0-mini-720p", relaycommon.TaskSubmitReq{Images: images}); err == nil {
		t.Fatal("expected image cap")
	}
	if err := originModelMediaLimit("seedance-2.0", relaycommon.TaskSubmitReq{VideoURL: "https://example.com/a.mp4"}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("wan-3.0-480p", relaycommon.TaskSubmitReq{
		VideoURL: "https://example.com/a.mp4",
	}); err == nil || !strings.Contains(err.Error(), "0 reference videos") {
		t.Fatalf("wan video ref: %v", err)
	}
	audios := make([]string, 6)
	for i := range audios {
		audios[i] = "https://example.com/a" + string(rune('a'+i)) + ".mp3"
	}
	if err := originModelMediaLimit("wan-3.0-480p", relaycommon.TaskSubmitReq{Audios: audios}); err == nil {
		t.Fatal("expected wan audio cap")
	}
}
