package model

var fotor2apiPublicModels = []string{
	"seedance-2.0-c2",
	"seedance-2.0-480p-c2",
	"seedance-2.0-fast-c2",
	"seedance-2.0-fast-480p-c2",
	"seedance-2.0-mini-c2",
	"wan-3.0-c2",
}

const fotor2apiChannelName = "Fotor Video"

func retireFotor2apiRouting() error {
	if err := disableChannelByName(fotor2apiChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels(fotor2apiPublicModels); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}
