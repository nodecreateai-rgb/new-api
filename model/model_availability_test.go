package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelAvailabilityTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	originalDB := DB
	DB = db
	require.NoError(t, db.AutoMigrate(&Model{}))

	t.Cleanup(func() {
		DB = originalDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestIsModelEnabledForRelay(t *testing.T) {
	db := setupModelAvailabilityTestDB(t)
	require.NoError(t, db.Create(&Model{ModelName: "enabled-model", Status: 1}).Error)
	require.NoError(t, db.Create(&Model{ModelName: "disabled-model", Status: 1}).Error)
	require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "disabled-model").Update("status", 0).Error)

	enabled, err := IsModelEnabledForRelay("enabled-model")
	require.NoError(t, err)
	require.True(t, enabled)

	enabled, err = IsModelEnabledForRelay("disabled-model")
	require.NoError(t, err)
	require.False(t, enabled)

	// Channels may expose custom models before marketplace metadata exists.
	enabled, err = IsModelEnabledForRelay("unregistered-custom-model")
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestFilterEnabledModelsForRelay(t *testing.T) {
	db := setupModelAvailabilityTestDB(t)
	require.NoError(t, db.Create(&Model{ModelName: "enabled-model", Status: 1}).Error)
	require.NoError(t, db.Create(&Model{ModelName: "disabled-model", Status: 1}).Error)
	require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "disabled-model").Update("status", 0).Error)

	filtered, err := FilterEnabledModelsForRelay([]string{"enabled-model", "disabled-model", "unregistered-custom-model"})
	require.NoError(t, err)
	require.Equal(t, []string{"enabled-model", "unregistered-custom-model"}, filtered)
}
