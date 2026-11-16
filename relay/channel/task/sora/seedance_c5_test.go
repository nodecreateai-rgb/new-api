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
	if err := originModelMediaLimit("seedance-2.0", relaycommon.TaskSubmitReq{Images: images}); err == nil {
		t.Fatal("seedance-2.0 accepts at most 9 images")
	}
	if err := originModelMediaLimit("seedance-2.0", relaycommon.TaskSubmitReq{Videos: []string{"a", "b", "c", "d"}}); err == nil {
		t.Fatal("seedance-2.0 accepts at most 3 videos")
	}
	if err := originModelMediaLimit("seedance-2.0", relaycommon.TaskSubmitReq{Audios: []string{"a", "b", "c", "d"}}); err == nil {
		t.Fatal("seedance-2.0 accepts at most 3 audios")
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
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c3", relaycommon.TaskSubmitReq{
		Images: images[:9],
		Videos: videos[:3],
		Audios: h3Audios[:3],
	}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c3", relaycommon.TaskSubmitReq{Images: images}); err == nil {
		t.Fatal("expected fast 720p c3 image cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c3", relaycommon.TaskSubmitReq{Videos: videos}); err == nil {
		t.Fatal("expected fast 720p c3 video cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c3", relaycommon.TaskSubmitReq{Audios: h3Audios}); err == nil {
		t.Fatal("expected fast 720p c3 audio cap")
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
	flashImages := make([]string, 11)
	for i := range flashImages {
		flashImages[i] = "https://example.com/f" + strings.Repeat("x", i+1) + ".jpg"
	}
	flashAudios := make([]string, 6)
	for i := range flashAudios {
		flashAudios[i] = "https://example.com/fa" + strings.Repeat("x", i+1) + ".mp3"
	}
	if err := originModelMediaLimit("seedance-2.5-flash-720p", relaycommon.TaskSubmitReq{
		Images: flashImages[:10],
		Audios: flashAudios[:5],
	}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("seedance-2.5-flash-720p", relaycommon.TaskSubmitReq{Images: flashImages}); err == nil {
		t.Fatal("expected seedance-2.5-flash-720p image cap")
	}
	if err := originModelMediaLimit("seedance-2.5-flash-720p", relaycommon.TaskSubmitReq{Videos: videos[:1]}); err == nil {
		t.Fatal("expected seedance-2.5-flash-720p video cap")
	}
	if err := originModelMediaLimit("seedance-2.5-flash-720p", relaycommon.TaskSubmitReq{Audios: flashAudios}); err == nil {
		t.Fatal("expected seedance-2.5-flash-720p audio cap")
	}
	if err := originModelMediaLimit("seedance-2.5-flash-720p", relaycommon.TaskSubmitReq{Audios: flashAudios[:1]}); err == nil {
		t.Fatal("expected seedance-2.5-flash-720p audio to require an image")
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
	c5Images := make([]string, 31)
	for i := range c5Images {
		c5Images[i] = "https://example.com/c5i" + strings.Repeat("x", i+1) + ".jpg"
	}
	c5Videos := make([]string, 11)
	for i := range c5Videos {
		c5Videos[i] = "https://example.com/c5v" + strings.Repeat("x", i+1) + ".mp4"
	}
	c5Audios := make([]string, 11)
	for i := range c5Audios {
		c5Audios[i] = "https://example.com/c5a" + strings.Repeat("x", i+1) + ".mp3"
	}
	if err := originModelMediaLimit("seedance-2.5-720p-c5", relaycommon.TaskSubmitReq{
		Images: c5Images[:30],
		Videos: c5Videos[:10],
		Audios: c5Audios[:10],
	}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("seedance-2.5-720p-c5", relaycommon.TaskSubmitReq{Images: c5Images}); err == nil {
		t.Fatal("expected 2.5 720p c5 image cap")
	}
	if err := originModelMediaLimit("seedance-2.5-720p-c5", relaycommon.TaskSubmitReq{Videos: c5Videos}); err == nil {
		t.Fatal("expected 2.5 720p c5 video cap")
	}
	if err := originModelMediaLimit("seedance-2.5-720p-c5", relaycommon.TaskSubmitReq{Audios: c5Audios}); err == nil {
		t.Fatal("expected 2.5 720p c5 audio cap")
	}
	flashC5Images := make([]string, 11)
	for i := range flashC5Images {
		flashC5Images[i] = "https://example.com/f" + strings.Repeat("y", i+1) + ".jpg"
	}
	flashC5Videos := make([]string, 6)
	for i := range flashC5Videos {
		flashC5Videos[i] = "https://example.com/f" + strings.Repeat("y", i+1) + ".mp4"
	}
	flashC5Audios := make([]string, 6)
	for i := range flashC5Audios {
		flashC5Audios[i] = "https://example.com/f" + strings.Repeat("y", i+1) + ".mp3"
	}
	for _, model := range []string{"seedance-2.5-flash-480p-c5", "seedance-2.5-flash-720p-c5"} {
		if err := originModelMediaLimit(model, relaycommon.TaskSubmitReq{
			Images: flashC5Images[:10],
			Videos: flashC5Videos[:5],
			Audios: flashC5Audios[:5],
		}); err != nil {
			t.Fatal(err)
		}
		if err := originModelMediaLimit(model, relaycommon.TaskSubmitReq{Images: flashC5Images}); err == nil {
			t.Fatalf("%s image cap", model)
		}
		if err := originModelMediaLimit(model, relaycommon.TaskSubmitReq{Videos: flashC5Videos}); err == nil {
			t.Fatalf("%s video cap", model)
		}
		if err := originModelMediaLimit(model, relaycommon.TaskSubmitReq{Audios: flashC5Audios}); err == nil {
			t.Fatalf("%s audio cap", model)
		}
	}
	fastImages := make([]string, 10)
	fastVideos := make([]string, 4)
	fastAudios := make([]string, 4)
	for i := range fastImages {
		fastImages[i] = "https://example.com/fast" + strings.Repeat("z", i+1) + ".jpg"
	}
	for i := range fastVideos {
		fastVideos[i] = "https://example.com/fast" + strings.Repeat("z", i+1) + ".mp4"
	}
	for i := range fastAudios {
		fastAudios[i] = "https://example.com/fast" + strings.Repeat("z", i+1) + ".mp3"
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c5", relaycommon.TaskSubmitReq{
		Images: fastImages[:9],
		Videos: fastVideos[:3],
		Audios: fastAudios[:3],
	}); err != nil {
		t.Fatal(err)
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c5", relaycommon.TaskSubmitReq{Images: fastImages}); err == nil {
		t.Fatal("expected fast 720p c5 image cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c5", relaycommon.TaskSubmitReq{Videos: fastVideos}); err == nil {
		t.Fatal("expected fast 720p c5 video cap")
	}
	if err := originModelMediaLimit("seedance-2.0-fast-720p-c5", relaycommon.TaskSubmitReq{Audios: fastAudios}); err == nil {
		t.Fatal("expected fast 720p c5 audio cap")
	}
}
