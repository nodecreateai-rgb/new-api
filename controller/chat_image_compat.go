package controller

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const contextKeyChatImageCompat = "chat_image_compat"

func maybeConvertChatImageCompat(c *gin.Context, relayFormat types.RelayFormat, request dto.Request) (types.RelayFormat, dto.Request, error) {
	if relayFormat != types.RelayFormatOpenAI || relayconstant.Path2RelayMode(c.Request.URL.Path) != relayconstant.RelayModeChatCompletions {
		return relayFormat, request, nil
	}
	textReq, ok := request.(*dto.GeneralOpenAIRequest)
	if !ok || !common.IsImageGenerationModel(textReq.Model) {
		return relayFormat, request, nil
	}
	imageReq, relayMode, err := chatCompletionToImageRequest(c, textReq)
	if err != nil {
		return relayFormat, request, err
	}
	// Downstream relay info, channel selection, billing and adaptor URL generation are keyed by request path/relay mode.
	// Rewrite only the in-memory request path so /v1/chat/completions can reuse the normal image pipeline.
	if relayMode == relayconstant.RelayModeImagesEdits {
		c.Request.URL.Path = "/v1/images/edits"
	} else {
		c.Request.URL.Path = "/v1/images/generations"
	}
	c.Set("relay_mode", relayMode)
	c.Set(contextKeyChatImageCompat, true)
	return types.RelayFormatOpenAIImage, imageReq, nil
}

func chatCompletionToImageRequest(c *gin.Context, textReq *dto.GeneralOpenAIRequest) (*dto.ImageRequest, int, error) {
	if textReq == nil {
		return nil, relayconstant.RelayModeUnknown, fmt.Errorf("request is nil")
	}
	raw := map[string]json.RawMessage{}
	if storage, err := common.GetBodyStorage(c); err == nil {
		if body, bErr := storage.Bytes(); bErr == nil && len(body) > 0 {
			_ = common.Unmarshal(body, &raw)
		}
	}

	promptParts := make([]string, 0)
	imageURLs := make([]string, 0)
	for _, message := range textReq.Messages {
		if message.Role == "system" || message.Role == "developer" || message.Role == "user" {
			for _, item := range message.ParseContent() {
				switch item.Type {
				case dto.ContentTypeText:
					if strings.TrimSpace(item.Text) != "" {
						promptParts = append(promptParts, strings.TrimSpace(item.Text))
					}
				case dto.ContentTypeImageURL:
					if img := item.GetImageMedia(); img != nil && strings.TrimSpace(img.Url) != "" {
						imageURLs = append(imageURLs, strings.TrimSpace(img.Url))
					}
				}
			}
		}
	}
	if textReq.Prompt != nil {
		if p := stringifyPromptLike(textReq.Prompt); p != "" {
			promptParts = append(promptParts, p)
		}
	}
	if len(promptParts) == 0 {
		return nil, relayconstant.RelayModeUnknown, fmt.Errorf("field prompt/messages text is required for image model chat compatibility")
	}

	imageReq := &dto.ImageRequest{
		Model:          textReq.Model,
		Prompt:         strings.Join(promptParts, "\n"),
		ResponseFormat: "url",
	}
	if textReq.N != nil && *textReq.N > 0 {
		n := uint(*textReq.N)
		imageReq.N = &n
	}
	if textReq.Size != "" {
		imageReq.Size = textReq.Size
	}
	copyRawString(raw, "size", &imageReq.Size)
	copyRawString(raw, "aspect_ratio", &imageReq.AspectRatio)
	copyRawString(raw, "quality", &imageReq.Quality)
	copyResponseFormat(raw, &imageReq.ResponseFormat)
	copyRawBoolPtr(raw, "async", &imageReq.Async)
	copyRawBoolPtr(raw, "async_task", &imageReq.AsyncTask)
	copyRawBoolPtr(raw, "return_task_id", &imageReq.ReturnTaskID)
	copyRawString(raw, "callback_url", &imageReq.CallbackURL)
	copyRaw(raw, "images", &imageReq.Images)
	copyRaw(raw, "image", &imageReq.Image)
	copyRaw(raw, "image_url", &imageReq.ImageURL)
	copyRaw(raw, "image_urls", &imageReq.ImageURLs)

	if len(imageURLs) > 0 && len(imageReq.ImageURLs) == 0 && len(imageReq.ImageURL) == 0 && len(imageReq.Images) == 0 && len(imageReq.Image) == 0 {
		b, _ := common.Marshal(imageURLs)
		imageReq.ImageURLs = b
	}
	if imageReq.ResponseFormat == "" || imageReq.ResponseFormat == "b64_json" {
		imageReq.ResponseFormat = "url"
	}
	relayMode := relayconstant.RelayModeImagesGenerations
	if len(imageReq.ImageURLs) > 0 || len(imageReq.ImageURL) > 0 || len(imageReq.Images) > 0 || len(imageReq.Image) > 0 {
		relayMode = relayconstant.RelayModeImagesEdits
	}
	return imageReq, relayMode, nil
}

func stringifyPromptLike(v any) string {
	switch p := v.(type) {
	case string:
		return strings.TrimSpace(p)
	case []any:
		parts := make([]string, 0, len(p))
		for _, item := range p {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, "\n")
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", p))
	}
}

func copyRaw(raw map[string]json.RawMessage, key string, target *json.RawMessage) {
	if v, ok := raw[key]; ok && len(v) > 0 && string(v) != "null" {
		*target = append((*target)[:0], v...)
	}
}

func copyRawString(raw map[string]json.RawMessage, key string, target *string) {
	if v, ok := raw[key]; ok && len(v) > 0 {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			*target = strings.TrimSpace(s)
		}
	}
}

func copyRawBoolPtr(raw map[string]json.RawMessage, key string, target **bool) {
	if v, ok := raw[key]; ok && len(v) > 0 {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			*target = &b
		}
	}
}

func copyResponseFormat(raw map[string]json.RawMessage, target *string) {
	v, ok := raw["response_format"]
	if !ok || len(v) == 0 {
		return
	}
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		*target = strings.TrimSpace(s)
		return
	}
	var obj map[string]any
	if err := json.Unmarshal(v, &obj); err == nil {
		if typ := strings.TrimSpace(common.Interface2String(obj["type"])); typ != "" {
			*target = typ
		}
	}
}
