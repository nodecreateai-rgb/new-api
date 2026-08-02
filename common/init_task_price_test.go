package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveTaskPricePatchesIncludesFixedPriceMyEditModels(t *testing.T) {
	patches := resolveTaskPricePatches("")
	for _, model := range []string{
		"sd2-fast", "seedance-720", "seedance-2.0-fast-720p", "seedance-2.0-720p", "seedance-2.0-1080p",
		"wan2.7-720p", "wan2.7-1080p", "viduq3-turbo-720p", "viduq3-pro-1080p",
		"happyhorse-1.1-1080p", "sora-2-1080p", "kling-v3-4k",
	} {
		require.Contains(t, patches, model)
	}
	require.NotContains(t, patches, "seedance-2.0-4k")
	require.NotContains(t, patches, "happy-horse-1.1")
	require.NotContains(t, patches, "happyhorse-1.1")
	require.NotContains(t, patches, "kling-v3")
	require.NotContains(t, patches, "wan2.7")
	require.NotContains(t, patches, "viduq3")
}

func TestResolveTaskPricePatchesEnvExtendsDefaults(t *testing.T) {
	patches := resolveTaskPricePatches(" custom-a, custom-b, seedance-2.0-4k ")
	require.Contains(t, patches, "seedance-2.0-4k")
	require.NotContains(t, patches, "seedance-video-fast")
	require.Contains(t, patches, "sora-2-pro")
	require.Contains(t, patches, "custom-a")
	require.Contains(t, patches, "custom-b")
	require.Equal(t, 1, countString(patches, "seedance-2.0-4k"))
}

func TestIsPerSecondTaskModelOnlyMatchesExplicitAliases(t *testing.T) {
	for _, model := range []string{"seedance-video-fast-per-second", "video_per_second", "测试按秒模型"} {
		require.True(t, IsPerSecondTaskModel(model), model)
	}
	for _, model := range []string{"seedance-2.0-4k", "seedance-2.0-720p", "sora-2-pro", "happy-horse-1.1"} {
		require.False(t, IsPerSecondTaskModel(model), model)
	}
}

func countString(values []string, target string) int {
	n := 0
	for _, value := range values {
		if value == target {
			n++
		}
	}
	return n
}
