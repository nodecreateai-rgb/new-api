package helper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}

func TestModelPriceHelperPerCallDopioGroupPrices(t *testing.T) {
	gin.SetMode(gin.TestMode)

	savedModelPrice := ratio_setting.ModelPrice2JSONString()
	savedGroupRatio := ratio_setting.GroupRatio2JSONString()
	savedModelGroupPrice := ratio_setting.ModelGroupPrice2JSONString()
	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrice))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(savedGroupRatio))
		require.NoError(t, ratio_setting.UpdateModelGroupPriceByJSONString(savedModelGroupPrice))
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"sd2-c1":4,"sd2-c2":3,"sd2-c3":4}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"svip":1}`))
	require.NoError(t, ratio_setting.UpdateModelGroupPriceByJSONString(`{"vip":{"sd2-c1":3,"sd2-c2":2,"sd2-c3":3},"svip":{"sd2-c1":2,"sd2-c2":2,"sd2-c3":2}}`))

	cases := []struct {
		group string
		model string
		rmb   float64
	}{
		{"default", "sd2-c1", 4},
		{"default", "sd2-c2", 3},
		{"default", "sd2-c3", 4},
		{"vip", "sd2-c1", 3},
		{"vip", "sd2-c2", 2},
		{"vip", "sd2-c3", 3},
		{"svip", "sd2-c1", 2},
		{"svip", "sd2-c2", 2},
		{"svip", "sd2-c3", 2},
	}
	for _, tc := range cases {
		t.Run(tc.group+"/"+tc.model, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
			info := &relaycommon.RelayInfo{
				OriginModelName: tc.model,
				UserGroup:       tc.group,
				UsingGroup:      tc.group,
			}
			priceData, err := ModelPriceHelperPerCall(ctx, info)
			require.NoError(t, err)
			require.Equal(t, int(tc.rmb*common.QuotaPerUnit), priceData.Quota)
			require.Equal(t, tc.rmb, priceData.ModelPrice)
			require.Equal(t, 1.0, priceData.GroupRatioInfo.GroupRatio)
		})
	}
}
