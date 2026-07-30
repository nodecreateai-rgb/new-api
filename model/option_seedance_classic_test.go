package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedanceClassicPublicModels(t *testing.T) {
	require.ElementsMatch(t, []string{
		"seedance-2.0-fast-720p",
		"seedance-2.0-720p",
	}, seedanceClassicPublicModels)
}

func TestHiggsSeedanceUniformPrice(t *testing.T) {
	require.Equal(t, 2.5, higgsSeedancePrice)
}

func TestEnsureCSVValueAddsVIP6Once(t *testing.T) {
	require.Equal(t, "default,vip,vip6", ensureCSVValue("default,vip", "vip6"))
	require.Equal(t, "default,vip6", ensureCSVValue("default,vip6", "vip6"))
}
