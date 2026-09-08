package model

import (
	"context"
	"embed"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/ClickHouse/clickhouse-go/v2"
	chdriver "gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

//go:embed clickhouse_primary_schema.sql
var clickHousePrimarySchemaFS embed.FS

var clickHousePrimarySchemaSQL string

func init() {
	b, err := clickHousePrimarySchemaFS.ReadFile("clickhouse_primary_schema.sql")
	if err == nil {
		clickHousePrimarySchemaSQL = string(b)
	}
}

var chIDSeq sync.Map // table -> *atomic.Int64

func openClickHouseGorm(dsn string, isLog bool) (*gorm.DB, error) {
	normalized := normalizeClickHouseDSN(dsn)
	opts, err := clickhouse.ParseDSN(normalized)
	if err != nil {
		return nil, fmt.Errorf("parse clickhouse dsn: %w", err)
	}
	if opts.Settings == nil {
		opts.Settings = clickhouse.Settings{}
	}
	opts.Settings["allow_experimental_lightweight_update"] = 1
	opts.Settings["apply_mutations_on_fly"] = 1

	sqlDB := clickhouse.OpenDB(opts)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	baseDialector := chdriver.New(chdriver.Config{
		Conn:                         sqlDB,
		DisableDatetimePrecision:     true,
		DontSupportRenameColumn:      true,
		DontSupportEmptyDefaultValue: true,
	})

	db, err := gorm.Open(baseDialector, &gorm.Config{
		PrepareStmt:                              false,
		SkipDefaultTransaction:                   true,
		DisableForeignKeyConstraintWhenMigrating: true,
		CreateBatchSize:                          100,
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	_ = db.Callback().Query().Before("gorm:query").Register("clickhouse:clear_for_update", clearClickHouseForUpdate)
	_ = db.Callback().Create().Before("gorm:create").Register("clickhouse:assign_id", assignClickHouseAutoID)


	if isLog {
		common.LogSqlType = common.DatabaseTypeClickHouse
		common.UsingClickHouse = true
		common.SysLog("using ClickHouse as log database")
	} else {
		common.UsingClickHouse = true
		common.UsingMySQL = false
		common.UsingPostgreSQL = false
		common.UsingSQLite = false
		if os.Getenv("LOG_SQL_DSN") == "" {
			common.LogSqlType = common.DatabaseTypeClickHouse
		}
		common.SysLog("using ClickHouse as primary database")
	}
	return db, nil
}

func clearClickHouseForUpdate(db *gorm.DB) {
	if db == nil || db.Statement == nil {
		return
	}
	db.Statement.Settings.Delete("gorm:query_option")
	// Also drop locking clauses if any were set via clause API.
	if db.Statement.Clauses != nil {
		delete(db.Statement.Clauses, "FOR")
		delete(db.Statement.Clauses, clause.Locking{}.Name())
	}
}

func assignClickHouseAutoID(db *gorm.DB) {
	if db == nil || db.Statement == nil || db.Statement.Schema == nil {
		return
	}
	field := db.Statement.Schema.PrioritizedPrimaryField
	if field == nil || !strings.EqualFold(field.DBName, "id") {
		return
	}
	rv := db.Statement.ReflectValue
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			assignOneClickHouseID(db, field, rv.Index(i))
		}
	default:
		assignOneClickHouseID(db, field, rv)
	}
}

func assignOneClickHouseID(db *gorm.DB, field *schema.Field, rv reflect.Value) {
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return
	}
	fv := field.ReflectValueOf(db.Statement.Context, rv)
	if !fv.IsValid() || !fv.CanSet() {
		return
	}
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fv.Int() != 0 {
			return
		}
		fv.SetInt(nextClickHouseTableID(db, db.Statement.Table))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if fv.Uint() != 0 {
			return
		}
		fv.SetUint(uint64(nextClickHouseTableID(db, db.Statement.Table)))
	}
}

func nextClickHouseTableID(db *gorm.DB, table string) int64 {
	table = strings.Trim(table, "`\"")
	if table == "" {
		return common.GetTimestamp()
	}
	v, _ := chIDSeq.LoadOrStore(table, &atomic.Int64{})
	seq := v.(*atomic.Int64)
	for {
		cur := seq.Load()
		if cur > 0 {
			return seq.Add(1)
		}
		// Seed from the current millisecond clock. Do NOT SELECT max(id) here:
		// running a query from the GORM create callback (or interleaved with an
		// INSERT on clickhouse-go native protocol) triggers
		// "Unexpected packet Query received from client" and drops log/task rows.
		seed := time.Now().UnixMilli()
		if seed <= 0 {
			seed = common.GetTimestamp()
		}
		if seq.CompareAndSwap(0, seed) {
			return seq.Add(1)
		}
	}
}

func quoteClickHouseIdent(name string) string {
	name = strings.ReplaceAll(name, "`", "")
	return "`" + name + "`"
}

func migrateClickHousePrimaryDB() error {
	common.SysLog("clickhouse primary: ensuring schema")
	schemaPath := strings.TrimSpace(os.Getenv("CLICKHOUSE_SCHEMA_FILE"))
	var sqlText string
	if schemaPath != "" {
		b, err := os.ReadFile(schemaPath)
		if err != nil {
			return err
		}
		sqlText = string(b)
	} else {
		sqlText = clickHousePrimarySchemaSQL
	}
	if strings.TrimSpace(sqlText) == "" {
		return fmt.Errorf("clickhouse primary schema SQL is empty")
	}
	// Run DDL on a fresh dedicated connection so GORM's pool is not disturbed.
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("clickhouse schema conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	for _, stmt := range splitSQLStatements(sqlText) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		up := strings.ToUpper(stmt)
		if strings.HasPrefix(up, "DROP DATABASE") || strings.HasPrefix(up, "CREATE DATABASE") {
			continue
		}
		if _, err := conn.ExecContext(context.Background(), stmt); err != nil {
			low := strings.ToLower(err.Error())
			if strings.Contains(low, "already exists") {
				continue
			}
			return fmt.Errorf("clickhouse schema exec failed: %w\nstmt: %s", err, truncate(stmt, 180))
		}
	}
	common.SysLog("clickhouse primary schema ready")
	return nil
}

func splitSQLStatements(sqlText string) []string {
	parts := strings.Split(sqlText, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
