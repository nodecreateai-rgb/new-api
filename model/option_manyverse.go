package model

// The 480p video gateway (channel "Seedance 480p Video") is permanently
// retired: its upstream stopped accepting free-tier credits, so every submit
// came back insufficient_balance. On every startup the channel is kept
// manually disabled and its models are withdrawn from the marketplace, so
// nothing can re-publish them.
const seedance480ChannelName = "Seedance 480p Video"

var seedance480PublicModels = []string{"seedance-2.0-480p", "seedance-2.5-480p", "wan-3.0-prime-720p"}

func ensureSeedance480Routing() error {
	if err := disableChannelByName(seedance480ChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels(seedance480PublicModels); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}

func ensureVIP11OnEnabledChannels() error {
	return ensureGroupOnEnabledChannels("vip11")
}
