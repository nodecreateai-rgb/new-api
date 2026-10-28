package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"gorm.io/gorm"
)

var aivideoPublicModels = []string{"wan-3.0-480p"}

const aivideoChannelName = "Spark Video"

// ensureAiimagetovideo2apiRouting retires wan-3.0-480p: the Spark Video
// channel is disabled and the model is withdrawn on every startup.
func ensureAiimagetovideo2apiRouting() error {
	if err := disableChannelByName("Vibbit Video"); err != nil {
		return err
	}
	if err := disableChannelByName(aivideoChannelName); err != nil {
		return err
	}
	if err := retireMarketplaceModels(aivideoPublicModels); err != nil {
		return err
	}
	InitChannelCache()
	return nil
}

func ensureAiimagetovideo2apiRoutingClickHouse(neutralName, modelsCSV, mappingJSON, groupsCSV, baseURL, key string, publicModels, groups []string, modelDescriptions map[string]string, endpoint string) error {
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
	} else if err := DB.Exec(`ALTER TABLE channels UPDATE
		type = ?, key = ?, status = ?, name = ?, base_url = ?, models = ?, `+commonGroupCol+` = ?, model_mapping = ?, priority = 10, auto_ban = 0
		WHERE id = ?`, constant.ChannelTypeSora, key, common.ChannelStatusEnabled, neutralName, baseURL, modelsCSV, groupsCSV, mappingJSON, channel.Id).Error; err != nil {
		return err
	}

	if err := DB.Model(&Ability{}).Where("channel_id = ? AND model NOT IN ?", channel.Id, publicModels).
		Update("enabled", false).Error; err != nil {
		return err
	}

	for _, modelName := range publicModels {
		if err := DB.Model(&Ability{}).Where("model = ? AND channel_id NOT IN ?", modelName, aivideoChannelIDs(channel.Id)).
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
				Updates(map[string]any{"enabled": true, "priority": int64(10)}).Error; err != nil {
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
		if err := DB.Exec(`ALTER TABLE models UPDATE description = ?, status = 1, updated_time = ? WHERE id = ?`, desc, common.GetTimestamp(), meta.Id).Error; err != nil {
			return err
		}
	}
	InvalidatePricingCache()
	InitChannelCache()
	return nil
}

func aivideoChannelIDs(primary int) []int {
	ids := []int{primary}
	var extra []int
	if err := DB.Model(&Channel{}).Where("name LIKE ?", aivideoChannelName+"%").Pluck("id", &extra).Error; err == nil {
		for _, id := range extra {
			if id != primary {
				ids = append(ids, id)
			}
		}
	}
	return ids
}
