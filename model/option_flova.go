package model

import (
	"errors"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"gorm.io/gorm"
)

var flovaPublicModels = []string{
	"wan-3.0",
	"minimax-h3",
}

// flovaRetiredModels were sold through Flova once and are gone for good:
// their abilities and model-square entries are removed on every start.
var flovaRetiredModels = []string{"sora-2", "wan-3.0-prime"}

const flovaChannelName = "Flova Video"

func ensureFlova2apiRouting() error {
	const modelsCSV = "wan-3.0,minimax-h3"
	const mappingJSON = `{"wan-3.0":"wan-3.0","minimax-h3":"minimax-h3"}`
	const groupsCSV = "default,vip,svip,vip1,vip2,vip3,vip6,vip8,vip9,vip10"
	baseURL := strings.TrimSpace(os.Getenv("FLOVA2API_BASE_URL"))
	if baseURL == "" {
		baseURL = "http://flova2api:38686"
	}
	key := strings.TrimSpace(os.Getenv("FLOVA2API_GATEWAY_KEY"))
	if key == "" {
		if keyFile := strings.TrimSpace(os.Getenv("FLOVA2API_GATEWAY_KEY_FILE")); keyFile != "" {
			if raw, err := os.ReadFile(keyFile); err == nil {
				key = strings.TrimSpace(string(raw))
			}
		}
	}
	if key == "" {
		key = strings.TrimSpace(os.Getenv("FLOVA2API_API_KEY"))
	}
	if key == "" {
		key = "flova"
	}

	groups := []string{"default", "vip", "svip", "vip1", "vip2", "vip3", "vip6", "vip8", "vip9", "vip10"}
	modelDescriptions := map[string]string{
		"wan-3.0":       "Wan 3.0 文生/图生视频（异步，¥1/次，VIP9 ¥0.8/次，原生480P，2-20秒，画幅 adaptive/16:9/4:3/1:1/3:4/9:16，最多10张图片、0段视频、5段音频）",
		"minimax-h3":    "MiniMax H3 文生/图生视频（异步，¥0.5/次，原生768P，4-15秒，画幅 21:9/16:9/4:3/1:1/3:4/9:16，最多9张图、3段视频、3段音频）",
	}
	endpoint := `{"openai-video":{"path":"/v1/videos","method":"POST"}}`

	if common.UsingClickHouse {
		return ensureFlova2apiRoutingClickHouse(flovaChannelName, modelsCSV, mappingJSON, groupsCSV, baseURL, key, flovaPublicModels, groups, modelDescriptions, endpoint)
	}

	var channel Channel
	err := DB.Where("name = ?", flovaChannelName).First(&channel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		weight := uint64(100)
		priority := int64(10)
		autoBan := 0
		mapping := mappingJSON
		channel = Channel{
			Type: constant.ChannelTypeSora, Key: key, Status: common.ChannelStatusEnabled,
			Name: flovaChannelName, Weight: &weight, CreatedTime: common.GetTimestamp(),
			BaseURL: stringPtr(baseURL), Models: modelsCSV, Group: groupsCSV,
			ModelMapping: &mapping, Priority: &priority, AutoBan: &autoBan,
		}
		if err := DB.Create(&channel).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
		"type": constant.ChannelTypeSora, "key": key, "status": common.ChannelStatusEnabled,
		"name": flovaChannelName, "base_url": baseURL, "models": modelsCSV, "group": groupsCSV,
		"model_mapping": mappingJSON, "priority": 10, "weight": 100, "auto_ban": 0,
	}).Error; err != nil {
		return err
	}

	if err := DB.Model(&Ability{}).Where("channel_id = ? AND model NOT IN ?", channel.Id, flovaPublicModels).
		Update("enabled", false).Error; err != nil {
		return err
	}
	for _, modelName := range flovaPublicModels {
		if err := DB.Model(&Ability{}).Where("model = ? AND channel_id NOT IN ?", modelName, flovaChannelIDs(channel.Id)).
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
				Updates(map[string]any{"enabled": true, "priority": int64(10)}).Error; err != nil {
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

	for _, publicModel := range flovaPublicModels {
		desc := modelDescriptions[publicModel]
		var meta Model
		err = DB.Unscoped().Where("model_name = ?", publicModel).First(&meta).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			meta = Model{
				ModelName: publicModel, Description: desc, Icon: "", Tags: "video",
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
	if err := retireFlovaModelsClickHouse(); err != nil {
		return err
	}
	InvalidatePricingCache()
	InitChannelCache()
	return nil
}

// retireFlovaModelsClickHouse drops retired Flova models from every channel's
// model list, the abilities index and the model square, so no channel clone
// or restart can bring them back.
func retireFlovaModelsClickHouse() error {
	for _, m := range flovaRetiredModels {
		if err := DB.Exec(`ALTER TABLE abilities DELETE WHERE model = ?`, m).Error; err != nil {
			return err
		}
		if err := DB.Exec(`ALTER TABLE channels UPDATE models = arrayStringConcat(arrayFilter(x -> x != ?, splitByChar(',', models)), ',') WHERE has(splitByChar(',', models), ?)`, m, m).Error; err != nil {
			return err
		}
		if err := DB.Exec(`ALTER TABLE models DELETE WHERE model_name = ?`, m).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureFlova2apiRoutingClickHouse(neutralName, modelsCSV, mappingJSON, groupsCSV, baseURL, key string, publicModels, groups []string, modelDescriptions map[string]string, endpoint string) error {
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
		if err := DB.Model(&Ability{}).Where("model = ? AND channel_id NOT IN ?", modelName, flovaChannelIDs(channel.Id)).
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

// flovaChannelIDs lists every Flova bridge channel ("Flova Video", "Flova Video 2", ...)
// so startup routing keeps the operator-added remote bridges enabled.
func flovaChannelIDs(primary int) []int {
	ids := []int{primary}
	var extra []int
	if err := DB.Model(&Channel{}).Where("name LIKE ?", flovaChannelName+"%").Pluck("id", &extra).Error; err == nil {
		for _, id := range extra {
			if id != primary {
				ids = append(ids, id)
			}
		}
	}
	return ids
}
