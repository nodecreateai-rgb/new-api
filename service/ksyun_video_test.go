package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestKsyunSeedanceReturnsUpstreamVideo(t *testing.T) {
	task := &model.Task{Properties: model.Properties{OriginModelName: "seedance-2.5"}}
	ch := &model.Channel{Name: "Ksyun Seedance 2.5"}
	base := "http://ksyun2api:38922"
	ch.BaseURL = &base
	if !ksyunSeedanceReturnsUpstreamVideo(ch, task) {
		t.Fatal("ksyun seedance-2.5 must return the upstream video URL")
	}
	other := &model.Task{Properties: model.Properties{OriginModelName: "seedance-2.0-mini-480p"}}
	if ksyunSeedanceReturnsUpstreamVideo(ch, other) {
		t.Fatal("other models on the ksyun channel must keep object storage")
	}
	roboneo := &model.Channel{Name: "Roboneo Seedance Mini"}
	roboneoBase := "http://roboneo2api:38688"
	roboneo.BaseURL = &roboneoBase
	if ksyunSeedanceReturnsUpstreamVideo(roboneo, task) {
		t.Fatal("seedance-2.5 on another channel must keep object storage")
	}
	if got := absoluteUpstreamVideoURL(ch, "https://cdn.example.com/a.mp4"); got != "https://cdn.example.com/a.mp4" {
		t.Fatalf("absolute url %q", got)
	}
	if got := absoluteUpstreamVideoURL(ch, "https://oss.domie.studio/videos/a.mp4"); got != "" {
		t.Fatalf("object storage url must not be treated as upstream, got %q", got)
	}
}
