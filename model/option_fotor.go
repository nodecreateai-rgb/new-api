package model

var fotorPublicModels = []string{
	"wan-3.0-480p-c2",
}

const fotorChannelName = "Fotor Video"

func retireFotor2apiRouting() error {
	if err := disableChannelByName(fotorChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels(fotorPublicModels); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}
