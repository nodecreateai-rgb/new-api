package model

var flovaPublicModels = []string{
	"wan-3.0-prime",
	"minimax-h3",
	"sora-2",
}

const flovaChannelName = "Flova Video"

func retireFlova2apiRouting() error {
	if err := disableChannelByName(flovaChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels(flovaPublicModels); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}
