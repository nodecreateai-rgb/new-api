package model

import "github.com/QuantumNous/new-api/common"

const vibbitChannelName = "Vibbit Video"

// ensureVibbit2apiRouting used to publish wan-3.0-480p. That public model now
// lives on Spark Video (aiimagetovideo2api). Keep this hook so startup still
// disables the old channel instead of racing ClickHouse ability updates.
func ensureVibbit2apiRouting() error {
	if err := disableChannelByName(vibbitChannelName); err != nil {
		return err
	}
	InitChannelCache()
	common.SysLog("retired Vibbit Video; wan-3.0-480p is served by Spark Video")
	return nil
}
