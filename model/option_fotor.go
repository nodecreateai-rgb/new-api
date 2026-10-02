package model

import (
	"errors"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"gorm.io/gorm"
)

func ensureFotor2apiRouting() error {
	const neutralName = "Fotor Video"
	const modelsCSV = "seedance-2.0-c2,seedance-2.0-480p-c2,seedance-2.0-fast-c2,seedance-2.0-fast-480p-c2,seedance-2.0-mini-c2,wan-3.0-c2"
	const mappingJSON = `{"seedance-2.0-c2":"seedance-2.0","seedance-2.0-480p-c2":"seedance-2.0-480p","seedance-2.0-fast-c2":"seedance-2.0-fast","seedance-2.0-fast-480p-c2":"seedance-2.0-fast-480p","seedance-2.0-mini-c2":"seedance-mini","wan-3.0-c2":"wan-3.0"}`
	const groupsCSV = "default,vip,svip,vip1,vip2,vip3,vip6,vip8,vip9"
	baseURL := strings.TrimSpace(os.Getenv("FOTOR2API_BASE_URL"))
	if baseURL == "" {
		baseURL = "http://fotor2api:38684"
	}
	key := strings.TrimSpace(os.Getenv("FOTOR2API_GATEWAY_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("FOTOR2API_API_KEY"))
	}
	if key == "" {
		if keyFile := strings.TrimSpace(os.Getenv("FOTOR2API_GATEWAY_KEY_FILE")); keyFile != "" {
			if raw, err := os.ReadFile(keyFile); err == nil {
				key = strings.TrimSpace(string(raw))
			}
		}
	}

	publicModels := []string{"seedance-2.0-c2", "seedance-2.0-480p-c2", "seedance-2.0-fast-c2", "seedance-2.0-fast-480p-c2", "seedance-2.0-mini-c2", "wan-3.0-c2"}
	groups := []string{"default", "vip", "svip", "vip1", "vip2", "vip3", "vip6", "vip8", "vip9"}
	modelDescriptions := map[string]string{
		"seedance-2.0-c2":           "Seedance 2.0 文生/图生视频（异步，¥0.8/次，903，最长15秒，不卡脸）",
		"seedance-2.0-480p-c2":      "Seedance 2.0 480p 文生/图生视频（异步，¥0.8/次，不超分，903，最长15秒，不卡脸）",
		"seedance-2.0-fast-c2":      "Seedance 2.0 Fast 文生/图生视频（异步，¥0.7/次，913，最长15秒，不卡脸）",
		"seedance-2.0-fast-480p-c2": "Seedance 2.0 Fast 480p 文生/图生视频（异步，¥0.7/次，不超分，913，最长15秒，不卡脸）",
		"seedance-2.0-mini-c2":      "Seedance 2.0 Mini 文生/图生视频（异步，¥0.6/次，903，超分720P，最长15秒）",
		"wan-3.0-c2":                "Wan 3.0 文生/图生视频（异步，¥1.5/次，原生720P，10图5音5视，最长30秒）",
	}
	endpoint := `{"openai-video":{"path":"/v1/videos","method":"POST"}}`

	if common.UsingClickHouse {
		return ensureFotor2apiRoutingClickHouse(neutralName, modelsCSV, mappingJSON, groupsCSV, baseURL, key, publicModels, groups, modelDescriptions, endpoint)
	}

	var channel Channel
	err := DB.Where("name = ?", neutralName).First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		weight := uint64(100)
		priority := int64(10)
		autoBan := 0
		mapping := mappingJSON
		channel = Channel{
			Type: constant.ChannelTypeSora, Key: key, Status: common.ChannelStatusEnabled,
			Name: neutralName, Weight: &weight, CreatedTime: common.GetTimestamp(),
			BaseURL: stringPtr(baseURL), Models: modelsCSV, Group: groupsCSV,
			ModelMapping: &mapping, Priority: &priority, AutoBan: &autoBan,
		}
		if err := DB.Create(&channel).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if key == "" {
			key = strings.TrimSpace(channel.Key)
		}
		updates := map[string]any{
			"type": constant.ChannelTypeSora, "status": common.ChannelStatusEnabled,
			"name": neutralName, "base_url": baseURL, "models": modelsCSV, "group": groupsCSV,
			"model_mapping": mappingJSON, "priority": 10, "weight": 100, "auto_ban": 0,
		}
		if key != "" {
			updates["key"] = key
		}
		if err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Updates(updates).Error; err != nil {
			return err
		}
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

	for _, name := range publicModels {
		desc := modelDescriptions[name]
		var meta Model
		err = DB.Unscoped().Where("model_name = ?", name).First(&meta).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			meta = Model{
				ModelName: name, Description: desc, Icon: "", Tags: "video",
				Endpoints: endpoint, Status: 1, SyncOfficial: 0,
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
			"description": desc, "icon": "", "tags": "video", "endpoints": endpoint,
			"status": 1, "sync_official": 0, "deleted_at": nil, "updated_time": common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
	}
	InvalidatePricingCache()
	InitChannelCache()
	return nil
}

func ensureFotor2apiRoutingClickHouse(neutralName, modelsCSV, mappingJSON, groupsCSV, baseURL, key string, publicModels, groups []string, modelDescriptions map[string]string, endpoint string) error {
	var channel Channel
	err := DB.Where("name = ?", neutralName).First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		id := nextClickHouseTableID(DB, "channels")
		now := common.GetTimestamp()
		info := `{"is_multi_key":false,"multi_key_size":0,"multi_key_status_list":null,"multi_key_polling_index":0,"multi_key_mode":""}`
		if err := DB.Exec(`INSERT INTO channels (
			id, type, key, status, name, weight, created_time, test_time, response_time,
			base_url, other, balance, balance_updated_time, models, `+commonGroupCol+`, used_quota,
			model_mapping, status_code_mapping, priority, auto_ban, other_info, channel_info, settings
		) VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, ?, '', 0, 0, ?, ?, 0, ?, '', ?, 0, '', ?, '')`,
			id, constant.ChannelTypeSora, key, common.ChannelStatusEnabled, neutralName, uint64(100), now,
			baseURL, modelsCSV, groupsCSV, mappingJSON, int64(10), info,
		).Error; err != nil {
			return err
		}
		channel.Id = int(id)
	} else if err != nil {
		return err
	} else {
		if key == "" {
			key = strings.TrimSpace(channel.Key)
		}
		if key == "" {
			if err := DB.Exec(`ALTER TABLE channels UPDATE
				type = ?, status = ?, name = ?, base_url = ?, models = ?, `+commonGroupCol+` = ?, model_mapping = ?, priority = 10, weight = 100, auto_ban = 0
				WHERE id = ?`, constant.ChannelTypeSora, common.ChannelStatusEnabled, neutralName, baseURL, modelsCSV, groupsCSV, mappingJSON, channel.Id).Error; err != nil {
				return err
			}
		} else if err := DB.Exec(`ALTER TABLE channels UPDATE
			type = ?, key = ?, status = ?, name = ?, base_url = ?, models = ?, `+commonGroupCol+` = ?, model_mapping = ?, priority = 10, weight = 100, auto_ban = 0
			WHERE id = ?`, constant.ChannelTypeSora, key, common.ChannelStatusEnabled, neutralName, baseURL, modelsCSV, groupsCSV, mappingJSON, channel.Id).Error; err != nil {
			return err
		}
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
				id, publicModel, desc, endpoint, now, now,
			).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if err := DB.Unscoped().Model(&Model{}).Where("id = ?", meta.Id).Updates(map[string]any{
			"description": desc, "tags": "video", "endpoints": endpoint,
			"status": 1, "sync_official": 0, "deleted_at": nil, "updated_time": common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		if err := DB.Exec(`ALTER TABLE models UPDATE description = ? WHERE id = ?`, desc, meta.Id).Error; err != nil {
			return err
		}
	}
	InvalidatePricingCache()
	InitChannelCache()
	return nil
}
