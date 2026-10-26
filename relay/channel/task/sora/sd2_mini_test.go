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
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-mini-720p"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-mini-480p-c2"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-fast-480p"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "grok-imagine-720p"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-720"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "kling-o3"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "wan-3.0"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "wan-3.0-c1"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "minimax-h3"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "minimax-h3-max"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-fast"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-fast-720p-c1"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "sora-2"}))
}

func TestFlovaUsesFixedPerRequestBilling(t *testing.T) {
	a := &TaskAdaptor{}
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "wan-3.0"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "wan-3.0-c1"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "minimax-h3"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "minimax-h3-max"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "sora-2"}))
}

func TestSeedanceC2UsesFixedPerRequestBilling(t *testing.T) {
	a := &TaskAdaptor{}
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.5-c1"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.5-c2"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-480p"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.5-480p"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.5-720p"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.5-1080p"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-c2"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-480p-c2"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-fast-c2"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "seedance-2.0-fast-480p-c2"}))
	require.False(t, a.UseRequestBillingRatios(&relaycommon.RelayInfo{OriginModelName: "wan-3.0-480p-c2"}))
}

func TestMitoCatalogLocksDurationAndResolution(t *testing.T) {
	body := map[string]interface{}{"duration": 4, "seconds": "4", "size": "1080x1920"}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{}, "seedance-2.0-720p")
	applyOriginModelResolution(body, "seedance-2.0-720p")
	require.Equal(t, 15, body["duration"])
	require.Equal(t, "15", body["seconds"])
	require.Equal(t, "720p", body["resolution"])
	require.Equal(t, "720p", body["size"])

	body = map[string]interface{}{"duration": 4, "seconds": "4", "size": "720x1280"}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Duration: 12}, "seedance-2.5-480p")
	applyOriginModelResolution(body, "seedance-2.5-480p")
	require.Equal(t, 12, body["duration"])
	require.Equal(t, "12", body["seconds"])
	require.Equal(t, "480p", body["resolution"])
	require.Equal(t, "480p", body["size"])

	body = map[string]interface{}{}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Duration: 40}, "seedance-2.5-480p")
	require.Equal(t, 30, body["duration"])
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Duration: 2}, "seedance-2.0-480p")
	applyOriginModelResolution(body, "seedance-2.0-480p")
	require.Equal(t, 4, body["duration"])
	require.Equal(t, "480p", body["resolution"])
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Duration: 20}, "seedance-2.0-480p")
	require.Equal(t, 15, body["duration"])
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

	body = map[string]interface{}{"size": "720x1280"}
	applyOriginModelResolution(body, "seedance-2.0-mini-480p-c2")
	require.Equal(t, "480p", body["resolution"])
	require.Equal(t, "480p", body["size"])

	body = map[string]interface{}{"size": "720x1280"}
	applyOriginModelResolution(body, "seedance-2.0-480p-c2")
	require.Equal(t, "480p", body["resolution"])
	require.Equal(t, "480p", body["size"])

	body = map[string]interface{}{"size": "720x1280"}
	applyOriginModelResolution(body, "seedance-2.0-fast-480p-c2")
	require.Equal(t, "480p", body["resolution"])
	require.Equal(t, "480p", body["size"])

	body = map[string]interface{}{"size": "720x1280"}
	applyOriginModelResolution(body, "seedance-2.0-mini-720p")
	require.Equal(t, "720p", body["resolution"])
	require.Equal(t, "720p", body["size"])

	body = map[string]interface{}{"size": "720x1280"}
	applyOriginModelResolution(body, "grok-imagine-720p")
	require.Equal(t, "720p", body["resolution"])
	require.Equal(t, "720p", body["size"])

	body = map[string]interface{}{"duration": 20, "seconds": "20", "size": "1080x1920"}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Duration: 20}, "seedance-2.0-fast-720p-c1")
	applyOriginModelResolution(body, "seedance-2.0-fast-720p-c1")
	require.Equal(t, 15, body["duration"])
	require.Equal(t, "15", body["seconds"])
	require.Equal(t, "720p", body["resolution"])
	require.Equal(t, "720p", body["size"])
	body = map[string]interface{}{}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Duration: 2}, "seedance-2.0-fast-720p-c1")
	require.Equal(t, 4, body["duration"])

	body = map[string]interface{}{"size": "1080x1920"}
	applyOriginModelResolution(body, "wan-3.0")
	require.Equal(t, "480p", body["resolution"])
	require.Equal(t, "480p", body["size"])

	body = map[string]interface{}{"size": "1080x1920"}
	applyOriginModelResolution(body, "wan-3.0-c1")
	require.Equal(t, "480p", body["resolution"])
	require.Equal(t, "480p", body["size"])
	body = map[string]interface{}{}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Duration: 1}, "wan-3.0-c1")
	require.Equal(t, 2, body["duration"])
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Duration: 30}, "wan-3.0-c1")
	require.Equal(t, 20, body["duration"])

	body = map[string]interface{}{"size": "1080x1920"}
	applyOriginModelResolution(body, "wan-3.0-480p-c2")
	require.Equal(t, "480p", body["resolution"])
	require.Equal(t, "480p", body["size"])

	body = map[string]interface{}{"size": "720x1280"}
	applyOriginModelResolution(body, "minimax-h3")
	require.Equal(t, "768p", body["resolution"])
	require.Equal(t, "768p", body["size"])

	body = map[string]interface{}{"duration": 20, "seconds": "20", "resolution": "2k"}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{Duration: 20}, "minimax-h3-max")
	applyOriginModelResolution(body, "minimax-h3-max")
	require.Equal(t, 12, body["duration"])
	require.Equal(t, "12", body["seconds"])
	require.Equal(t, "768p", body["resolution"])
	require.Equal(t, "768p", body["size"])
	body = map[string]interface{}{}
	applyOriginModelDurationLimit(body, relaycommon.TaskSubmitReq{}, "minimax-h3-max")
	require.Equal(t, 12, body["duration"])

	body = map[string]interface{}{"size": "1080x1920"}
	applyOriginModelResolution(body, "sora-2")
	require.Equal(t, "720p", body["resolution"])
	require.Equal(t, "720p", body["size"])
}

func TestMinimaxH3MaxRequiresReferenceImage(t *testing.T) {
	err := minimaxH3MaxReferenceImageError("minimax-h3-max", nil)
	require.NotNil(t, err)
	require.Equal(t, "至少上传一张参考图", err.Message)
	require.NotNil(t, minimaxH3MaxReferenceImageError("minimax-h3-max", []string{}))
	require.Nil(t, minimaxH3MaxReferenceImageError("minimax-h3-max", []string{"https://example.com/a.png"}))
	require.Nil(t, minimaxH3MaxReferenceImageError("minimax-h3", nil))
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
