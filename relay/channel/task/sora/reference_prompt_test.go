package sora

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestCanonicalizeChineseImageReferencePrompt(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt:   "参考图 1 环绕镜头查看周围",
		ImageURL: "https://assets.example/image.png",
	}
	body := taskSubmitReqToUpstreamVideoBody(req, "seedance-2.0-fast")
	if body["prompt"] != "@Image1 环绕镜头查看周围" {
		t.Fatalf("prompt=%q", body["prompt"])
	}
}

func TestCanonicalizeImageAliases(t *testing.T) {
	for _, input := range []string{"Image 1 环绕镜头", "@image01 环绕镜头", "图片1环绕镜头", "参考图片 01 环绕镜头"} {
		got := canonicalizeImageReferencePrompt(input, 1)
		if got != "@Image1 环绕镜头" {
			t.Fatalf("input=%q got=%q", input, got)
		}
	}
}

func TestCanonicalizeDoesNotInventMissingReference(t *testing.T) {
	got := canonicalizeImageReferencePrompt("参考图 1 环绕镜头查看周围", 0)
	if got != "参考图 1 环绕镜头查看周围" {
		t.Fatalf("prompt=%q", got)
	}
}

func TestCanonicalizeDoesNotRewriteOutOfRangeReference(t *testing.T) {
	got := canonicalizeImageReferencePrompt("参考图 2 环绕镜头", 1)
	if got != "参考图 2 环绕镜头" {
		t.Fatalf("prompt=%q", got)
	}
}

func TestPromptMentionsImageReference(t *testing.T) {
	for _, prompt := range []string{"参考图 1 环绕", "@Image1 orbit", "图片2展示"} {
		if !promptMentionsImageReference(prompt) {
			t.Fatalf("not detected: %q", prompt)
		}
	}
	if promptMentionsImageReference("环绕镜头查看周围") {
		t.Fatal("plain creative prompt must not be detected")
	}
}
