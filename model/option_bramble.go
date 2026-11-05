package model

// Bramble Video stays disabled. Its former public ids are retired on every start.
var bramblePublicModels = []string{"wan3.0-video-prime", "seedance-2.5"}

const brambleChannelName = "Bramble Video"

// ensureBramble2apiRouting retires the Bramble channel on every start.
func ensureBramble2apiRouting() error {
	if err := disableChannelByName(brambleChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels(bramblePublicModels); err != nil {
		return err
	}
	if err := stripModelsFromChannels(bramblePublicModels); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}
