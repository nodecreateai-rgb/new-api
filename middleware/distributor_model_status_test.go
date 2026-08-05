package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDistributorModelStatusTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Model{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestDistributeRejectsDisabledModelBeforeChannelSelection(t *testing.T) {
	db := setupDistributorModelStatusTestDB(t)
	require.NoError(t, db.Create(&model.Model{ModelName: "disabled-model", Status: 1}).Error)
	require.NoError(t, db.Model(&model.Model{}).Where("model_name = ?", "disabled-model").Update("status", 0).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"disabled-model","prompt":"test"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	Distribute()(ctx)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"code":"model_not_found"`)
	require.Contains(t, recorder.Body.String(), "disabled")
}
