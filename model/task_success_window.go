package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// TaskSuccessWindowRow counts finished tasks whose outcome falls in each window.
// A task finished 2 hours ago is included in 3h, 12h, and 24h. A task finished
// 20 hours ago is included only in 24h.
type TaskSuccessWindowRow struct {
	ModelName  string `gorm:"column:model_name"`
	Success3h  int64  `gorm:"column:success_3h"`
	Failure3h  int64  `gorm:"column:failure_3h"`
	Success12h int64  `gorm:"column:success_12h"`
	Failure12h int64  `gorm:"column:failure_12h"`
	Success24h int64  `gorm:"column:success_24h"`
	Failure24h int64  `gorm:"column:failure_24h"`
}

func GetTaskSuccessWindows(now int64) ([]TaskSuccessWindowRow, error) {
	start24 := now - 24*3600
	start12 := now - 12*3600
	start3 := now - 3*3600
	modelExpr := taskOriginModelExpr()

	var rows []TaskSuccessWindowRow
	err := DB.Table("tasks").
		Select(
			modelExpr+" as model_name, "+
				taskCountExpr("SUCCESS", start3)+" as success_3h, "+
				taskCountExpr("FAILURE", start3)+" as failure_3h, "+
				taskCountExpr("SUCCESS", start12)+" as success_12h, "+
				taskCountExpr("FAILURE", start12)+" as failure_12h, "+
				taskCountExpr("SUCCESS", start24)+" as success_24h, "+
				taskCountExpr("FAILURE", start24)+" as failure_24h",
		).
		Where("finish_time >= ? AND status IN ?", start24, []string{"SUCCESS", "FAILURE"}).
		Where(modelExpr + " <> ''").
		Group(modelExpr).
		Find(&rows).Error
	return rows, err
}

func taskOriginModelExpr() string {
	switch {
	case common.UsingClickHouse:
		return "JSONExtractString(ifNull(properties, ''), 'origin_model_name')"
	case common.UsingPostgreSQL:
		return "(properties->>'origin_model_name')"
	case common.UsingSQLite:
		return "json_extract(properties, '$.origin_model_name')"
	default:
		return "JSON_UNQUOTE(JSON_EXTRACT(properties, '$.origin_model_name'))"
	}
}

func taskCountExpr(status string, start int64) string {
	condition := fmt.Sprintf("status = '%s' AND finish_time >= %d", status, start)
	if common.UsingClickHouse {
		return "toInt64(countIf(" + condition + "))"
	}
	cast := "SIGNED"
	if common.UsingPostgreSQL {
		cast = "BIGINT"
	} else if common.UsingSQLite {
		cast = "INTEGER"
	}
	return "CAST(SUM(CASE WHEN " + condition + " THEN 1 ELSE 0 END) AS " + cast + ")"
}
