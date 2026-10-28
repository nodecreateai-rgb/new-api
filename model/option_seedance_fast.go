package model

import (
	"errors"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"gorm.io/gorm"
)

const (
	seedanceFastPublicModel = "seedance-2.0-fast-480p"
	seedanceFastChannelName = "Seedance 2.0 Fast"
	// seedanceFast720Model is the same pool rendered at 720p (45 credits/s,
	// 15 s = 675, so one 1000-credit login covers one 720p plus one 480p).
	seedanceFast720Model       = "seedance-2.0-720p"
	seedanceFast720Description = "Seedance 2.0 720p 文生/图生视频（异步，¥3/次，VIP10 ¥2.5/次，903，原生720P，15秒，不卡人脸，最多9张图、3段音频）"
	// seedanceFastPublic720Model is the public alias of the same 720p render
	// (seedance-2.0-720p stays private to one user): ¥2.5/次 in every group.
	seedanceFastPublic720Model       = "seedance-2.0-fast-720p"
	seedanceFastPublic720Description = "Seedance 2.0 Fast 720p 文生/图生视频（异步，¥2.5/次，903，原生720P，15秒，不卡人脸，最多9张图、3段音频）"
	seedanceFastModelsCSV            = seedanceFastPublicModel + "," + seedanceFast720Model + "," + seedanceFastPublic720Model
	seedanceFastMappingJSON          = `{"seedance-2.0-fast-480p":"seedance-2.0-fast","seedance-2.0-720p":"seedance-2.0-720p","seedance-2.0-fast-720p":"seedance-2.0-720p"}`
	seedanceFastGroupsCSV            = "default,vip,svip,vip1,vip2,vip3,vip6,vip8,vip9,vip10"
	seedanceFastDescription          = "Seedance 2.0 Fast 480p 文生/图生视频（异步，¥1.5/次，VIP10 ¥1.4/次，903，最长15秒，不卡人脸，最多9张图、3段音频）"
	seedanceFastEndpointJSON         = `{"openai-video":{"path":"/v1/videos","method":"POST"}}`
)

func ensureSeedanceFastRouting() error {
	baseURL := strings.TrimSpace(os.Getenv("SEEDANCE_FAST_BASE_URL"))
	if baseURL == "" {
		baseURL = "http://seedance-fast:38685"
	}
	key := strings.TrimSpace(os.Getenv("SEEDANCE_FAST_GATEWAY_KEY"))
	if key == "" {
		if keyFile := strings.TrimSpace(os.Getenv("SEEDANCE_FAST_GATEWAY_KEY_FILE")); keyFile != "" {
			if raw, err := os.ReadFile(keyFile); err == nil {
				key = strings.TrimSpace(string(raw))
			}
		}
	}
	if key == "" {
		key = "seedance"
	}

	publicModels := []string{seedanceFastPublicModel, seedanceFast720Model, seedanceFastPublic720Model}
	groups := []string{"default", "vip", "svip", "vip1", "vip2", "vip3", "vip6", "vip8", "vip9", "vip10"}
	modelDescriptions := map[string]string{
		seedanceFastPublicModel:    seedanceFastDescription,
		seedanceFast720Model:       seedanceFast720Description,
		seedanceFastPublic720Model: seedanceFastPublic720Description,
	}

	if common.UsingClickHouse {
		return ensureSeedanceFastRoutingClickHouse(baseURL, key, publicModels, groups, modelDescriptions)
	}

	var channel Channel
	err := DB.Where("name = ?", seedanceFastChannelName).First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		weight := uint64(100)
		priority := int64(10)
		autoBan := 0
		mapping := seedanceFastMappingJSON
		channel = Channel{
			Type: constant.ChannelTypeSora, Key: key, Status: common.ChannelStatusEnabled,
			Name: seedanceFastChannelName, Weight: &weight, CreatedTime: common.GetTimestamp(),
			BaseURL: stringPtr(baseURL), Models: seedanceFastModelsCSV, Group: seedanceFastGroupsCSV,
			ModelMapping: &mapping, Priority: &priority, AutoBan: &autoBan,
		}
		if err := DB.Create(&channel).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
		"type": constant.ChannelTypeSora, "key": key, "status": common.ChannelStatusEnabled,
		"name": seedanceFastChannelName, "base_url": baseURL, "models": seedanceFastModelsCSV, "group": seedanceFastGroupsCSV,
		"model_mapping": seedanceFastMappingJSON, "priority": 10, "weight": 100, "auto_ban": 0,
	}).Error; err != nil {
		return err
	}

	if err := DB.Model(&Ability{}).Where("channel_id = ? AND model NOT IN ?", channel.Id, publicModels).
		Update("enabled", false).Error; err != nil {
		return err
	}
	for _, modelName := range publicModels {
		if err := DB.Model(&Ability{}).Where("model = ? AND channel_id <> ?", modelName, channel.Id).
			Update("enabled", false).Error; err != nil {
			return err
		}
		allowed := map[string]struct{}{}
		for _, group := range groups {
			allowed[group] = struct{}{}
			ability := Ability{Group: group, Model: modelName, ChannelId: channel.Id}
			if err := DB.Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, modelName, channel.Id).
				FirstOrCreate(&ability).Error; err != nil {
				return err
			}
			if err := DB.Model(&Ability{}).Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, modelName, channel.Id).
				Updates(map[string]any{"enabled": true, "priority": int64(10), "weight": uint64(100)}).Error; err != nil {
				return err
			}
		}
		var existing []Ability
		if err := DB.Where("model = ? AND channel_id = ?", modelName, channel.Id).Find(&existing).Error; err != nil {
			return err
		}
		for _, ability := range existing {
			if _, ok := allowed[ability.Group]; ok {
				continue
			}
			if err := DB.Model(&Ability{}).Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", ability.Group, modelName, channel.Id).
				Update("enabled", false).Error; err != nil {
				return err
			}
		}
	}

	for _, publicModel := range publicModels {
		desc := modelDescriptions[publicModel]
		var meta Model
		err = DB.Unscoped().Where("model_name = ?", publicModel).First(&meta).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			meta = Model{
				ModelName: publicModel, Description: desc, Icon: "", Tags: "video",
				Endpoints: seedanceFastEndpointJSON, Status: 1, SyncOfficial: 0,
				CreatedTime: common.GetTimestamp(), UpdatedTime: common.GetTimestamp(),
			}
			if err := DB.Create(&meta).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := DB.Unscoped().Model(&Model{}).Where("id = ?", meta.Id).Updates(map[string]any{
			"description": desc, "icon": "", "tags": "video", "endpoints": seedanceFastEndpointJSON,
			"status": 1, "sync_official": 0, "deleted_at": nil, "updated_time": common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	InvalidatePricingCache()
	InitChannelCache()
	return nil
}

func ensureSeedanceFastRoutingClickHouse(baseURL, key string, publicModels, groups []string, modelDescriptions map[string]string) error {
	var channel Channel
	err := DB.Where("name = ?", seedanceFastChannelName).First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		id := nextClickHouseTableID(DB, "channels")
		now := common.GetTimestamp()
		info := `{"is_multi_key":false,"multi_key_size":0,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":""}`
		if err := DB.Exec(`INSERT INTO channels (
			id, type, key, status, name, weight, created_time, test_time, response_time,
			base_url, other, balance, balance_updated_time, models, `+commonGroupCol+`, used_quota,
			model_mapping, status_code_mapping, priority, auto_ban, other_info, channel_info, settings
		) VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, ?, '', 0, 0, ?, ?, 0, ?, '', ?, 0, '', ?, '')`,
			id, constant.ChannelTypeSora, key, common.ChannelStatusEnabled, seedanceFastChannelName, uint64(100), now,
			baseURL, seedanceFastModelsCSV, seedanceFastGroupsCSV, seedanceFastMappingJSON, int64(10), info,
		).Error; err != nil {
			return err
		}
		channel.Id = int(id)
	} else if err != nil {
		return err
	} else if err := DB.Exec(`ALTER TABLE channels UPDATE
		type = ?, key = ?, status = ?, name = ?, base_url = ?, models = ?, `+commonGroupCol+` = ?, model_mapping = ?, priority = 10, weight = 100, auto_ban = 0
		WHERE id = ?`, constant.ChannelTypeSora, key, common.ChannelStatusEnabled, seedanceFastChannelName, baseURL, seedanceFastModelsCSV, seedanceFastGroupsCSV, seedanceFastMappingJSON, channel.Id).Error; err != nil {
		return err
	}

	if err := DB.Model(&Ability{}).Where("channel_id = ? AND model NOT IN ?", channel.Id, publicModels).
		Update("enabled", false).Error; err != nil {
		return err
	}

	for _, modelName := range publicModels {
		if err := DB.Model(&Ability{}).Where("model = ? AND channel_id <> ?", modelName, channel.Id).
			Update("enabled", false).Error; err != nil {
			return err
		}
		for _, group := range groups {
			var count int64
			if err := DB.Model(&Ability{}).Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, modelName, channel.Id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := DB.Exec(
					`INSERT INTO abilities (`+commonGroupCol+`, model, channel_id, enabled, priority, weight, tag) VALUES (?, ?, ?, 1, 10, 100, '')`,
					group, modelName, channel.Id,
				).Error; err != nil {
					return err
				}
			} else if err := DB.Model(&Ability{}).Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, modelName, channel.Id).
				Updates(map[string]any{"enabled": true, "priority": int64(10), "weight": uint64(100)}).Error; err != nil {
				return err
			}
		}
	}

	for _, publicModel := range publicModels {
		desc := modelDescriptions[publicModel]
		var meta Model
		err = DB.Unscoped().Where("model_name = ?", publicModel).First(&meta).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			id := nextClickHouseTableID(DB, "models")
			now := common.GetTimestamp()
			if err := DB.Exec(
				`INSERT INTO models (id, model_name, description, icon, tags, endpoints, status, sync_official, created_time, updated_time, name_rule)
				 VALUES (?, ?, ?, '', 'video', ?, 1, 0, ?, ?, 0)`,
				id, publicModel, desc, seedanceFastEndpointJSON, now, now,
			).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := DB.Unscoped().Model(&Model{}).Where("id = ?", meta.Id).Updates(map[string]any{
			"description": desc, "tags": "video", "endpoints": seedanceFastEndpointJSON,
			"status": 1, "sync_official": 0, "deleted_at": nil, "updated_time": common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		if err := DB.Exec(`ALTER TABLE models UPDATE description = ?, status = 1, updated_time = ? WHERE id = ?`, desc, common.GetTimestamp(), meta.Id).Error; err != nil {
			return err
		}
	}
	InvalidatePricingCache()
	InitChannelCache()
	return nil
}
