package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveTaskPricePatchesIncludesFixedPriceSeedanceModels(t *testing.T) {
	patches := resolveTaskPricePatches("")
	require.Contains(t, patches, "seedance-2.0-fast-720p")
	require.Contains(t, patches, "seedance-2.0-720p")
	require.Contains(t, patches, "seedance-2.0-1080p")
	require.Contains(t, patches, "seedance-2.0-4k")
}

func TestResolveTaskPricePatchesEnvExtendsDefaults(t *testing.T) {
	patches := resolveTaskPricePatches(" custom-a, custom-b, seedance-2.0-4k ")
	require.Contains(t, patches, "seedance-2.0-4k")
	require.Contains(t, patches, "seedance-video-fast")
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
