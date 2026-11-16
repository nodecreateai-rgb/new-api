package model

// Video Seedance C8 stays disabled. Its former public ids are retired on every start.
var seedanceC8PublicModels = []string{
	"seedance-2.0-c8",
	"seedance-2.0-mini-c8",
}

const seedanceC8ChannelName = "Video Seedance C8"

// ensureSeedanceC8Routing retires the C8 channel on every start.
func ensureSeedanceC8Routing() error {
	if err := disableChannelByName(seedanceC8ChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels(seedanceC8PublicModels); err != nil {
		return err
	}
	if err := stripModelsFromChannels(seedanceC8PublicModels); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}
