package model

var videoStudioPublicModels = []string{
	"seedance-2.0",
	"sora-2",
	"wan-3.0",
	"minimax-h3",
}

const videoStudioChannelName = "Video Studio"

func retireVideoStudioRouting() error {
	if err := disableChannelByName(videoStudioChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels(videoStudioPublicModels); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}
