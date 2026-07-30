package sora

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestSD2MiniUsesFixedPerRequestBilling(t *testing.T) {
	a := &TaskAdaptor{}
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "sd2-mini"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-720"}))
}

func TestSD2MiniCanonicalBodyMatchesSeedanceMediaShape(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt:      "use all references",
		Duration:    5,
		Resolution:  "720p",
		AspectRatio: "16:9",
		Images:      []string{"https://example.com/image.png"},
		Videos:      []string{"https://example.com/video.mp4"},
		Audios:      []string{"https://example.com/audio.mp3"},
	}
	body := taskSubmitReqToUpstreamVideoBody(req, "seedance2_mini")
	require.Equal(t, "seedance2_mini", body["model"])
	require.Equal(t, 5, body["duration"])
	require.Equal(t, "720p", body["size"])
	require.Equal(t, "16:9", body["aspect_ratio"])
	require.Equal(t, []string{"https://example.com/image.png"}, body["image_refs"])
	require.Equal(t, []string{"https://example.com/video.mp4"}, body["videos"])
	require.Equal(t, []string{"https://example.com/audio.mp3"}, body["audios"])
}
