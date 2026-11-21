package model

// Video Seedance C7 stays disabled. All former C7 public ids are retired on every start.
var seedanceC7PublicModels = []string{
	"seedance-2.0-mini-480p-c7",
	"seedance-2.0-480p-c7",
	"seedance-2.0-fast-480p-c7",
	"seedance-2.0-mini-720p-c7",
}

const seedanceC7ChannelName = "Video Seedance C7"

// ensureSeedanceC7Routing retires the C7 channel and its models on every start.
func ensureSeedanceC7Routing() error {
	return retireNamedChannelAndModels(seedanceC7ChannelName, seedanceC7PublicModels)
}
