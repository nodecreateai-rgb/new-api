package model

// Playad Video stays disabled. Its former public id is retired separately.
var playadPublicModels = []string{}

const playadChannelName = "Playad Video"

// ensurePlayad2apiRouting retires the Playad channel on every start.
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
