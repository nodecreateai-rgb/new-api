package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

var air2apiImageModels = map[string]struct{}{
	"gpt-image-2":            {},
	"gpt-image-2.5-flare":    {},
	"gpt-image-2.5-sunburst": {},
	"nano-banana-2":          {},
	"nano-banana-2-lite":     {},
	"nano-banana-pro":        {},
}

func isAir2APIImageModel(model string) bool {
	_, ok := air2apiImageModels[strings.ToLower(strings.TrimSpace(model))]
	return ok
}

func shouldForceAir2APIAsync(req *dto.ImageRequest) bool {
	return req != nil && isAir2APIImageModel(req.Model)
}

func isAir2APIBaseURL(baseURL string) bool {
	baseURL = strings.ToLower(strings.TrimSpace(baseURL))
	if baseURL == "" {
		return false
	}
	for _, marker := range []string{"air2api", ":38474", "air.dopio.cyou"} {
		if strings.Contains(baseURL, marker) {
			return true
		}
	}
	return false
}
