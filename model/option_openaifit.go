package model

// Video Fast 720p C1 stays disabled. Its former public id is retired on every start.
var openaifitPublicModels = []string{"seedance-2.0-fast-720p-c1"}

const openaifitChannelName = "Video Fast 720p C1"

// ensureOpenaifitRouting retires the Fast 720p C1 channel on every start.
func ensureOpenaifitRouting() error {
	if err := disableChannelByName(openaifitChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels(openaifitPublicModels); err != nil {
		return err
	}
	if err := stripModelsFromChannels(openaifitPublicModels); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}
