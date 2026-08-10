package model

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeedanceClassicPublicModels(t *testing.T) {
	require.ElementsMatch(t, []string{
		"seedance-2.0-fast-720p",
		"seedance-2.0-720p",
		"kling-o3",
	}, seedanceClassicPublicModels)
}

func TestClassicRoutingDefaultsToUnambiguousAdobeAlias(t *testing.T) {
	data, err := os.ReadFile("option.go")
	require.NoError(t, err)
	s := string(data)
	require.Contains(t, s, `baseURL = "http://adobe2api:39918"`)
	require.NotContains(t, s, `baseURL = "http://video-seedance-classic:39918"`)
}

func TestClassicMarketplaceRegistrationPrecedesGatewayKeyValidation(t *testing.T) {
	data, err := os.ReadFile("option.go")
	require.NoError(t, err)
	s := string(data)
	start := strings.Index(s, "func ensureAdobeSeedanceClassicRouting() error")
	end := strings.Index(s[start:], "func ensureClassicVideoMarketplaceModels")
	require.NotEqual(t, -1, start)
	require.NotEqual(t, -1, end)
	body := s[start : start+end]
	marketplace := strings.Index(body, "ensureClassicVideoMarketplaceModels(publicModels)")
	keyValidation := strings.Index(body, `os.Getenv("ADOBE2API_GATEWAY_KEY")`)
	require.NotEqual(t, -1, marketplace)
	require.NotEqual(t, -1, keyValidation)
	require.Less(t, marketplace, keyValidation, "marketplace registration must not be blocked by a missing gateway key")
}

func TestHiggsSeedanceUniformPrice(t *testing.T) {
	require.Equal(t, 2.5, higgsSeedancePrice)
}

func TestEnsureCSVValueAddsVIP6Once(t *testing.T) {
	require.Equal(t, "default,vip,vip6", ensureCSVValue("default,vip", "vip6"))
	require.Equal(t, "default,vip6", ensureCSVValue("default,vip6", "vip6"))
}
