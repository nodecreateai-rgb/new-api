package common

import (
	"testing"

	basecommon "github.com/QuantumNous/new-api/common"
)

func TestTaskSubmitReqAcceptsRatioAlias(t *testing.T) {
	var req TaskSubmitReq
	if err := basecommon.Unmarshal([]byte(`{"model":"seedance-video-standard","ratio":"9:16"}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.AspectRatio != "9:16" {
		t.Fatalf("aspect_ratio=%q ratio=%q", req.AspectRatio, req.Ratio)
	}
}

func TestTaskSubmitReqAspectRatioTakesPrecedence(t *testing.T) {
	var req TaskSubmitReq
	if err := basecommon.Unmarshal([]byte(`{"aspect_ratio":"16:9","ratio":"9:16"}`), &req); err != nil {
		t.Fatal(err)
	}
	if req.AspectRatio != "16:9" {
		t.Fatalf("aspect_ratio=%q", req.AspectRatio)
	}
}

func TestTaskSubmitReqAcceptsResolutionAndAspectAliases(t *testing.T) {
	var byResolution TaskSubmitReq
	if err := basecommon.Unmarshal([]byte(`{"model":"seedance-video-fast","resolution":"1920x1080"}`), &byResolution); err != nil {
		t.Fatal(err)
	}
	if byResolution.Resolution != "1920x1080" {
		t.Fatalf("resolution=%q", byResolution.Resolution)
	}

	var byAspect TaskSubmitReq
	if err := basecommon.Unmarshal([]byte(`{"model":"seedance-video-fast","aspect":"16:9"}`), &byAspect); err != nil {
		t.Fatal(err)
	}
	if byAspect.AspectRatio != "16:9" || byAspect.Aspect != "16:9" {
		t.Fatalf("aspect=%q aspect_ratio=%q", byAspect.Aspect, byAspect.AspectRatio)
	}
}
