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
	if err := originModelMediaLimit("minimax-h3-768p", relaycommon.TaskSubmitReq{
		VideoURL: "https://example.com/a.mp4",
	}); err == nil || !strings.Contains(err.Error(), "0 reference videos") {
		t.Fatalf("h3 768p video ref: %v", err)
	}
	if err := originModelMediaLimit("minimax-h3-768p", relaycommon.TaskSubmitReq{Images: images}); err == nil {
		t.Fatal("expected h3 768p image cap")
	}
	h3Audios := make([]string, 4)
	for i := range h3Audios {
		h3Audios[i] = "https://example.com/h" + string(rune('a'+i)) + ".mp3"
	}
	if err := originModelMediaLimit("minimax-h3-768p", relaycommon.TaskSubmitReq{Audios: h3Audios}); err == nil {
		t.Fatal("expected h3 768p audio cap")
	}
	if err := originModelMediaLimit("minimax-h3-768p", relaycommon.TaskSubmitReq{
		Images: images[:9],
		Audios: h3Audios[:3],
	}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("minimax-h3-768p", relaycommon.TaskSubmitReq{ImageRefs: images}); err == nil {
		t.Fatal("expected h3 768p image_refs cap")
	}
	if err := originModelMediaLimit("minimax-h3-768p", relaycommon.TaskSubmitReq{AudioRefs: h3Audios}); err == nil {
		t.Fatal("expected h3 768p audio_refs cap")
	}
	if err := originModelMediaLimit("minimax-h3-768p", relaycommon.TaskSubmitReq{VideoRefs: []string{"https://example.com/a.mp4"}}); err == nil {
		t.Fatal("expected h3 768p video_refs cap")
	}
	if err := originModelMediaLimit("minimax-h3-768p", relaycommon.TaskSubmitReq{
		ImageRefs: images[:9],
		AudioRefs: h3Audios[:3],
	}); err != nil {
		t.Fatal(err)
	}
	videos := []string{
		"https://example.com/a.mp4", "https://example.com/b.mp4",
		"https://example.com/c.mp4", "https://example.com/d.mp4",
	}
	if err := originModelMediaLimit("seedance-2.0-720p-c1", relaycommon.TaskSubmitReq{
		Images: images[:9],
		Videos: videos[:3],
		Audios: h3Audios[:3],
	}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("seedance-2.0-720p-c1", relaycommon.TaskSubmitReq{Images: images}); err == nil {
		t.Fatal("expected 720p c1 image cap")
	}
	if err := originModelMediaLimit("seedance-2.0-720p-c1", relaycommon.TaskSubmitReq{Videos: videos}); err == nil {
		t.Fatal("expected 720p c1 video cap")
	}
	if err := originModelMediaLimit("seedance-2.0-720p-c1", relaycommon.TaskSubmitReq{Audios: h3Audios}); err == nil {
		t.Fatal("expected 720p c1 audio cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c1", relaycommon.TaskSubmitReq{
		Images: images[:9],
		Videos: videos[:3],
		Audios: h3Audios[:3],
	}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c1", relaycommon.TaskSubmitReq{Images: images}); err == nil {
		t.Fatal("expected fast 720p c1 image cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c1", relaycommon.TaskSubmitReq{Videos: videos}); err == nil {
		t.Fatal("expected fast 720p c1 video cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c1", relaycommon.TaskSubmitReq{Audios: h3Audios}); err == nil {
		t.Fatal("expected fast 720p c1 audio cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c2", relaycommon.TaskSubmitReq{
		Images: images[:9],
		Audios: h3Audios[:3],
	}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c2", relaycommon.TaskSubmitReq{Images: images}); err == nil {
		t.Fatal("expected fast 720p c2 image cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c2", relaycommon.TaskSubmitReq{Videos: videos[:1]}); err == nil {
		t.Fatal("expected fast 720p c2 video cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c2", relaycommon.TaskSubmitReq{Audios: h3Audios}); err == nil {
		t.Fatal("expected fast 720p c2 audio cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c2", relaycommon.TaskSubmitReq{Audios: h3Audios[:1]}); err == nil {
		t.Fatal("expected fast 720p c2 audio to require an image")
	}
	if err := originModelMediaLimit("seedance-2.0-720p-c2", relaycommon.TaskSubmitReq{
		Images: images[:9],
		Audios: h3Audios[:3],
	}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("seedance-2.0-720p-c2", relaycommon.TaskSubmitReq{Videos: videos[:1]}); err == nil {
		t.Fatal("expected 720p c2 video cap")
	}
	if err := originModelMediaLimit("seedance-2.0-720p-c3", relaycommon.TaskSubmitReq{
		Images: images[:9],
		Videos: videos[:3],
		Audios: h3Audios[:3],
	}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("seedance-2.5-720p-c5", relaycommon.TaskSubmitReq{Videos: videos}); err == nil {
		t.Fatal("expected 2.5 720p c5 video cap")
	}
}
