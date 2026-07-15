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

func TestResolveTaskPricePatchesEnvOverridesDefaults(t *testing.T) {
	require.Equal(t, []string{"custom-a", "custom-b"}, resolveTaskPricePatches(" custom-a, custom-b "))
}
