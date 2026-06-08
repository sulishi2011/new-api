package model

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	baseLogTableName = "logs"
	logTablePrefix   = "logs_"
)

var (
	ensuredLogTables sync.Map

	ErrLogCrossMonthQuery = errors.New("日志分表后最多支持跨 2 张连续月表查询，请缩小时间范围")
)

type logReadTableRange struct {
	TableName      string
	StartTimestamp int64
	EndTimestamp   int64
}

func createLog(log *Log) error {
	if log == nil {
		return errors.New("log is nil")
	}
	if log.CreatedAt <= 0 {
		log.CreatedAt = common.GetTimestamp()
	}
	assignLogID(log)
	tableName, err := ensureLogTableForTimestamp(log.CreatedAt)
	if err != nil {
		return err
	}
	return LOG_DB.Table(tableName).Create(log).Error
}

func logTableNameForTimestamp(timestamp int64) string {
	if timestamp <= 0 {
		timestamp = common.GetTimestamp()
	}
	return logTablePrefix + time.Unix(timestamp, 0).UTC().Format("200601")
}

func logTableNameForID(logID int64) string {
	if timestamp, ok := timestampFromGeneratedLogID(logID); ok {
		return logTableNameForTimestamp(timestamp)
	}
	return baseLogTableName
}

func resolveLogReadTable(startTimestamp int64, endTimestamp int64) (string, error) {
	ranges, err := resolveLogReadTableRanges(startTimestamp, endTimestamp)
	if err != nil {
		return "", err
	}
	if len(ranges) != 1 {
		return "", ErrLogCrossMonthQuery
	}
	return ranges[0].TableName, nil
}

func resolveLogReadTableForID(logID int64) string {
	tableName := logTableNameForID(logID)
	if tableName == baseLogTableName {
		return baseLogTableName
	}
	return existingLogReadTable(tableName)
}

func currentLogReadTable() string {
	return existingLogReadTable(logTableNameForTimestamp(common.GetTimestamp()))
}

func existingLogReadTable(tableName string) string {
	if tableName == "" {
		return baseLogTableName
	}
	if logReadDB().Migrator().HasTable(tableName) {
		return tableName
	}
	return baseLogTableName
}

func logReadTableExpr(tableName string) string {
	return quoteLogIdentifier(tableName) + " AS logs"
}

func logRawTableExpr(tableName string) string {
	return quoteLogIdentifier(tableName)
}

func logMonthKey(timestamp int64) string {
	if timestamp <= 0 {
		timestamp = common.GetTimestamp()
	}
	return time.Unix(timestamp, 0).UTC().Format("200601")
}

func logMonthStart(timestamp int64) time.Time {
	if timestamp <= 0 {
		timestamp = common.GetTimestamp()
	}
	t := time.Unix(timestamp, 0).UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func buildBoundedLogReadTableRanges(startTimestamp int64, endTimestamp int64) ([]logReadTableRange, error) {
	if endTimestamp < startTimestamp {
		return []logReadTableRange{{
			TableName:      existingLogReadTable(logTableNameForTimestamp(startTimestamp)),
			StartTimestamp: startTimestamp,
			EndTimestamp:   endTimestamp,
		}}, nil
	}

	startMonth := logMonthStart(startTimestamp)
	endMonth := logMonthStart(endTimestamp)
	ranges := make([]logReadTableRange, 0, 2)
	for monthStart := startMonth; !monthStart.After(endMonth); monthStart = monthStart.AddDate(0, 1, 0) {
		if len(ranges) >= 2 {
			return nil, ErrLogCrossMonthQuery
		}
		nextMonthStart := monthStart.AddDate(0, 1, 0).Unix()
		rangeStart := monthStart.Unix()
		if rangeStart < startTimestamp {
			rangeStart = startTimestamp
		}
		rangeEnd := nextMonthStart - 1
		if rangeEnd > endTimestamp {
			rangeEnd = endTimestamp
		}
		tableName := existingLogReadTable(logTablePrefix + monthStart.Format("200601"))
		ranges = append(ranges, logReadTableRange{
			TableName:      tableName,
			StartTimestamp: rangeStart,
			EndTimestamp:   rangeEnd,
		})
	}
	return ranges, nil
}

func resolveLogReadTableRanges(startTimestamp int64, endTimestamp int64) ([]logReadTableRange, error) {
	now := common.GetTimestamp()
	switch {
	case startTimestamp == 0 && endTimestamp == 0:
		return []logReadTableRange{{
			TableName: existingLogReadTable(logTableNameForTimestamp(now)),
		}}, nil
	case startTimestamp != 0 && endTimestamp != 0:
		return buildBoundedLogReadTableRanges(startTimestamp, endTimestamp)
	case startTimestamp != 0:
		effectiveEnd := now
		if startTimestamp > effectiveEnd {
			effectiveEnd = startTimestamp
		}
		ranges, err := buildBoundedLogReadTableRanges(startTimestamp, effectiveEnd)
		if err != nil {
			return nil, err
		}
		if len(ranges) > 0 {
			ranges[len(ranges)-1].EndTimestamp = 0
		}
		return ranges, nil
	default:
		if logMonthKey(endTimestamp) != logMonthKey(now) {
			return nil, ErrLogCrossMonthQuery
		}
		return []logReadTableRange{{
			TableName:    existingLogReadTable(logTableNameForTimestamp(endTimestamp)),
			EndTimestamp: endTimestamp,
		}}, nil
	}
}

func ensureLogTableForTimestamp(timestamp int64) (string, error) {
	tableName := logTableNameForTimestamp(timestamp)
	if _, ok := ensuredLogTables.Load(tableName); ok {
		if LOG_DB.Migrator().HasTable(tableName) {
			return tableName, nil
		}
	}
	if LOG_DB.Migrator().HasTable(tableName) {
		if err := migrateLogShardIndexes(LOG_DB, tableName); err != nil {
			return "", err
		}
		ensuredLogTables.Store(tableName, struct{}{})
		return tableName, nil
	}

	var err error
	switch logDatabaseType() {
	case common.DatabaseTypePostgreSQL:
		err = createPostgresLogTable(tableName)
	case common.DatabaseTypeMySQL:
		err = LOG_DB.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s LIKE %s", quoteLogIdentifier(tableName), quoteLogIdentifier(baseLogTableName))).Error
	default:
		err = createSQLiteLogTable(tableName)
	}
	if err != nil {
		return "", err
	}
	if err := migrateLogShardIndexes(LOG_DB, tableName); err != nil {
		return "", err
	}
	ensuredLogTables.Store(tableName, struct{}{})
	return tableName, nil
}

func createPostgresLogTable(tableName string) error {
	quotedTable := quoteLogIdentifier(tableName)
	quotedBase := quoteLogIdentifier(baseLogTableName)
	if err := LOG_DB.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (LIKE %s INCLUDING DEFAULTS)", quotedTable, quotedBase)).Error; err != nil {
		return err
	}
	if err := ensurePostgresBigintColumn(LOG_DB, tableName, "id"); err != nil {
		return err
	}
	pkName := tableName + "_pkey"
	if err := LOG_DB.Exec(fmt.Sprintf(`DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1 FROM pg_constraint WHERE conname = %s
	) THEN
		ALTER TABLE %s ADD CONSTRAINT %s PRIMARY KEY ("id");
	END IF;
END $$;`, quoteLogLiteral(pkName), quotedTable, quoteLogIdentifier(pkName))).Error; err != nil {
		return err
	}
	for _, stmt := range postgresLogIndexStatements(tableName) {
		if err := LOG_DB.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func createSQLiteLogTable(tableName string) error {
	quotedTable := quoteLogIdentifier(tableName)
	quotedBase := quoteLogIdentifier(baseLogTableName)
	if err := LOG_DB.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s AS SELECT * FROM %s WHERE 0", quotedTable, quotedBase)).Error; err != nil {
		return err
	}
	for _, stmt := range sqliteLogIndexStatements(tableName) {
		if err := LOG_DB.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func postgresLogIndexStatements(tableName string) []string {
	return logShardIndexStatements(tableName)
}

func sqliteLogIndexStatements(tableName string) []string {
	return logShardIndexStatements(tableName)
}

func logShardIndexStatements(tableName string) []string {
	stmts := make([]string, 0, len(logShardTargetIndexSpecs()))
	for _, spec := range logShardTargetIndexSpecs() {
		stmts = append(stmts, buildCreateLogTableIndexSQL(tableName, logShardIndexName(tableName, spec.Suffix), spec.Columns, true))
	}
	return stmts
}

func logDatabaseType() string {
	if os.Getenv("LOG_SQL_DSN") != "" {
		return common.LogSqlType
	}
	switch {
	case common.UsingPostgreSQL:
		return common.DatabaseTypePostgreSQL
	case common.UsingMySQL:
		return common.DatabaseTypeMySQL
	default:
		return common.DatabaseTypeSQLite
	}
}

func quoteLogIdentifier(identifier string) string {
	if logDatabaseType() == common.DatabaseTypePostgreSQL {
		return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
	}
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

func quoteLogLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func ensurePostgresBigintColumn(db *gorm.DB, tableName string, columnName string) error {
	return db.Exec(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE BIGINT", quotePostgresIdentifier(tableName), quotePostgresIdentifier(columnName))).Error
}

func quotePostgresIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
