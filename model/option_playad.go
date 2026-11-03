package model

// playadPublicModels is withdrawn. seedance-2.0-fast-720p-c1 stays off the
// catalog; startup must not publish it again.
var playadPublicModels = []string{"seedance-2.0-fast-720p-c1"}

const playadChannelName = "Playad Video"

// ensurePlayad2apiRouting retires the Playad listing. The public model is
// removed and the channel is disabled on every start.
func ensurePlayad2apiRouting() error {
	if err := disableChannelByName(playadChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels(playadPublicModels); err != nil {
		return err
	}
	if err := stripModelsFromChannels(playadPublicModels); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}
