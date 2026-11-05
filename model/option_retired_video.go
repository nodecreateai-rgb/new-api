package model

import "github.com/QuantumNous/new-api/common"

// deadVideoModels no longer work upstream and are withdrawn for good.
var deadVideoModels = []string{
	"minimax-h3", "minimax-h3-max",
	"sd2-c7", "sd2-c6", "sd2.5",
	"seedance-2.0-mini-480p",
	"wan-3.0", "wan-3.0-c1",
	// mindvideo2api (Video 480p C3)
	"wan-3.0-480p-c3", "seedance-2.0-480p-c3", "seedance-2.0-mini-480p-c3",
	"seedance-2.0-fast-480p-c3", "seedance-2.5-c3", "minimax-h3-c3",
	// aqy2api (Video Seedance C5)
	"seedance-2.0-mini-720p", "seedance-2.0-fast-480p-c1", "wan-3.0-480p",
	"seedance-2.0-fast-720p", "seedance-2.0-480p",
}

// deadVideoChannels served only dead models, so they are switched off.
var deadVideoChannels = []string{
	"Dola2API Seedance Video", "Video Omni", "Roboneo Seedance Mini",
	"Flova Video", "Flova Video 2", "Flova Video 3",
	"Flick Video", "Newtake Video", "Playad Video",
	"Video 480p C3", "Video Seedance C5",
}

// retireDeadVideoModels runs after every ensure step, because several of
// those steps publish these models again on each start.
func retireDeadVideoModels() error {
	if err := stripModelsFromChannels(deadVideoModels); err != nil {
		return err
	}
	for _, name := range deadVideoChannels {
		if err := disableChannelByName(name); err != nil {
			return err
		}
	}
	if err := retireMarketplaceModels(deadVideoModels); err != nil {
		return err
	}
	InitChannelCache()
	common.SysLog("retired dead video models and their channels")
	return nil
}
