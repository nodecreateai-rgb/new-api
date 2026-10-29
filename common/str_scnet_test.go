package common

import (
	"strings"
	"testing"
)

func TestScnetErrorsAreMasked(t *testing.T) {
	for _, raw := range []string{
		"scnet api 429 code=200100: The daily number of requests for the model has reached its maximum quota",
		"download https://oss.ksai.scnet.cn:58003/llm/x.mp4: http 403",
		"POST /api/video: http 500 from scnet2api",
	} {
		got := PublicErrorMessage(raw)
		if strings.Contains(strings.ToLower(got), "scnet") || strings.Contains(got, "ksai") || strings.Contains(got, "http") {
			t.Fatalf("leak: %q -> %q", raw, got)
		}
	}
	if got := MaskUpstreamProviderInfo("served by scnet2api at api.scnet.cn"); strings.Contains(strings.ToLower(got), "scnet") {
		t.Fatalf("leak: %q", got)
	}
}
