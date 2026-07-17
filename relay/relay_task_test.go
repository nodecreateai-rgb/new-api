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

func TestShouldApplyTaskBillingRatiosForFixedPriceSeedanceModels(t *testing.T) {
	savedPatches := append([]string(nil), constant.TaskPricePatches...)
	t.Cleanup(func() { constant.TaskPricePatches = savedPatches })
	constant.TaskPricePatches = []string{
		"seedance-2.0-fast-720p", "seedance-2.0-720p", "seedance-2.0-1080p", "seedance-2.0-4k",
	}

	adaptor := &sora.TaskAdaptor{}
	for _, modelName := range constant.TaskPricePatches {
		info := &relaycommon.RelayInfo{OriginModelName: modelName}
		require.False(t, shouldApplyTaskBillingRatios(adaptor, info, modelName), modelName)
	}
}

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

func TestFixedPriceModelDoesNotMultiplyEightYuanByTenSeconds(t *testing.T) {
	adaptor := &sora.TaskAdaptor{}
	info := &relaycommon.RelayInfo{
		OriginModelName: "seedance-2.0-4k",
		PriceData: types.PriceData{
			Quota:       int(8 * common.QuotaPerUnit),
			OtherRatios: map[string]float64{"seconds": 10},
		},
	}
	require.False(t, shouldApplyTaskBillingRatios(adaptor, info, info.OriginModelName))
	require.Equal(t, int(8*common.QuotaPerUnit), info.PriceData.Quota)
}

func TestUnknownAsyncModelDefaultsToPerItem(t *testing.T) {
	adaptor := &sora.TaskAdaptor{}
	info := &relaycommon.RelayInfo{OriginModelName: "future-video-model"}
	require.False(t, shouldApplyTaskBillingRatios(adaptor, info, info.OriginModelName))
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
