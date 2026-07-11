package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel/task/sora"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestShouldApplyTaskBillingRatiosForSeedancePerSecondAlias(t *testing.T) {
	savedPatches := append([]string(nil), constant.TaskPricePatches...)
	t.Cleanup(func() { constant.TaskPricePatches = savedPatches })
	constant.TaskPricePatches = []string{"seedance-video-fast", "seedance-video-standard"}

	adaptor := &sora.TaskAdaptor{}

	perItem := &relaycommon.RelayInfo{OriginModelName: "seedance-video-fast"}
	require.False(t, shouldApplyTaskBillingRatios(adaptor, perItem, perItem.OriginModelName))

	perSecond := &relaycommon.RelayInfo{OriginModelName: "seedance-video-fast-per-second"}
	require.True(t, shouldApplyTaskBillingRatios(adaptor, perSecond, perSecond.OriginModelName))
}

func TestRecalcQuotaFromRatiosUsesSecondsForSeedancePerSecond(t *testing.T) {
	info := &relaycommon.RelayInfo{
		PriceData: types.PriceData{
			Quota:       int(0.2 * common.QuotaPerUnit),
			OtherRatios: map[string]float64{"seconds": 1},
		},
	}

	quota := recalcQuotaFromRatios(info, map[string]float64{"seconds": 10, "size": 1})
	require.Equal(t, int(0.2*10*common.QuotaPerUnit), quota)
}
