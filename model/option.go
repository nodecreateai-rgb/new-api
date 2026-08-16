package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/performance_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"gorm.io/gorm"
)

type Option struct {
	Key   string `json:"key" gorm:"primaryKey"`
	Value string `json:"value"`
}

const higgsSeedancePrice = 2.5

func AllOption() ([]*Option, error) {
	var options []*Option
	var err error
	err = DB.Find(&options).Error
	return options, err
}

func InitOptionMap() {
	common.OptionMapRWMutex.Lock()
	common.OptionMap = make(map[string]string)

	// 添加原有的系统配置
	common.OptionMap["FileUploadPermission"] = strconv.Itoa(common.FileUploadPermission)
	common.OptionMap["FileDownloadPermission"] = strconv.Itoa(common.FileDownloadPermission)
	common.OptionMap["ImageUploadPermission"] = strconv.Itoa(common.ImageUploadPermission)
	common.OptionMap["ImageDownloadPermission"] = strconv.Itoa(common.ImageDownloadPermission)
	common.OptionMap["PasswordLoginEnabled"] = strconv.FormatBool(common.PasswordLoginEnabled)
	common.OptionMap["PasswordRegisterEnabled"] = strconv.FormatBool(common.PasswordRegisterEnabled)
	common.OptionMap["EmailVerificationEnabled"] = strconv.FormatBool(common.EmailVerificationEnabled)
	common.OptionMap["GitHubOAuthEnabled"] = strconv.FormatBool(common.GitHubOAuthEnabled)
	common.OptionMap["LinuxDOOAuthEnabled"] = strconv.FormatBool(common.LinuxDOOAuthEnabled)
	common.OptionMap["TelegramOAuthEnabled"] = strconv.FormatBool(common.TelegramOAuthEnabled)
	common.OptionMap["WeChatAuthEnabled"] = strconv.FormatBool(common.WeChatAuthEnabled)
	common.OptionMap["TurnstileCheckEnabled"] = strconv.FormatBool(common.TurnstileCheckEnabled)
	common.OptionMap["RegisterEnabled"] = strconv.FormatBool(common.RegisterEnabled)
	common.OptionMap["AutomaticDisableChannelEnabled"] = strconv.FormatBool(common.AutomaticDisableChannelEnabled)
	common.OptionMap["AutomaticEnableChannelEnabled"] = strconv.FormatBool(common.AutomaticEnableChannelEnabled)
	common.OptionMap["LogConsumeEnabled"] = strconv.FormatBool(common.LogConsumeEnabled)
	common.OptionMap["DisplayInCurrencyEnabled"] = strconv.FormatBool(common.DisplayInCurrencyEnabled)
	common.OptionMap["DisplayTokenStatEnabled"] = strconv.FormatBool(common.DisplayTokenStatEnabled)
	common.OptionMap["DrawingEnabled"] = strconv.FormatBool(common.DrawingEnabled)
	common.OptionMap["TaskEnabled"] = strconv.FormatBool(common.TaskEnabled)
	common.OptionMap["DataExportEnabled"] = strconv.FormatBool(common.DataExportEnabled)
	common.OptionMap["ChannelDisableThreshold"] = strconv.FormatFloat(common.ChannelDisableThreshold, 'f', -1, 64)
	common.OptionMap["EmailDomainRestrictionEnabled"] = strconv.FormatBool(common.EmailDomainRestrictionEnabled)
	common.OptionMap["EmailAliasRestrictionEnabled"] = strconv.FormatBool(common.EmailAliasRestrictionEnabled)
	common.OptionMap["EmailDomainWhitelist"] = strings.Join(common.EmailDomainWhitelist, ",")
	common.OptionMap["SMTPServer"] = ""
	common.OptionMap["SMTPFrom"] = ""
	common.OptionMap["SMTPPort"] = strconv.Itoa(common.SMTPPort)
	common.OptionMap["SMTPAccount"] = ""
	common.OptionMap["SMTPToken"] = ""
	common.OptionMap["SMTPSSLEnabled"] = strconv.FormatBool(common.SMTPSSLEnabled)
	common.OptionMap["SMTPForceAuthLogin"] = strconv.FormatBool(common.SMTPForceAuthLogin)
	common.OptionMap["Notice"] = ""
	common.OptionMap["About"] = ""
	common.OptionMap["HomePageContent"] = ""
	common.OptionMap["Footer"] = common.Footer
	common.OptionMap["SystemName"] = common.SystemName
	common.OptionMap["Logo"] = common.Logo
	common.OptionMap["ServerAddress"] = ""
	common.OptionMap["WorkerUrl"] = system_setting.WorkerUrl
	common.OptionMap["WorkerValidKey"] = system_setting.WorkerValidKey
	common.OptionMap["WorkerAllowHttpImageRequestEnabled"] = strconv.FormatBool(system_setting.WorkerAllowHttpImageRequestEnabled)
	common.OptionMap["PayAddress"] = ""
	common.OptionMap["CustomCallbackAddress"] = ""
	common.OptionMap["EpayId"] = ""
	common.OptionMap["EpayKey"] = ""
	common.OptionMap["Price"] = strconv.FormatFloat(operation_setting.Price, 'f', -1, 64)
	common.OptionMap["USDExchangeRate"] = strconv.FormatFloat(operation_setting.USDExchangeRate, 'f', -1, 64)
	common.OptionMap["MinTopUp"] = strconv.Itoa(operation_setting.MinTopUp)
	common.OptionMap["StripeMinTopUp"] = strconv.Itoa(setting.StripeMinTopUp)
	common.OptionMap["StripeApiSecret"] = setting.StripeApiSecret
	common.OptionMap["StripeWebhookSecret"] = setting.StripeWebhookSecret
	common.OptionMap["StripePriceId"] = setting.StripePriceId
	common.OptionMap["StripeUnitPrice"] = strconv.FormatFloat(setting.StripeUnitPrice, 'f', -1, 64)
	common.OptionMap["StripePromotionCodesEnabled"] = strconv.FormatBool(setting.StripePromotionCodesEnabled)
	common.OptionMap["CreemApiKey"] = setting.CreemApiKey
	common.OptionMap["CreemProducts"] = setting.CreemProducts
	common.OptionMap["CreemTestMode"] = strconv.FormatBool(setting.CreemTestMode)
	common.OptionMap["CreemWebhookSecret"] = setting.CreemWebhookSecret
	common.OptionMap["WaffoEnabled"] = strconv.FormatBool(setting.WaffoEnabled)
	common.OptionMap["WaffoApiKey"] = setting.WaffoApiKey
	common.OptionMap["WaffoPrivateKey"] = setting.WaffoPrivateKey
	common.OptionMap["WaffoPublicCert"] = setting.WaffoPublicCert
	common.OptionMap["WaffoSandboxPublicCert"] = setting.WaffoSandboxPublicCert
	common.OptionMap["WaffoSandboxApiKey"] = setting.WaffoSandboxApiKey
	common.OptionMap["WaffoSandboxPrivateKey"] = setting.WaffoSandboxPrivateKey
	common.OptionMap["WaffoSandbox"] = strconv.FormatBool(setting.WaffoSandbox)
	common.OptionMap["WaffoMerchantId"] = setting.WaffoMerchantId
	common.OptionMap["WaffoNotifyUrl"] = setting.WaffoNotifyUrl
	common.OptionMap["WaffoReturnUrl"] = setting.WaffoReturnUrl
	common.OptionMap["WaffoSubscriptionReturnUrl"] = setting.WaffoSubscriptionReturnUrl
	common.OptionMap["WaffoCurrency"] = setting.WaffoCurrency
	common.OptionMap["WaffoUnitPrice"] = strconv.FormatFloat(setting.WaffoUnitPrice, 'f', -1, 64)
	common.OptionMap["WaffoMinTopUp"] = strconv.Itoa(setting.WaffoMinTopUp)
	common.OptionMap["WaffoPayMethods"] = setting.WaffoPayMethods2JsonString()
	common.OptionMap["WaffoPancakeMerchantID"] = setting.WaffoPancakeMerchantID
	common.OptionMap["WaffoPancakePrivateKey"] = setting.WaffoPancakePrivateKey
	common.OptionMap["WaffoPancakeReturnURL"] = setting.WaffoPancakeReturnURL
	common.OptionMap["WaffoPancakeUnitPrice"] = strconv.FormatFloat(setting.WaffoPancakeUnitPrice, 'f', -1, 64)
	common.OptionMap["WaffoPancakeMinTopUp"] = strconv.Itoa(setting.WaffoPancakeMinTopUp)
	common.OptionMap["WaffoPancakeStoreID"] = setting.WaffoPancakeStoreID
	common.OptionMap["WaffoPancakeProductID"] = setting.WaffoPancakeProductID
	common.OptionMap["TopupGroupRatio"] = common.TopupGroupRatio2JSONString()
	common.OptionMap["Chats"] = setting.Chats2JsonString()
	common.OptionMap["AutoGroups"] = setting.AutoGroups2JsonString()
	common.OptionMap["DefaultUseAutoGroup"] = strconv.FormatBool(setting.DefaultUseAutoGroup)
	common.OptionMap["PayMethods"] = operation_setting.PayMethods2JsonString()
	common.OptionMap["GitHubClientId"] = ""
	common.OptionMap["GitHubClientSecret"] = ""
	common.OptionMap["TelegramBotToken"] = ""
	common.OptionMap["TelegramBotName"] = ""
	common.OptionMap["WeChatServerAddress"] = ""
	common.OptionMap["WeChatServerToken"] = ""
	common.OptionMap["WeChatAccountQRCodeImageURL"] = ""
	common.OptionMap["TurnstileSiteKey"] = ""
	common.OptionMap["TurnstileSecretKey"] = ""
	common.OptionMap["QuotaForNewUser"] = strconv.Itoa(common.QuotaForNewUser)
	common.OptionMap["QuotaForInviter"] = strconv.Itoa(common.QuotaForInviter)
	common.OptionMap["QuotaForInvitee"] = strconv.Itoa(common.QuotaForInvitee)
	common.OptionMap["QuotaRemindThreshold"] = strconv.Itoa(common.QuotaRemindThreshold)
	common.OptionMap["PreConsumedQuota"] = strconv.Itoa(common.PreConsumedQuota)
	common.OptionMap["ModelRequestRateLimitCount"] = strconv.Itoa(setting.ModelRequestRateLimitCount)
	common.OptionMap["ModelRequestRateLimitDurationMinutes"] = strconv.Itoa(setting.ModelRequestRateLimitDurationMinutes)
	common.OptionMap["ModelRequestRateLimitSuccessCount"] = strconv.Itoa(setting.ModelRequestRateLimitSuccessCount)
	common.OptionMap["ModelRequestRateLimitGroup"] = setting.ModelRequestRateLimitGroup2JSONString()
	common.OptionMap["ModelRatio"] = ratio_setting.ModelRatio2JSONString()
	common.OptionMap["ModelPrice"] = ratio_setting.ModelPrice2JSONString()
	common.OptionMap["ModelGroupPrice"] = ratio_setting.ModelGroupPrice2JSONString()
	common.OptionMap["CacheRatio"] = ratio_setting.CacheRatio2JSONString()
	common.OptionMap["CreateCacheRatio"] = ratio_setting.CreateCacheRatio2JSONString()
	common.OptionMap["GroupRatio"] = ratio_setting.GroupRatio2JSONString()
	common.OptionMap["GroupGroupRatio"] = ratio_setting.GroupGroupRatio2JSONString()
	common.OptionMap["UserUsableGroups"] = setting.UserUsableGroups2JSONString()
	common.OptionMap["CompletionRatio"] = ratio_setting.CompletionRatio2JSONString()
	common.OptionMap["ImageRatio"] = ratio_setting.ImageRatio2JSONString()
	common.OptionMap["AudioRatio"] = ratio_setting.AudioRatio2JSONString()
	common.OptionMap["AudioCompletionRatio"] = ratio_setting.AudioCompletionRatio2JSONString()
	common.OptionMap["TopUpLink"] = common.TopUpLink
	//common.OptionMap["ChatLink"] = common.ChatLink
	//common.OptionMap["ChatLink2"] = common.ChatLink2
	common.OptionMap["QuotaPerUnit"] = strconv.FormatFloat(common.QuotaPerUnit, 'f', -1, 64)
	common.OptionMap["RetryTimes"] = strconv.Itoa(common.RetryTimes)
	common.OptionMap["DataExportInterval"] = strconv.Itoa(common.DataExportInterval)
	common.OptionMap["DataExportDefaultTime"] = common.DataExportDefaultTime
	common.OptionMap["DefaultCollapseSidebar"] = strconv.FormatBool(common.DefaultCollapseSidebar)
	common.OptionMap["MjNotifyEnabled"] = strconv.FormatBool(setting.MjNotifyEnabled)
	common.OptionMap["MjAccountFilterEnabled"] = strconv.FormatBool(setting.MjAccountFilterEnabled)
	common.OptionMap["MjModeClearEnabled"] = strconv.FormatBool(setting.MjModeClearEnabled)
	common.OptionMap["MjForwardUrlEnabled"] = strconv.FormatBool(setting.MjForwardUrlEnabled)
	common.OptionMap["MjActionCheckSuccessEnabled"] = strconv.FormatBool(setting.MjActionCheckSuccessEnabled)
	common.OptionMap["CheckSensitiveEnabled"] = strconv.FormatBool(setting.CheckSensitiveEnabled)
	common.OptionMap["DemoSiteEnabled"] = strconv.FormatBool(operation_setting.DemoSiteEnabled)
	common.OptionMap["SelfUseModeEnabled"] = strconv.FormatBool(operation_setting.SelfUseModeEnabled)
	common.OptionMap["ModelRequestRateLimitEnabled"] = strconv.FormatBool(setting.ModelRequestRateLimitEnabled)
	common.OptionMap["CheckSensitiveOnPromptEnabled"] = strconv.FormatBool(setting.CheckSensitiveOnPromptEnabled)
	common.OptionMap["StopOnSensitiveEnabled"] = strconv.FormatBool(setting.StopOnSensitiveEnabled)
	common.OptionMap["SensitiveWords"] = setting.SensitiveWordsToString()
	common.OptionMap["StreamCacheQueueLength"] = strconv.Itoa(setting.StreamCacheQueueLength)
	common.OptionMap["AutomaticDisableKeywords"] = operation_setting.AutomaticDisableKeywordsToString()
	common.OptionMap["AutomaticDisableStatusCodes"] = operation_setting.AutomaticDisableStatusCodesToString()
	common.OptionMap["AutomaticRetryStatusCodes"] = operation_setting.AutomaticRetryStatusCodesToString()
	common.OptionMap["ExposeRatioEnabled"] = strconv.FormatBool(ratio_setting.IsExposeRatioEnabled())

	// 自动添加所有注册的模型配置
	modelConfigs := config.GlobalConfig.ExportAllConfigs()
	for k, v := range modelConfigs {
		common.OptionMap[k] = v
	}

	common.OptionMapRWMutex.Unlock()
	loadOptionsFromDatabase()
	ensureDopioRMBPricing()
}

func loadOptionsFromDatabase() {
	options, _ := AllOption()
	for _, option := range options {
		err := updateOptionMap(option.Key, option.Value)
		if err != nil {
			common.SysLog("failed to update option map: " + err.Error())
		}
	}
}

// ensureDopioRMBPricing keeps the Dopio Seedance aliases billed directly in RMB.
// Historically these prices were stored as USD amounts converted from RMB with
// USDExchangeRate≈7.3. Dopio now treats account balance/prices as RMB 1:1, so
// startup normalizes both the exchange rate and these per-call model prices.
func ensureDopioRMBPricing() {
	const targetPrice = "1"
	const targetUSDExchangeRate = "1"
	const targetQuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	targetModelPrices := map[string]float64{
		"sd2-c1":                             4,
		"sd2-c2":                             3,
		"sd2-c3":                             4,
		"sd2-c5":                             5,
		"sd2-c6":                             0.5,
		"sd2-c7":                             1,
		"sd2-mini":                           0.6,
		"sd2-fast":                           1,
		"sd2.5":                              1.5,
		"sd2-c8":                             5,
		"sd2-c9":                             2,
		"sd2-c10":                            4,
		"sd2-c11":                            2.5,
		"sd2-c12":                            3,
		"seedance-720":                       higgsSeedancePrice,
		"seedance-2.0-720p":                  3,
		"seedance-2.0-fast-720p":             2,
		"seedance-2.0-1080p":                 4,
		"seedance-video-fast-per-second":     0.2,
		"seedance-video-standard-per-second": 0.33,
		"wan2.7-720p":                        0.5,
		"wan2.7-1080p":                       1,
		"viduq3-turbo-720p":                  0.3,
		"viduq3-turbo-1080p":                 0.5,
		"viduq3-pro-1080p":                   1,
		"happyhorse-1.1-720p":                0.5,
		"happyhorse-1.1-1080p":               1,
		"sora-2-720p":                        0.5,
		"sora-2-1080p":                       1,
		"sora-2-pro-720p":                    5,
		"sora-2-pro-1080p":                   8,
		"kling-v3-720p":                      0.5,
		"kling-v3-1080p":                     1,
		"kling-v3-4k":                        2,
		"kling-3-pro":                        1,
		"kling-o3":                           1,
		"gpt-image-2":                        0.03,
		"nano-banana-2":                      0.01,
		"nano-banana-pro":                    0.01,
	}
	targetGroupRatios := map[string]float64{
		"default": 1,
		"vip1":    1,
		"vip":     1,
		"svip":    1,
		"vip2":    1,
		"vip6":    1,
	}
	targetUserUsableGroups := map[string]string{
		"vip6": "VIP6分组",
	}
	// Fixed per-group RMB prices for Dopio video aliases. ModelGroupPrice
	// overrides ModelPrice and resets group ratio to 1 during billing, which is
	// required for absolute discounts (vip is base price minus ¥1; svip is ¥2 for
	// every sd2 model) rather than multiplicative discounts such as 0.75×.
	targetModelGroupPrices := map[string]map[string]float64{
		"default": {
			"seedance-video-fast":                3,
			"seedance-video-standard":            5,
			"seedance-2.0-720p":                  3,
			"seedance-2.0-fast-720p":             2,
			"seedance-2.0-1080p":                 4,
			"seedance-video-fast-per-second":     0.2,
			"seedance-video-standard-per-second": 0.33,
			"wan2.7-720p":                        0.5, "wan2.7-1080p": 1,
			"viduq3-turbo-720p": 0.3, "viduq3-turbo-1080p": 0.5, "viduq3-pro-1080p": 1,
			"happyhorse-1.1-720p": 0.5, "happyhorse-1.1-1080p": 1,
			"sora-2-720p": 0.5, "sora-2-1080p": 1, "sora-2-pro-720p": 5, "sora-2-pro-1080p": 8,
			"kling-v3-720p": 0.5, "kling-v3-1080p": 1, "kling-v3-4k": 2,
			"sd2-c6":          0.5,
			"sd2-c7":          1,
			"gpt-image-2":     0.03,
			"nano-banana-2":   0.01,
			"nano-banana-pro": 0.01,
		},
		"vip1": {
			"seedance-video-fast":                3,
			"seedance-video-standard":            5,
			"seedance-2.0-720p":                  3,
			"seedance-2.0-fast-720p":             2,
			"seedance-2.0-1080p":                 4,
			"seedance-video-fast-per-second":     0.2,
			"seedance-video-standard-per-second": 0.33,
			"sd2-c5":                             3,
			"sd2-c6":                             0.5,
			"sd2-c7":                             1,
			"sd2-c8":                             3,
			"sd2-c9":                             1,
			"sd2-c10":                            0.5,
			"gpt-image-2":                        0.03,
			"nano-banana-2":                      0.01,
			"nano-banana-pro":                    0.01,
		},
		"vip": {
			"seedance-video-fast":                3,
			"seedance-video-standard":            5,
			"seedance-2.0-720p":                  3,
			"seedance-2.0-fast-720p":             2,
			"seedance-2.0-1080p":                 4,
			"seedance-video-fast-per-second":     0.2,
			"seedance-video-standard-per-second": 0.33,
			"sd2-c1":                             3,
			"sd2-c2":                             2,
			"sd2-c3":                             3,
			"sd2-c5":                             4,
			"sd2-c6":                             0.5,
			"sd2-c7":                             1,
			"gpt-image-2":                        0.03,
			"nano-banana-2":                      0.01,
			"nano-banana-pro":                    0.01,
		},
		"svip": {
			"seedance-video-fast":                3,
			"seedance-video-standard":            5,
			"seedance-2.0-720p":                  3,
			"seedance-2.0-fast-720p":             2,
			"seedance-2.0-1080p":                 4,
			"seedance-video-fast-per-second":     0.2,
			"seedance-video-standard-per-second": 0.33,
			"sd2-c1":                             2,
			"sd2-c2":                             2,
			"sd2-c3":                             2,
			"sd2-c5":                             2,
			"sd2-c6":                             0.5,
			"sd2-c7":                             1,
			"gpt-image-2":                        0.03,
			"nano-banana-2":                      0.01,
			"nano-banana-pro":                    0.01,
		},
		"vip2": {
			"seedance-video-fast":                3,
			"seedance-video-standard":            5,
			"seedance-2.0-720p":                  3,
			"seedance-2.0-fast-720p":             2,
			"seedance-2.0-1080p":                 4,
			"seedance-video-fast-per-second":     0.2,
			"seedance-video-standard-per-second": 0.33,
			"wan2.7-720p":                        0.5, "wan2.7-1080p": 1,
			"viduq3-turbo-720p": 0.3, "viduq3-turbo-1080p": 0.5, "viduq3-pro-1080p": 1,
			"happyhorse-1.1-720p": 0.5, "happyhorse-1.1-1080p": 1,
			"sora-2-720p": 0.5, "sora-2-1080p": 1, "sora-2-pro-720p": 5, "sora-2-pro-1080p": 8,
			"kling-v3-720p": 0.5, "kling-v3-1080p": 1, "kling-v3-4k": 2,
			"sd2-c6":          0.3,
			"sd2-c7":          1,
			"sd2-c11":         1.5,
			"sd2-c12":         2,
			"gpt-image-2":     0.03,
			"nano-banana-2":   0.01,
			"nano-banana-pro": 0.01,
		},
		"vip3": {
			"seedance-video-fast":                3,
			"seedance-video-standard":            5,
			"seedance-2.0-720p":                  3,
			"seedance-2.0-fast-720p":             2,
			"seedance-2.0-1080p":                 4,
			"seedance-video-fast-per-second":     0.2,
			"seedance-video-standard-per-second": 0.33,
		},
		"vip6": {
			"seedance-2.0-fast-720p": 1,
			"seedance-2.0-720p":      2,
			"sd2.5":                  1,
		},
	}
	for group := range targetModelGroupPrices {
		targetModelGroupPrices[group]["seedance-720"] = higgsSeedancePrice
		// sd2-c7 is a fixed ¥1 per-call alias in every actual user group.
		targetModelGroupPrices[group]["sd2-c7"] = 1
	}

	updates := map[string]string{}
	common.OptionMapRWMutex.RLock()
	currentPrice := common.OptionMap["Price"]
	currentRate := common.OptionMap["USDExchangeRate"]
	currentQuotaDisplayType := common.OptionMap["general_setting.quota_display_type"]
	currentModelPrice := common.OptionMap["ModelPrice"]
	currentGroupRatio := common.OptionMap["GroupRatio"]
	currentModelGroupPrice := common.OptionMap["ModelGroupPrice"]
	currentUserUsableGroups := common.OptionMap["UserUsableGroups"]
	common.OptionMapRWMutex.RUnlock()

	if currentPrice != targetPrice {
		updates["Price"] = targetPrice
	}
	if currentRate != targetUSDExchangeRate {
		updates["USDExchangeRate"] = targetUSDExchangeRate
	}
	if currentQuotaDisplayType != targetQuotaDisplayType {
		updates["general_setting.quota_display_type"] = targetQuotaDisplayType
	}

	prices := map[string]float64{}
	if currentModelPrice != "" {
		if err := json.Unmarshal([]byte(currentModelPrice), &prices); err != nil {
			common.SysLog("failed to parse ModelPrice while enforcing Dopio RMB pricing: " + err.Error())
			prices = map[string]float64{}
		}
	}
	changed := false
	for _, staleModel := range []string{"happy-horse-1.1", "happyhorse-1.1", "kling-v3", "wan2.7", "viduq3", "seedance-video-fast", "seedance-video-standard", "seedance-video-fast-per-second", "seedance-video-standard-per-second"} {
		if _, exists := prices[staleModel]; exists {
			delete(prices, staleModel)
			changed = true
		}
	}
	if _, exists := prices["seedance-2.0-4k"]; exists {
		delete(prices, "seedance-2.0-4k")
		changed = true
	}
	for model, price := range targetModelPrices {
		if prices[model] != price {
			prices[model] = price
			changed = true
		}
	}
	if changed {
		if b, err := json.Marshal(prices); err == nil {
			updates["ModelPrice"] = string(b)
		} else {
			common.SysLog("failed to marshal ModelPrice while enforcing Dopio RMB pricing: " + err.Error())
		}
	}

	groupRatios := map[string]float64{}
	if currentGroupRatio != "" {
		if err := json.Unmarshal([]byte(currentGroupRatio), &groupRatios); err != nil {
			common.SysLog("failed to parse GroupRatio while enforcing Dopio RMB pricing: " + err.Error())
			groupRatios = map[string]float64{}
		}
	}
	changed = false
	for group, ratio := range targetGroupRatios {
		if groupRatios[group] != ratio {
			groupRatios[group] = ratio
			changed = true
		}
	}
	if changed {
		if b, err := json.Marshal(groupRatios); err == nil {
			updates["GroupRatio"] = string(b)
		} else {
			common.SysLog("failed to marshal GroupRatio while enforcing Dopio RMB pricing: " + err.Error())
		}
	}

	userUsableGroups := map[string]string{}
	if currentUserUsableGroups != "" {
		if err := json.Unmarshal([]byte(currentUserUsableGroups), &userUsableGroups); err != nil {
			common.SysLog("failed to parse UserUsableGroups while enforcing Dopio RMB pricing: " + err.Error())
			userUsableGroups = map[string]string{}
		}
	}
	changed = false
	for group, description := range targetUserUsableGroups {
		if userUsableGroups[group] != description {
			userUsableGroups[group] = description
			changed = true
		}
	}
	if changed {
		if b, err := json.Marshal(userUsableGroups); err == nil {
			updates["UserUsableGroups"] = string(b)
		} else {
			common.SysLog("failed to marshal UserUsableGroups while enforcing Dopio RMB pricing: " + err.Error())
		}
	}

	modelGroupPrices := map[string]map[string]float64{}
	if currentModelGroupPrice != "" {
		if err := json.Unmarshal([]byte(currentModelGroupPrice), &modelGroupPrices); err != nil {
			common.SysLog("failed to parse ModelGroupPrice while enforcing Dopio RMB pricing: " + err.Error())
			modelGroupPrices = map[string]map[string]float64{}
		}
	}
	changed = false
	for group, groupPrices := range modelGroupPrices {
		for _, staleModel := range []string{"happy-horse-1.1", "happyhorse-1.1", "kling-v3", "wan2.7", "viduq3", "seedance-video-fast", "seedance-video-standard", "seedance-video-fast-per-second", "seedance-video-standard-per-second"} {
			if _, exists := groupPrices[staleModel]; exists {
				delete(groupPrices, staleModel)
				changed = true
			}
		}
		if _, exists := groupPrices["seedance-2.0-4k"]; exists {
			delete(groupPrices, "seedance-2.0-4k")
			changed = true
		}
		for model, price := range targetModelPrices {
			if groupPrices[model] != price {
				groupPrices[model] = price
				changed = true
			}
		}
		// sd2-mini and sd2-fast are fixed per request in every group.
		if groupPrices["sd2-mini"] != 0.6 {
			groupPrices["sd2-mini"] = 0.6
			changed = true
		}
		if groupPrices["sd2-fast"] != 1 {
			groupPrices["sd2-fast"] = 1
			changed = true
		}
		modelGroupPrices[group] = groupPrices
	}
	for group, targetPrices := range targetModelGroupPrices {
		if modelGroupPrices[group] == nil {
			modelGroupPrices[group] = map[string]float64{}
		}
		for model, price := range targetPrices {
			if modelGroupPrices[group][model] != price {
				modelGroupPrices[group][model] = price
				changed = true
			}
		}
	}
	if changed {
		if b, err := json.Marshal(modelGroupPrices); err == nil {
			updates["ModelGroupPrice"] = string(b)
		} else {
			common.SysLog("failed to marshal ModelGroupPrice while enforcing Dopio RMB pricing: " + err.Error())
		}
	}

	if len(updates) > 0 {
		if err := UpdateOptionsBulk(updates); err != nil {
			common.SysLog("failed to enforce Dopio RMB pricing: " + err.Error())
			return
		}
	}
	if err := ensureChannelGroupAbilities(15, "vip6"); err != nil {
		common.SysLog("failed to ensure vip6 channel abilities: " + err.Error())
		return
	}
	if err := ensureSeedance720HiggsRouting(); err != nil {
		common.SysLog("failed to enforce Seedance 720 gateway routing: " + err.Error())
		return
	}
	if err := ensureAdobeSeedanceClassicRouting(); err != nil {
		common.SysLog("failed to enforce classic Seedance gateway routing: " + err.Error())
		return
	}
	if err := ensureDolaSeedanceRouting(); err != nil {
		common.SysLog("failed to enforce Dola Seedance gateway routing: " + err.Error())
		return
	}
	if err := ensureSD2FastRouting(); err != nil {
		common.SysLog("failed to enforce sd2-fast gateway routing: " + err.Error())
		return
	}
	if err := ensureSD25Routing(); err != nil {
		common.SysLog("failed to enforce sd2.5 gateway routing: " + err.Error())
		return
	}
	common.SysLog("enforced Dopio RMB pricing incl sd2.5=1.5 per call, vip6 sd2.5=1, sd2-fast=1 per call, vip6 Seedance 720p fast=1/full=2, banana=0.01, sd2-c6=1, sd2-c7=1, sd2-c11=2.5, sd2-c12=3, Price=1, USDExchangeRate=1, quota_display_type=CNY")
}

func ensureDolaSeedanceRouting() error {
	const channelID = 2
	baseURL := strings.TrimSpace(os.Getenv("DOLA2API_BASE_URL"))
	if baseURL == "" {
		baseURL = "http://dola2api:38472"
	}
	return DB.Model(&Channel{}).Where("id = ?", channelID).Update("base_url", baseURL).Error
}

func ensureSD2FastRouting() error {
	// Both services persist membership in the same overlay network. Use the
	// neutral internal alias so upstream restarts do not depend on host-published
	// ports, NAT/hairpin routing, or a fixed manager address.
	const publicModel = "sd2-fast"
	const upstreamModel = "seedance-2.0"
	const neutralName = "Video Fast"
	const groups = "default,vip,svip,vip1,vip2,vip3,vip6"
	baseURL := strings.TrimSpace(os.Getenv("SD2_FAST_BASE_URL"))
	if baseURL == "" {
		// Traecn2API is a manager-owned Compose container while New-API runs on
		// another Swarm node. Docker does not advertise that container alias to
		// the remote worker, so use the worker-verified published route by default.
		baseURL = "http://187.124.94.7:38474"
	}
	key := strings.TrimSpace(os.Getenv("SD2_FAST_GATEWAY_KEY"))
	if key == "" {
		if keyFile := strings.TrimSpace(os.Getenv("SD2_FAST_GATEWAY_KEY_FILE")); keyFile != "" {
			if raw, err := os.ReadFile(keyFile); err == nil {
				key = strings.TrimSpace(string(raw))
			}
		}
	}
	if key == "" {
		key = strings.TrimSpace(os.Getenv("ADOBE2API_GATEWAY_KEY"))
	}
	var channel Channel
	err := DB.Where("name = ?", neutralName).First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if key == "" {
			return fmt.Errorf("SD2_FAST_GATEWAY_KEY is required")
		}
		weight := uint(100)
		priority := int64(10)
		autoBan := 0
		mapping := fmt.Sprintf(`{"%s":"%s"}`, publicModel, upstreamModel)
		channel = Channel{
			Type: 1, Key: key, Status: common.ChannelStatusEnabled, Name: neutralName,
			Weight: &weight, CreatedTime: common.GetTimestamp(), BaseURL: stringPtr(baseURL),
			Models: publicModel, Group: groups, ModelMapping: &mapping,
			Priority: &priority, AutoBan: &autoBan,
		}
		if err := DB.Create(&channel).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if key == "" {
			key = channel.Key
		}
		if key == "" {
			return fmt.Errorf("SD2_FAST_GATEWAY_KEY is required")
		}
		mapping := fmt.Sprintf(`{"%s":"%s"}`, publicModel, upstreamModel)
		if err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
			"type": 1, "key": key, "status": common.ChannelStatusEnabled, "name": neutralName,
			"base_url": baseURL, "models": publicModel, "group": groups, "model_mapping": mapping,
			"priority": 10, "weight": 100, "auto_ban": 0,
		}).Error; err != nil {
			return err
		}
	}

	if err := DB.Model(&Ability{}).Where("channel_id = ? AND model <> ?", channel.Id, publicModel).Update("enabled", false).Error; err != nil {
		return err
	}
	for _, group := range strings.Split(groups, ",") {
		ability := Ability{Group: group, Model: publicModel, ChannelId: channel.Id}
		if err := DB.Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, publicModel, channel.Id).FirstOrCreate(&ability).Error; err != nil {
			return err
		}
		if err := DB.Model(&Ability{}).Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, publicModel, channel.Id).
			Updates(map[string]any{"enabled": true, "priority": int64(10), "weight": uint(100)}).Error; err != nil {
			return err
		}
	}

	endpoint := `{"openai-video":{"path":"/v1/videos","method":"POST"}}`
	var meta Model
	err = DB.Unscoped().Where("model_name = ?", publicModel).First(&meta).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		meta = Model{ModelName: publicModel, Description: "", Icon: "", Tags: "video", Endpoints: endpoint, Status: 1, SyncOfficial: 0, CreatedTime: common.GetTimestamp(), UpdatedTime: common.GetTimestamp()}
		return DB.Create(&meta).Error
	}
	if err != nil {
		return err
	}
	return DB.Unscoped().Model(&Model{}).Where("id = ?", meta.Id).Updates(map[string]any{
		"description": "", "icon": "", "tags": "video", "endpoints": endpoint,
		"status": 1, "sync_official": 0, "deleted_at": nil, "updated_time": common.GetTimestamp(),
	}).Error
}

func ensureSD25Routing() error {
	const publicModel = "sd2.5"
	const upstreamModel = "seedance2.5-c1"
	const neutralName = "Video Omni"
	const groups = "default,vip,svip,vip1,vip2,vip3,vip6"
	baseURL := strings.TrimSpace(os.Getenv("SD25_BASE_URL"))
	if baseURL == "" {
		baseURL = "http://dola2api:38472"
	}
	key := strings.TrimSpace(os.Getenv("SD25_GATEWAY_KEY"))
	if key == "" {
		if keyFile := strings.TrimSpace(os.Getenv("SD25_GATEWAY_KEY_FILE")); keyFile != "" {
			if raw, err := os.ReadFile(keyFile); err == nil {
				key = strings.TrimSpace(string(raw))
			}
		}
	}
	if key == "" {
		key = strings.TrimSpace(os.Getenv("ADOBE2API_GATEWAY_KEY"))
	}
	var channel Channel
	err := DB.Where("name = ?", neutralName).First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if key == "" {
			return fmt.Errorf("SD25_GATEWAY_KEY is required")
		}
		weight := uint(100)
		priority := int64(10)
		autoBan := 0
		mapping := fmt.Sprintf(`{"%s":"%s"}`, publicModel, upstreamModel)
		channel = Channel{Type: 1, Key: key, Status: common.ChannelStatusEnabled, Name: neutralName, Weight: &weight,
			CreatedTime: common.GetTimestamp(), BaseURL: stringPtr(baseURL), Models: publicModel, Group: groups,
			ModelMapping: &mapping, Priority: &priority, AutoBan: &autoBan}
		if err := DB.Create(&channel).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if key == "" {
			key = channel.Key
		}
		if key == "" {
			return fmt.Errorf("SD25_GATEWAY_KEY is required")
		}
		mapping := fmt.Sprintf(`{"%s":"%s"}`, publicModel, upstreamModel)
		if err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
			"type": 1, "key": key, "status": common.ChannelStatusEnabled, "name": neutralName, "base_url": baseURL,
			"models": publicModel, "group": groups, "model_mapping": mapping, "priority": 10, "weight": 100, "auto_ban": 0,
		}).Error; err != nil {
			return err
		}
	}
	if err := DB.Model(&Ability{}).Where("channel_id = ? AND model <> ?", channel.Id, publicModel).Update("enabled", false).Error; err != nil {
		return err
	}
	// sd2.5 is exclusively owned by this Dola-backed channel. Disable stale
	// abilities on any previous provider so no group can route elsewhere.
	if err := DB.Model(&Ability{}).Where("model = ? AND channel_id <> ?", publicModel, channel.Id).Update("enabled", false).Error; err != nil {
		return err
	}
	for _, group := range strings.Split(groups, ",") {
		ability := Ability{Group: group, Model: publicModel, ChannelId: channel.Id}
		if err := DB.Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, publicModel, channel.Id).FirstOrCreate(&ability).Error; err != nil {
			return err
		}
		if err := DB.Model(&Ability{}).Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, publicModel, channel.Id).Updates(map[string]any{"enabled": true, "priority": int64(10), "weight": uint(100)}).Error; err != nil {
			return err
		}
	}
	endpoint := `{"openai-video":{"path":"/v1/videos","method":"POST"}}`
	var meta Model
	err = DB.Unscoped().Where("model_name = ?", publicModel).First(&meta).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		meta = Model{ModelName: publicModel, Description: "", Icon: "", Tags: "video", Endpoints: endpoint, Status: 1, SyncOfficial: 0, CreatedTime: common.GetTimestamp(), UpdatedTime: common.GetTimestamp()}
		return DB.Create(&meta).Error
	}
	if err != nil {
		return err
	}
	return DB.Unscoped().Model(&Model{}).Where("id = ?", meta.Id).Updates(map[string]any{"description": "", "icon": "", "tags": "video", "endpoints": endpoint, "status": 1, "sync_official": 0, "deleted_at": nil, "updated_time": common.GetTimestamp()}).Error
}

func stringPtr(value string) *string { return &value }

func ensureSeedance720HiggsRouting() error {
	const channelID = 15
	const baseURL = "http://video-seedance-hub:38473"
	const publicModel = "seedance-720"

	var channel Channel
	if err := DB.First(&channel, channelID).Error; err != nil {
		return err
	}
	mapping := map[string]string{
		publicModel: "seedance-2.0",
	}
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name":          "Video Model 720p",
		"status":        common.ChannelStatusEnabled,
		"base_url":      baseURL,
		"models":        publicModel,
		"model_mapping": string(mappingJSON),
		"priority":      10,
		"weight":        100,
	}
	if err := DB.Model(&Channel{}).Where("id = ?", channelID).Updates(updates).Error; err != nil {
		return err
	}
	if err := DB.Model(&Ability{}).Where("channel_id = ? AND model <> ?", channelID, publicModel).Update("enabled", false).Error; err != nil {
		return err
	}
	for _, group := range strings.Split(channel.Group, ",") {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		ability := Ability{Group: group, Model: publicModel, ChannelId: channelID}
		if err := DB.Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, publicModel, channelID).
			FirstOrCreate(&ability).Error; err != nil {
			return err
		}
		priority := int64(10)
		if err := DB.Model(&Ability{}).
			Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, publicModel, channelID).
			Updates(map[string]any{"enabled": true, "priority": &priority, "weight": uint(100)}).Error; err != nil {
			return err
		}
	}
	return nil
}

var seedanceClassicPublicModels = []string{"seedance-2.0-fast-720p", "seedance-2.0-720p", "kling-o3"}

func ensureCSVValue(csv, value string) string {
	value = strings.TrimSpace(value)
	parts := make([]string, 0)
	seen := false
	for _, part := range strings.Split(csv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == value {
			seen = true
		}
		parts = append(parts, part)
	}
	if !seen && value != "" {
		parts = append(parts, value)
	}
	return strings.Join(parts, ",")
}

func ensureAdobeSeedanceClassicRouting() error {
	const channelName = "Generation Service"
	const legacyChannelID = 14
	const defaultGroups = "default,vip,svip,vip1,vip2,vip3,vip6"
	baseURL := strings.TrimSpace(os.Getenv("ADOBE_SEEDANCE_CLASSIC_BASE_URL"))
	if baseURL == "" {
		baseURL = "http://adobe2api:39918"
	}
	publicModels := seedanceClassicPublicModels
	mapping := map[string]string{
		"seedance-2.0-fast-720p": "seedance-2.0-fast",
		"seedance-2.0-720p":      "seedance-2.0",
		"kling-o3":               "kling-o3",
	}
	mappingJSON, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	// Marketplace registration is database-only and must not depend on the
	// upstream gateway credential. Dokploy may temporarily start a cluster node
	// without injected secrets; the public catalog should still expose supported
	// models while routing reports its own configuration error separately.
	if err := ensureClassicVideoMarketplaceModels(publicModels); err != nil {
		return err
	}
	apiKey := strings.TrimSpace(os.Getenv("ADOBE2API_GATEWAY_KEY"))
	if apiKey == "" {
		return fmt.Errorf("ADOBE2API_GATEWAY_KEY is empty")
	}

	var channel Channel
	err = DB.Where("name = ?", channelName).First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = DB.First(&channel, legacyChannelID).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		weight := uint(100)
		priority := int64(10)
		autoBan := 0
		channel = Channel{
			Type:         1,
			Key:          apiKey,
			Status:       common.ChannelStatusEnabled,
			Name:         channelName,
			Weight:       &weight,
			CreatedTime:  common.GetTimestamp(),
			BaseURL:      stringPtr(baseURL),
			Models:       strings.Join(publicModels, ","),
			Group:        defaultGroups,
			ModelMapping: stringPtr(string(mappingJSON)),
			Priority:     &priority,
			AutoBan:      &autoBan,
		}
		if err := DB.Create(&channel).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	channelID := channel.Id
	groups := ensureCSVValue(channel.Group, "vip6")
	if strings.TrimSpace(groups) == "" {
		groups = defaultGroups
	}
	if err := DB.Model(&Channel{}).Where("id = ?", channelID).Updates(map[string]any{
		"name":          channelName,
		"status":        common.ChannelStatusEnabled,
		"base_url":      baseURL,
		"key":           apiKey,
		"models":        strings.Join(publicModels, ","),
		"model_mapping": string(mappingJSON),
		"group":         groups,
		"priority":      10,
		"weight":        100,
	}).Error; err != nil {
		return err
	}
	if err := DB.Model(&Ability{}).Where("channel_id = ? AND model NOT IN ?", channelID, publicModels).Update("enabled", false).Error; err != nil {
		return err
	}
	for _, group := range strings.Split(groups, ",") {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		for _, publicModel := range publicModels {
			ability := Ability{Group: group, Model: publicModel, ChannelId: channelID}
			if err := DB.Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, publicModel, channelID).
				FirstOrCreate(&ability).Error; err != nil {
				return err
			}
			priority := int64(10)
			if err := DB.Model(&Ability{}).
				Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, publicModel, channelID).
				Updates(map[string]any{"enabled": true, "priority": &priority, "weight": uint(100)}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureClassicVideoMarketplaceModels(publicModels []string) error {
	endpointJSON := `{"openai-video":{"path":"/v1/videos","method":"POST"}}`
	for _, publicModel := range publicModels {
		var marketplaceModel Model
		err := DB.Where("model_name = ? AND deleted_at IS NULL", publicModel).Order("id DESC").First(&marketplaceModel).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			marketplaceModel = Model{
				ModelName:    publicModel,
				Tags:         "video",
				Status:       1,
				SyncOfficial: 0,
				Endpoints:    endpointJSON,
			}
			if err := marketplaceModel.Insert(); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if err := DB.Model(&marketplaceModel).Updates(map[string]any{
			"status":        1,
			"tags":          "video",
			"endpoints":     endpointJSON,
			"sync_official": 0,
			"updated_time":  common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureChannelGroupAbilities(channelID int, group string) error {
	var channel Channel
	if err := DB.First(&channel, channelID).Error; err != nil {
		return err
	}
	groups := strings.Split(channel.Group, ",")
	found := false
	for _, existingGroup := range groups {
		if strings.TrimSpace(existingGroup) == group {
			found = true
			break
		}
	}
	if !found {
		groups = append(groups, group)
		channel.Group = strings.Join(groups, ",")
		if err := DB.Model(&Channel{}).Where("id = ?", channelID).Update("group", channel.Group).Error; err != nil {
			return err
		}
	}
	return channel.AddAbilities(nil)
}

func SyncOptions(frequency int) {
	for {
		time.Sleep(time.Duration(frequency) * time.Second)
		common.SysLog("syncing options from database")
		loadOptionsFromDatabase()
	}
}

func UpdateOption(key string, value string) error {
	// Save to database first
	option := Option{
		Key: key,
	}
	// https://gorm.io/docs/update.html#Save-All-Fields
	DB.FirstOrCreate(&option, Option{Key: key})
	option.Value = value
	// Save is a combination function.
	// If save value does not contain primary key, it will execute Create,
	// otherwise it will execute Update (with all fields).
	DB.Save(&option)
	// Update OptionMap
	return updateOptionMap(key, value)
}

// UpdateOptionsBulk persists multiple key/value pairs in a single database
// transaction, then dispatches them through updateOptionMap in one pass. If
// any DB write fails the whole transaction rolls back and no in-memory state
// is touched — safe for callers that must commit a set of related options
// atomically (e.g. payment gateway binding).
func UpdateOptionsBulk(values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		for k, v := range values {
			option := Option{Key: k}
			if err := tx.FirstOrCreate(&option, Option{Key: k}).Error; err != nil {
				return err
			}
			option.Value = v
			if err := tx.Save(&option).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for k, v := range values {
		if err := updateOptionMap(k, v); err != nil {
			return err
		}
	}
	return nil
}

func updateOptionMap(key string, value string) (err error) {
	common.OptionMapRWMutex.Lock()
	defer common.OptionMapRWMutex.Unlock()
	common.OptionMap[key] = value

	// 检查是否是模型配置 - 使用更规范的方式处理
	if handleConfigUpdate(key, value) {
		return nil // 已由配置系统处理
	}

	// 处理传统配置项...
	if strings.HasSuffix(key, "Permission") {
		intValue, _ := strconv.Atoi(value)
		switch key {
		case "FileUploadPermission":
			common.FileUploadPermission = intValue
		case "FileDownloadPermission":
			common.FileDownloadPermission = intValue
		case "ImageUploadPermission":
			common.ImageUploadPermission = intValue
		case "ImageDownloadPermission":
			common.ImageDownloadPermission = intValue
		}
	}
	if strings.HasSuffix(key, "Enabled") || key == "DefaultCollapseSidebar" || key == "DefaultUseAutoGroup" || key == "SMTPForceAuthLogin" {
		boolValue := value == "true"
		switch key {
		case "PasswordRegisterEnabled":
			common.PasswordRegisterEnabled = boolValue
		case "PasswordLoginEnabled":
			common.PasswordLoginEnabled = boolValue
		case "EmailVerificationEnabled":
			common.EmailVerificationEnabled = boolValue
		case "GitHubOAuthEnabled":
			common.GitHubOAuthEnabled = boolValue
		case "LinuxDOOAuthEnabled":
			common.LinuxDOOAuthEnabled = boolValue
		case "WeChatAuthEnabled":
			common.WeChatAuthEnabled = boolValue
		case "TelegramOAuthEnabled":
			common.TelegramOAuthEnabled = boolValue
		case "TurnstileCheckEnabled":
			common.TurnstileCheckEnabled = boolValue
		case "RegisterEnabled":
			common.RegisterEnabled = boolValue
		case "EmailDomainRestrictionEnabled":
			common.EmailDomainRestrictionEnabled = boolValue
		case "EmailAliasRestrictionEnabled":
			common.EmailAliasRestrictionEnabled = boolValue
		case "AutomaticDisableChannelEnabled":
			common.AutomaticDisableChannelEnabled = boolValue
		case "AutomaticEnableChannelEnabled":
			common.AutomaticEnableChannelEnabled = boolValue
		case "LogConsumeEnabled":
			common.LogConsumeEnabled = boolValue
		case "DisplayInCurrencyEnabled":
			// 兼容旧字段：同步到新配置 general_setting.quota_display_type（运行时生效）
			// true -> USD, false -> TOKENS
			newVal := "USD"
			if !boolValue {
				newVal = "TOKENS"
			}
			if cfg := config.GlobalConfig.Get("general_setting"); cfg != nil {
				_ = config.UpdateConfigFromMap(cfg, map[string]string{"quota_display_type": newVal})
			}
		case "DisplayTokenStatEnabled":
			common.DisplayTokenStatEnabled = boolValue
		case "DrawingEnabled":
			common.DrawingEnabled = boolValue
		case "TaskEnabled":
			common.TaskEnabled = boolValue
		case "DataExportEnabled":
			common.DataExportEnabled = boolValue
		case "DefaultCollapseSidebar":
			common.DefaultCollapseSidebar = boolValue
		case "MjNotifyEnabled":
			setting.MjNotifyEnabled = boolValue
		case "MjAccountFilterEnabled":
			setting.MjAccountFilterEnabled = boolValue
		case "MjModeClearEnabled":
			setting.MjModeClearEnabled = boolValue
		case "MjForwardUrlEnabled":
			setting.MjForwardUrlEnabled = boolValue
		case "MjActionCheckSuccessEnabled":
			setting.MjActionCheckSuccessEnabled = boolValue
		case "CheckSensitiveEnabled":
			setting.CheckSensitiveEnabled = boolValue
		case "DemoSiteEnabled":
			operation_setting.DemoSiteEnabled = boolValue
		case "SelfUseModeEnabled":
			operation_setting.SelfUseModeEnabled = boolValue
		case "CheckSensitiveOnPromptEnabled":
			setting.CheckSensitiveOnPromptEnabled = boolValue
		case "ModelRequestRateLimitEnabled":
			setting.ModelRequestRateLimitEnabled = boolValue
		case "StopOnSensitiveEnabled":
			setting.StopOnSensitiveEnabled = boolValue
		case "SMTPSSLEnabled":
			common.SMTPSSLEnabled = boolValue
		case "SMTPForceAuthLogin":
			common.SMTPForceAuthLogin = boolValue
		case "WorkerAllowHttpImageRequestEnabled":
			system_setting.WorkerAllowHttpImageRequestEnabled = boolValue
		case "DefaultUseAutoGroup":
			setting.DefaultUseAutoGroup = boolValue
		case "ExposeRatioEnabled":
			ratio_setting.SetExposeRatioEnabled(boolValue)
		}
	}
	switch key {
	case "EmailDomainWhitelist":
		common.EmailDomainWhitelist = strings.Split(value, ",")
	case "SMTPServer":
		common.SMTPServer = value
	case "SMTPPort":
		intValue, _ := strconv.Atoi(value)
		common.SMTPPort = intValue
	case "SMTPAccount":
		common.SMTPAccount = value
	case "SMTPFrom":
		common.SMTPFrom = value
	case "SMTPToken":
		common.SMTPToken = value
	case "ServerAddress":
		system_setting.ServerAddress = value
	case "WorkerUrl":
		system_setting.WorkerUrl = value
	case "WorkerValidKey":
		system_setting.WorkerValidKey = value
	case "PayAddress":
		operation_setting.PayAddress = value
	case "Chats":
		err = setting.UpdateChatsByJsonString(value)
	case "AutoGroups":
		err = setting.UpdateAutoGroupsByJsonString(value)
	case "CustomCallbackAddress":
		operation_setting.CustomCallbackAddress = value
	case "EpayId":
		operation_setting.EpayId = value
	case "EpayKey":
		operation_setting.EpayKey = value
	case "Price":
		operation_setting.Price, _ = strconv.ParseFloat(value, 64)
	case "USDExchangeRate":
		operation_setting.USDExchangeRate, _ = strconv.ParseFloat(value, 64)
	case "MinTopUp":
		operation_setting.MinTopUp, _ = strconv.Atoi(value)
	case "StripeApiSecret":
		setting.StripeApiSecret = value
	case "StripeWebhookSecret":
		setting.StripeWebhookSecret = value
	case "StripePriceId":
		setting.StripePriceId = value
	case "StripeUnitPrice":
		setting.StripeUnitPrice, _ = strconv.ParseFloat(value, 64)
	case "StripeMinTopUp":
		setting.StripeMinTopUp, _ = strconv.Atoi(value)
	case "StripePromotionCodesEnabled":
		setting.StripePromotionCodesEnabled = value == "true"
	case "CreemApiKey":
		setting.CreemApiKey = value
	case "CreemProducts":
		setting.CreemProducts = value
	case "CreemTestMode":
		setting.CreemTestMode = value == "true"
	case "CreemWebhookSecret":
		setting.CreemWebhookSecret = value
	case "WaffoEnabled":
		setting.WaffoEnabled = value == "true"
	case "WaffoApiKey":
		setting.WaffoApiKey = value
	case "WaffoPrivateKey":
		setting.WaffoPrivateKey = value
	case "WaffoPublicCert":
		setting.WaffoPublicCert = value
	case "WaffoSandboxPublicCert":
		setting.WaffoSandboxPublicCert = value
	case "WaffoSandboxApiKey":
		setting.WaffoSandboxApiKey = value
	case "WaffoSandboxPrivateKey":
		setting.WaffoSandboxPrivateKey = value
	case "WaffoSandbox":
		setting.WaffoSandbox = value == "true"
	case "WaffoMerchantId":
		setting.WaffoMerchantId = value
	case "WaffoNotifyUrl":
		setting.WaffoNotifyUrl = value
	case "WaffoReturnUrl":
		setting.WaffoReturnUrl = value
	case "WaffoSubscriptionReturnUrl":
		setting.WaffoSubscriptionReturnUrl = value
	case "WaffoCurrency":
		setting.WaffoCurrency = value
	case "WaffoUnitPrice":
		setting.WaffoUnitPrice, _ = strconv.ParseFloat(value, 64)
	case "WaffoMinTopUp":
		setting.WaffoMinTopUp, _ = strconv.Atoi(value)
	case "WaffoPancakeMerchantID":
		setting.WaffoPancakeMerchantID = value
	case "WaffoPancakePrivateKey":
		setting.WaffoPancakePrivateKey = value
	case "WaffoPancakeReturnURL":
		setting.WaffoPancakeReturnURL = value
	case "WaffoPancakeStoreID":
		setting.WaffoPancakeStoreID = value
	case "WaffoPancakeProductID":
		setting.WaffoPancakeProductID = value
	case "WaffoPancakeUnitPrice":
		setting.WaffoPancakeUnitPrice, _ = strconv.ParseFloat(value, 64)
	case "WaffoPancakeMinTopUp":
		setting.WaffoPancakeMinTopUp, _ = strconv.Atoi(value)
	case "TopupGroupRatio":
		err = common.UpdateTopupGroupRatioByJSONString(value)
	case "GitHubClientId":
		common.GitHubClientId = value
	case "GitHubClientSecret":
		common.GitHubClientSecret = value
	case "LinuxDOClientId":
		common.LinuxDOClientId = value
	case "LinuxDOClientSecret":
		common.LinuxDOClientSecret = value
	case "LinuxDOMinimumTrustLevel":
		common.LinuxDOMinimumTrustLevel, _ = strconv.Atoi(value)
	case "Footer":
		common.Footer = value
	case "SystemName":
		common.SystemName = value
	case "Logo":
		common.Logo = value
	case "WeChatServerAddress":
		common.WeChatServerAddress = value
	case "WeChatServerToken":
		common.WeChatServerToken = value
	case "WeChatAccountQRCodeImageURL":
		common.WeChatAccountQRCodeImageURL = value
	case "TelegramBotToken":
		common.TelegramBotToken = value
	case "TelegramBotName":
		common.TelegramBotName = value
	case "TurnstileSiteKey":
		common.TurnstileSiteKey = value
	case "TurnstileSecretKey":
		common.TurnstileSecretKey = value
	case "QuotaForNewUser":
		common.QuotaForNewUser, _ = strconv.Atoi(value)
	case "QuotaForInviter":
		common.QuotaForInviter, _ = strconv.Atoi(value)
	case "QuotaForInvitee":
		common.QuotaForInvitee, _ = strconv.Atoi(value)
	case "QuotaRemindThreshold":
		common.QuotaRemindThreshold, _ = strconv.Atoi(value)
	case "PreConsumedQuota":
		common.PreConsumedQuota, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitCount":
		setting.ModelRequestRateLimitCount, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitDurationMinutes":
		setting.ModelRequestRateLimitDurationMinutes, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitSuccessCount":
		setting.ModelRequestRateLimitSuccessCount, _ = strconv.Atoi(value)
	case "ModelRequestRateLimitGroup":
		err = setting.UpdateModelRequestRateLimitGroupByJSONString(value)
	case "RetryTimes":
		common.RetryTimes, _ = strconv.Atoi(value)
	case "DataExportInterval":
		common.DataExportInterval, _ = strconv.Atoi(value)
	case "DataExportDefaultTime":
		common.DataExportDefaultTime = value
	case "ModelRatio":
		err = ratio_setting.UpdateModelRatioByJSONString(value)
	case "GroupRatio":
		err = ratio_setting.UpdateGroupRatioByJSONString(value)
	case "GroupGroupRatio":
		err = ratio_setting.UpdateGroupGroupRatioByJSONString(value)
	case "UserUsableGroups":
		err = setting.UpdateUserUsableGroupsByJSONString(value)
	case "CompletionRatio":
		err = ratio_setting.UpdateCompletionRatioByJSONString(value)
	case "ModelPrice":
		err = ratio_setting.UpdateModelPriceByJSONString(value)
	case "ModelGroupPrice":
		err = ratio_setting.UpdateModelGroupPriceByJSONString(value)
	case "CacheRatio":
		err = ratio_setting.UpdateCacheRatioByJSONString(value)
	case "CreateCacheRatio":
		err = ratio_setting.UpdateCreateCacheRatioByJSONString(value)
	case "ImageRatio":
		err = ratio_setting.UpdateImageRatioByJSONString(value)
	case "AudioRatio":
		err = ratio_setting.UpdateAudioRatioByJSONString(value)
	case "AudioCompletionRatio":
		err = ratio_setting.UpdateAudioCompletionRatioByJSONString(value)
	case "TopUpLink":
		common.TopUpLink = value
	//case "ChatLink":
	//	common.ChatLink = value
	//case "ChatLink2":
	//	common.ChatLink2 = value
	case "ChannelDisableThreshold":
		common.ChannelDisableThreshold, _ = strconv.ParseFloat(value, 64)
	case "QuotaPerUnit":
		common.QuotaPerUnit, _ = strconv.ParseFloat(value, 64)
	case "SensitiveWords":
		setting.SensitiveWordsFromString(value)
	case "AutomaticDisableKeywords":
		operation_setting.AutomaticDisableKeywordsFromString(value)
	case "AutomaticDisableStatusCodes":
		err = operation_setting.AutomaticDisableStatusCodesFromString(value)
	case "AutomaticRetryStatusCodes":
		err = operation_setting.AutomaticRetryStatusCodesFromString(value)
	case "StreamCacheQueueLength":
		setting.StreamCacheQueueLength, _ = strconv.Atoi(value)
	case "PayMethods":
		err = operation_setting.UpdatePayMethodsByJsonString(value)
	case "WaffoPayMethods":
		// WaffoPayMethods is read directly from OptionMap via setting.GetWaffoPayMethods().
		// The value is already stored in OptionMap at the top of this function (line: common.OptionMap[key] = value).
		// No additional in-memory variable to update.
	}
	return err
}

// handleConfigUpdate 处理分层配置更新，返回是否已处理
func handleConfigUpdate(key, value string) bool {
	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return false // 不是分层配置
	}

	configName := parts[0]
	configKey := parts[1]

	// 获取配置对象
	cfg := config.GlobalConfig.Get(configName)
	if cfg == nil {
		return false // 未注册的配置
	}

	// 更新配置
	configMap := map[string]string{
		configKey: value,
	}
	config.UpdateConfigFromMap(cfg, configMap)

	// 特定配置的后处理
	if configName == "performance_setting" {
		performance_setting.UpdateAndSync()
	} else if configName == "tool_price_setting" {
		operation_setting.RebuildToolPriceIndex()
	} else if configName == "billing_setting" {
		InvalidatePricingCache()
		ratio_setting.InvalidateExposedDataCache()
	} else if configName == "theme" {
		system_setting.UpdateAndSyncTheme()
	}

	return true // 已处理
}
