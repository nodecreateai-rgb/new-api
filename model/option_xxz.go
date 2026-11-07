package model

// Video Fast 720p C2 stays disabled. seedance-2.0-fast-720p-c2 now lives on
// the Video Fast 720p C3 (jm2api) channel, so this seeder must not republish
// the public id or strip it from other channels.
const xxzChannelName = "Video Fast 720p C2"

func ensureXxzRouting() error {
	if err := disableChannelByName(xxzChannelName); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}
