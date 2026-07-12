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
