package model

import (
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm/clause"
)

const claudeOpus55PublicModel = "anthropic/claude-opus-5-5"

const claudeChatNeutralName = "Claude Chat"

func ensureClaudeChatRouting() error {
	const modelsCSV = claudeOpus55PublicModel
	const mappingJSON = `{"anthropic/claude-opus-5-5":"anthropic/claude-opus-5-5"}`
	const groupsCSV = "default,vip,svip,vip1,vip2,vip3,vip6,vip7,vip8,vip9,llm-vip"
	baseURL := strings.TrimSpace(os.Getenv("CLAUDE_CHAT_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("RUNBIOS2API_BASE_URL"))
	}
	if baseURL == "" {
		baseURL = "http://runbios2api:8911"
	}
	key := strings.TrimSpace(os.Getenv("CLAUDE_CHAT_GATEWAY_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("RUNBIOS2API_GATEWAY_KEY"))
	}
	if key == "" {
		if keyFile := strings.TrimSpace(os.Getenv("CLAUDE_CHAT_GATEWAY_KEY_FILE")); keyFile != "" {
			if raw, err := os.ReadFile(keyFile); err == nil {
				key = strings.TrimSpace(string(raw))
			}
		}
	}
	if key == "" {
		key = strings.TrimSpace(os.Getenv("RB2API_API_KEY"))
	}

	publicModels := []string{claudeOpus55PublicModel}
	groups := []string{"default", "vip", "svip", "vip1", "vip2", "vip3", "vip6", "vip7", "vip8", "vip9", "llm-vip"}
	modelDescriptions := map[string]string{
		claudeOpus55PublicModel: "",
	}
	endpoint := `{"openai":{"path":"/v1/chat/completions","method":"POST"}}`
	if err := renameLegacyClaudeChatChannels(); err != nil {
		return err
	}
	if err := ensureNamedOpenAIChatRouting(claudeChatNeutralName, modelsCSV, mappingJSON, groupsCSV, baseURL, key, publicModels, groups, modelDescriptions, endpoint); err != nil {
		return err
	}
	return ensureClaudeChatVendor()
}

func renameLegacyClaudeChatChannels() error {
	var channels []Channel
	if err := DB.Find(&channels).Error; err != nil {
		return err
	}
	for _, ch := range channels {
		if !strings.Contains(strings.ToLower(ch.Name), "runbios") {
			continue
		}
		if common.UsingClickHouse {
			if err := DB.Exec(`ALTER TABLE channels UPDATE name = ? WHERE id = ?`, claudeChatNeutralName, ch.Id).Error; err != nil {
				return err
			}
			continue
		}
		if err := DB.Model(&Channel{}).Where("id = ?", ch.Id).Update("name", claudeChatNeutralName).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureClaudeChatVendor() error {
	vendorID, err := ensureNamedVendorID("Anthropic", "", "Claude.Color")
	if err != nil || vendorID <= 0 {
		return err
	}
	return DB.Unscoped().Model(&Model{}).Where("model_name = ?", claudeOpus55PublicModel).
		Update("vendor_id", vendorID).Error
}

func ensureLLMVipOnEnabledChannels() error {
	const group = "llm-vip"
	var channels []Channel
	if err := DB.Where("status = ?", common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return err
	}
	for i := range channels {
		channel := &channels[i]
		if err := ensureChannelHasGroup(channel, group); err != nil {
			return err
		}
		for _, modelName := range strings.Split(channel.Models, ",") {
			modelName = strings.TrimSpace(modelName)
			if modelName == "" {
				continue
			}
			if err := upsertEnabledAbility(*channel, group, modelName); err != nil {
				return err
			}
		}
	}
	InitChannelCache()
	return nil
}

func ensureChannelHasGroup(channel *Channel, group string) error {
	groups := strings.Split(channel.Group, ",")
	for _, existing := range groups {
		if strings.TrimSpace(existing) == group {
			return nil
		}
	}
	next := strings.TrimSpace(channel.Group)
	if next == "" {
		next = group
	} else {
		next = next + "," + group
	}
	channel.Group = next
	if common.UsingClickHouse {
		return DB.Exec(`ALTER TABLE channels UPDATE `+commonGroupCol+` = ? WHERE id = ?`, next, channel.Id).Error
	}
	return DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("group", next).Error
}

func upsertEnabledAbility(channel Channel, group, modelName string) error {
	priority := channel.Priority
	if priority == nil {
		zero := int64(0)
		priority = &zero
	}
	weight := uint64(channel.GetWeight())
	if common.UsingClickHouse {
		var count int64
		if err := DB.Model(&Ability{}).Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, modelName, channel.Id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return DB.Exec(
				`INSERT INTO abilities (`+commonGroupCol+`, model, channel_id, enabled, priority, weight, tag) VALUES (?, ?, ?, 1, ?, ?, '')`,
				group, modelName, channel.Id, *priority, weight,
			).Error
		}
		return DB.Model(&Ability{}).Where(commonGroupCol+" = ? AND model = ? AND channel_id = ?", group, modelName, channel.Id).
			Updates(map[string]any{"enabled": true, "priority": *priority, "weight": weight}).Error
	}
	ability := Ability{
		Group: group, Model: modelName, ChannelId: channel.Id,
		Enabled: channel.Status == common.ChannelStatusEnabled, Priority: priority, Weight: weight, Tag: channel.Tag,
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "group"}, {Name: "model"}, {Name: "channel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "priority", "weight", "tag"}),
	}).Create(&ability).Error
}
