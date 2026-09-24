package sora

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestSD2MiniUsesFixedPerRequestBilling(t *testing.T) {
	a := &TaskAdaptor{}
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "sd2.5"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "sd2-mini"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "sd2-fast"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "sd2-c6"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "sd2-c7"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-mini"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-mini-480p"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-720"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "kling-o3"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-fast"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "sora-2"}))
}

func TestSeedanceC2UsesFixedPerRequestBilling(t *testing.T) {
	a := &TaskAdaptor{}
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.5-c1"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.5-c2"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.5-480p"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.5-720p"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.5-1080p"}))
}

func TestLaihuaSeedanceLocksResolutionFromModel(t *testing.T) {
	body := map[string]interface{}{"size": "1080x1920", "aspect_ratio": "9:16"}
	applyOriginModelResolution(body, "seedance-2.5-480p")
	require.Equal(t, "480p", body["resolution"])
	require.Equal(t, "480p", body["size"])

	body = map[string]interface{}{"size": "1080x1920", "aspect_ratio": "9:16"}
	applyOriginModelResolution(body, "seedance-2.5-720p")
	require.Equal(t, "720p", body["resolution"])
	require.Equal(t, "720p", body["size"])

	body = map[string]interface{}{"size": "720x1280"}
	applyOriginModelResolution(body, "seedance-2.5-1080p")
	require.Equal(t, "1080p", body["resolution"])
	require.Equal(t, "1080p", body["size"])
}

func TestKlingO3UpstreamBodyPassesDurationAspectAndImages(t *testing.T) {
	req := relaycommon.TaskSubmitReq{
		Prompt:      "cinematic apple on table",
		Duration:    15,
		AspectRatio: "9:16",
		Images:      []string{"https://example.com/frame1.png", "https://example.com/frame2.png"},
	}
	body := taskSubmitReqToUpstreamVideoBody(req, "kling-o3")
	require.Equal(t, "kling-o3", body["model"])
	require.Equal(t, 15, body["duration"])
	require.Equal(t, "9:16", body["aspect_ratio"])
	require.Equal(t, []string{"https://example.com/frame1.png", "https://example.com/frame2.png"}, body["image_refs"])
	require.NotContains(t, body, "image_url")
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
