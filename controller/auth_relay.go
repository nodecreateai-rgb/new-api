package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/helper"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const oauth2AuthModelName = "oauth2"

func RelayAuth(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "server_error",
			},
		})
		return
	}
	relayInfo.InitChannelMeta(c)
	relayInfo.OriginModelName = oauth2AuthModelName
	relayInfo.Action = "auth"

	priceData, err := helper.ModelPriceHelperPerCall(c, relayInfo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}
	relayInfo.PriceData = priceData

	if !priceData.FreeModel {
		if apiErr := service.PreConsumeBilling(c, priceData.Quota, relayInfo); apiErr != nil {
			c.JSON(apiErr.StatusCode, gin.H{"error": apiErr.ToOpenAIError()})
			return
		}
	}

	billed := false
	defer func() {
		if !billed && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	bodyStorage, err := common.GetBodyStorage(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}
	bodyBytes, err := bodyStorage.Bytes()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	baseURL := strings.TrimRight(relayInfo.ChannelBaseUrl, "/")
	if baseURL == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "oauth2 upstream base url is not configured",
				"type":    "server_error",
			},
		})
		return
	}
	upstreamURL := baseURL + "/v1/oauth"

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "server_error",
			},
		})
		return
	}
	contentType := strings.TrimSpace(c.GetHeader("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-api-key", relayInfo.ApiKey)

	client := service.GetHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("oauth2 upstream request failed: %s", err.Error()),
				"type":    "upstream_error",
			},
		})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("read oauth2 upstream response failed: %s", err.Error()),
				"type":    "upstream_error",
			},
		})
		return
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		if settleErr := service.SettleBilling(c, relayInfo, priceData.Quota); settleErr != nil {
			common.SysError("settle oauth2 auth billing error: " + settleErr.Error())
		}
		service.LogTaskConsumption(c, relayInfo)
		billed = true
	}

	for key, values := range resp.Header {
		if len(values) == 0 {
			continue
		}
		lowerKey := strings.ToLower(key)
		if lowerKey == "content-length" || lowerKey == "transfer-encoding" || lowerKey == "connection" {
			continue
		}
		c.Header(key, values[0])
	}
	c.Status(resp.StatusCode)
	if len(respBody) > 0 {
		_, _ = c.Writer.Write(respBody)
	}
}
