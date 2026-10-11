package model

const mitoChannelName = "Mito Video"

func retireMito2apiRouting() error {
	if err := disableChannelByName(mitoChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels([]string{"seedance-2.5-480p"}); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}
