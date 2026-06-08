package model

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var commonGroupCol string
var commonKeyCol string
var commonTrueVal string
var commonFalseVal string

var logKeyCol string
var logGroupCol string

func initCol() {
	// init common column names
	if common.UsingPostgreSQL {
		commonGroupCol = `"group"`
		commonKeyCol = `"key"`
		commonTrueVal = "true"
		commonFalseVal = "false"
	} else {
		commonGroupCol = "`group`"
		commonKeyCol = "`key`"
		commonTrueVal = "1"
		commonFalseVal = "0"
	}
	if os.Getenv("LOG_SQL_DSN") != "" {
		switch common.LogSqlType {
		case common.DatabaseTypePostgreSQL:
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		default:
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	} else {
		// LOG_SQL_DSN 为空时，日志数据库与主数据库相同
		if common.UsingPostgreSQL {
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		} else {
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	}
	// log sql type and database type
	//common.SysLog("Using Log SQL Type: " + common.LogSqlType)
}

var DB *gorm.DB

var READ_DB *gorm.DB

var LOG_DB *gorm.DB

var LOG_READ_DB *gorm.DB

func configureSQLPool(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
	sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
	sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))
	return nil
}

func chooseReadDB(envName string) (*gorm.DB, error) {
	dsn := os.Getenv(envName)
	if dsn == "" {
		return nil, nil
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		common.SysLog("using PostgreSQL as read database for " + envName)
		return gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		}), &gorm.Config{
			PrepareStmt: true,
		})
	}
	if strings.HasPrefix(dsn, "local") {
		common.SysLog("using SQLite as read database for " + envName)
		return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
			PrepareStmt: true,
		})
	}
	common.SysLog("using MySQL as read database for " + envName)
	if !strings.Contains(dsn, "parseTime") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}
	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		PrepareStmt: true,
	})
}

func initReadDB(envName string, fallback *gorm.DB) (*gorm.DB, error) {
	readDB, err := chooseReadDB(envName)
	if err != nil {
		return nil, err
	}
	if readDB == nil {
		return fallback, nil
	}
	if common.DebugEnabled {
		readDB = readDB.Debug()
	}
	if err := configureSQLPool(readDB); err != nil {
		return nil, err
	}
	return readDB, nil
}

func readDB() *gorm.DB {
	if READ_DB != nil {
		return READ_DB
	}
	return DB
}

func logReadDB() *gorm.DB {
	if LOG_READ_DB != nil {
		return LOG_READ_DB
	}
	return LOG_DB
}

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != common.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		common.SysLog("no user exists, create a root user for you: username is root, password is 123456")
		hashedPassword, err := common.Password2Hash("123456")
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
		DB.Create(&rootUser)
	}
	return nil
}

func CheckSetup() {
	setup := GetSetup()
	if setup == nil {
		// No setup record exists, check if we have a root user
		if RootUserExists() {
			common.SysLog("system is not initialized, but root user exists")
			// Create setup record
			newSetup := Setup{
				Version:       common.Version,
				InitializedAt: time.Now().Unix(),
			}
			err := DB.Create(&newSetup).Error
			if err != nil {
				common.SysLog("failed to create setup record: " + err.Error())
			}
			constant.Setup = true
		} else {
			common.SysLog("system is not initialized and no root user exists")
			constant.Setup = false
		}
	} else {
		// Setup record exists, system is initialized
		common.SysLog("system is already initialized at: " + time.Unix(setup.InitializedAt, 0).String())
		constant.Setup = true
	}
}

func chooseDB(envName string, isLog bool) (*gorm.DB, error) {
	defer func() {
		initCol()
	}()
	dsn := os.Getenv(envName)
	if dsn != "" {
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			// Use PostgreSQL
			common.SysLog("using PostgreSQL as database")
			if !isLog {
				common.UsingPostgreSQL = true
			} else {
				common.LogSqlType = common.DatabaseTypePostgreSQL
			}
			return gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true, // disables implicit prepared statement usage
			}), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		if strings.HasPrefix(dsn, "local") {
			common.SysLog("SQL_DSN not set, using SQLite as database")
			if !isLog {
				common.UsingSQLite = true
			} else {
				common.LogSqlType = common.DatabaseTypeSQLite
			}
			return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		// Use MySQL
		common.SysLog("using MySQL as database")
		// check parseTime
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		if !isLog {
			common.UsingMySQL = true
		} else {
			common.LogSqlType = common.DatabaseTypeMySQL
		}
		return gorm.Open(mysql.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
	}
	// Use SQLite
	common.SysLog("SQL_DSN not set, using SQLite as database")
	common.UsingSQLite = true
	return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func InitDB() (err error) {
	db, err := chooseDB("SQL_DSN", false)
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		DB = db
		// MySQL charset/collation startup check: ensure Chinese-capable charset
		if common.UsingMySQL {
			if err := checkMySQLChineseSupport(DB); err != nil {
				panic(err)
			}
		}
		if err := configureSQLPool(DB); err != nil {
			return err
		}
		READ_DB, err = initReadDB("SQL_READ_DSN", DB)
		if err != nil {
			return err
		}

		if !common.IsMasterNode {
			return nil
		}
		if common.UsingMySQL {
			//_, _ = sqlDB.Exec("ALTER TABLE channels MODIFY model_mapping TEXT;") // TODO: delete this line when most users have upgraded
		}
		common.SysLog("database migration started")
		err = migrateDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func InitLogDB() (err error) {
	if os.Getenv("LOG_SQL_DSN") == "" {
		LOG_DB = DB
		LOG_READ_DB, err = initReadDB("LOG_SQL_READ_DSN", LOG_DB)
		if err != nil {
			return err
		}
		if common.IsMasterNode {
			_, err = ensureLogTableForTimestamp(common.GetTimestamp())
		}
		return err
	}
	db, err := chooseDB("LOG_SQL_DSN", true)
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		LOG_DB = db
		// If log DB is MySQL, also ensure Chinese-capable charset
		if common.LogSqlType == common.DatabaseTypeMySQL {
			if err := checkMySQLChineseSupport(LOG_DB); err != nil {
				panic(err)
			}
		}
		if err := configureSQLPool(LOG_DB); err != nil {
			return err
		}
		LOG_READ_DB, err = initReadDB("LOG_SQL_READ_DSN", LOG_DB)
		if err != nil {
			return err
		}

		if !common.IsMasterNode {
			return nil
		}
		common.SysLog("database migration started")
		err = migrateLOGDB()
		if err != nil {
			return err
		}
		_, err = ensureLogTableForTimestamp(common.GetTimestamp())
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func migrateDB() error {
	// Migrate price_amount column from float/double to decimal for existing tables
	migrateSubscriptionPlanPriceAmount()
	// Migrate model_limits column from varchar to text for existing tables
	if err := migrateTokenModelLimitsToText(); err != nil {
		return err
	}

	err := DB.AutoMigrate(
		&Channel{},
		&Token{},
		&User{},
		&PasskeyCredential{},
		&Option{},
		&Redemption{},
		&Ability{},
		&ProviderKey{},
		&Log{},
		&LogTrace{},
		&Midjourney{},
		&TopUp{},
		&QuotaData{},
		&Task{},
		&Model{},
		&Vendor{},
		&VendorProfile{},
		&UsageLedger{},
		&UsageAggregateHourly{},
		&UsageAggregateDaily{},
		&UsageAggregationJob{},
		&PrefillGroup{},
		&Setup{},
		&TwoFA{},
		&TwoFABackupCode{},
		&Checkin{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionPreConsumeRecord{},
		&CustomOAuthProvider{},
		&UserOAuthBinding{},
		&PerfMetric{},
	)
	if err != nil {
		return err
	}
	if err := migrateLogIndexes(DB); err != nil {
		return err
	}
	if common.UsingPostgreSQL {
		if err := ensurePostgresBigintColumn(DB, "log_traces", "log_id"); err != nil {
			return err
		}
		if err := ensurePostgresBigintColumn(DB, "usage_ledgers", "newapi_log_id"); err != nil {
			return err
		}
	}
	if common.UsingSQLite {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
	} else {
		if err := DB.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	return nil
}

func migrateDBFast() error {

	var wg sync.WaitGroup

	migrations := []struct {
		model interface{}
		name  string
	}{
		{&Channel{}, "Channel"},
		{&Token{}, "Token"},
		{&User{}, "User"},
		{&PasskeyCredential{}, "PasskeyCredential"},
		{&Option{}, "Option"},
		{&Redemption{}, "Redemption"},
		{&Ability{}, "Ability"},
		{&ProviderKey{}, "ProviderKey"},
		{&Log{}, "Log"},
		{&LogTrace{}, "LogTrace"},
		{&Midjourney{}, "Midjourney"},
		{&TopUp{}, "TopUp"},
		{&QuotaData{}, "QuotaData"},
		{&Task{}, "Task"},
		{&Model{}, "Model"},
		{&Vendor{}, "Vendor"},
		{&VendorProfile{}, "VendorProfile"},
		{&UsageLedger{}, "UsageLedger"},
		{&UsageAggregateHourly{}, "UsageAggregateHourly"},
		{&UsageAggregateDaily{}, "UsageAggregateDaily"},
		{&UsageAggregationJob{}, "UsageAggregationJob"},
		{&PrefillGroup{}, "PrefillGroup"},
		{&Setup{}, "Setup"},
		{&TwoFA{}, "TwoFA"},
		{&TwoFABackupCode{}, "TwoFABackupCode"},
		{&Checkin{}, "Checkin"},
		{&SubscriptionOrder{}, "SubscriptionOrder"},
		{&UserSubscription{}, "UserSubscription"},
		{&SubscriptionPreConsumeRecord{}, "SubscriptionPreConsumeRecord"},
		{&CustomOAuthProvider{}, "CustomOAuthProvider"},
		{&UserOAuthBinding{}, "UserOAuthBinding"},
		{&PerfMetric{}, "PerfMetric"},
	}
	// 动态计算migration数量，确保errChan缓冲区足够大
	errChan := make(chan error, len(migrations))

	for _, m := range migrations {
		wg.Add(1)
		go func(model interface{}, name string) {
			defer wg.Done()
			if err := DB.AutoMigrate(model); err != nil {
				errChan <- fmt.Errorf("failed to migrate %s: %v", name, err)
			}
		}(m.model, m.name)
	}

	// Wait for all migrations to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	if err := migrateLogIndexes(DB); err != nil {
		return err
	}
	if common.UsingPostgreSQL {
		if err := ensurePostgresBigintColumn(DB, "log_traces", "log_id"); err != nil {
			return err
		}
		if err := ensurePostgresBigintColumn(DB, "usage_ledgers", "newapi_log_id"); err != nil {
			return err
		}
	}
	if common.UsingSQLite {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
	} else {
		if err := DB.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	common.SysLog("database migrated")
	return nil
}

func migrateLOGDB() error {
	var err error
	if err = LOG_DB.AutoMigrate(&ProviderKey{}); err != nil {
		return err
	}
	if err = LOG_DB.AutoMigrate(&Log{}); err != nil {
		return err
	}
	if err = LOG_DB.AutoMigrate(&LogTrace{}); err != nil {
		return err
	}
	if err = migrateLogIndexes(LOG_DB); err != nil {
		return err
	}
	if logDatabaseType() == common.DatabaseTypePostgreSQL {
		if err := ensurePostgresBigintColumn(LOG_DB, "log_traces", "log_id"); err != nil {
			return err
		}
	}
	return nil
}

func migrateLogIndexes(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if db.Migrator().HasTable(&Log{}) {
		if err := migrateBaseLogIndexes(db); err != nil {
			return err
		}
	}
	tables, err := db.Migrator().GetTables()
	if err != nil {
		return err
	}
	for _, tableName := range tables {
		if !isMonthlyLogTable(tableName) {
			continue
		}
		if err := migrateLogShardIndexes(db, tableName); err != nil {
			return err
		}
	}
	return nil
}

func migrateBaseLogIndexes(db *gorm.DB) error {
	if err := ensureLogIndexColumns(db, "idx_created_at_id", []string{"created_at", "id"}); err != nil {
		return err
	}

	for _, indexName := range []string{
		"idx_logs_type_created_id",
		"idx_logs_channel_created_id",
		"idx_logs_vendor_profile_created",
		"idx_logs_biz_line_scene_created",
	} {
		if err := createLogIndexIfMissing(db, indexName); err != nil {
			return err
		}
	}

	for _, indexName := range []string{
		"idx_user_id_id",
		"idx_created_at_type",
		"idx_logs_channel_type_created_id",
		"index_username_model_name",
	} {
		if err := dropLogIndexIfExists(db, indexName); err != nil {
			return err
		}
	}

	return nil
}

func migrateLogShardIndexes(db *gorm.DB, tableName string) error {
	if db == nil || tableName == "" || !db.Migrator().HasTable(tableName) {
		return nil
	}
	for _, spec := range logShardTargetIndexSpecs() {
		indexName := logShardIndexNameForDB(db, tableName, spec.Suffix)
		if err := ensureLogTableIndexColumns(db, tableName, indexName, spec.Columns); err != nil {
			return err
		}
	}
	for _, indexName := range logShardObsoleteIndexNames(db, tableName) {
		if err := dropLogTableIndexIfExists(db, tableName, indexName); err != nil {
			return err
		}
	}
	return nil
}

func createLogIndexIfMissing(db *gorm.DB, indexName string) error {
	if db.Migrator().HasIndex(&Log{}, indexName) {
		return nil
	}
	if err := db.Migrator().CreateIndex(&Log{}, indexName); err != nil {
		return fmt.Errorf("failed to create logs index %s: %w", indexName, err)
	}
	return nil
}

func dropLogIndexIfExists(db *gorm.DB, indexName string) error {
	if !db.Migrator().HasIndex(&Log{}, indexName) {
		return nil
	}
	if err := db.Migrator().DropIndex(&Log{}, indexName); err != nil {
		return fmt.Errorf("failed to drop logs index %s: %w", indexName, err)
	}
	return nil
}

func ensureLogTableIndexColumns(db *gorm.DB, tableName string, indexName string, expectedColumns []string) error {
	columns, err := getLogIndexColumns(db, tableName, indexName)
	if err != nil {
		return fmt.Errorf("failed to inspect logs index %s on %s: %w", indexName, tableName, err)
	}
	if sameColumns(columns, expectedColumns) {
		return nil
	}
	if len(columns) > 0 {
		if err := dropLogTableIndexIfExists(db, tableName, indexName); err != nil {
			return err
		}
	}
	if err := createLogTableIndex(db, tableName, indexName, expectedColumns); err != nil {
		return fmt.Errorf("failed to create logs index %s on %s: %w", indexName, tableName, err)
	}
	return nil
}

func createLogTableIndex(db *gorm.DB, tableName string, indexName string, columns []string) error {
	return db.Exec(buildCreateLogTableIndexSQLForDB(db, tableName, indexName, columns, false)).Error
}

func dropLogTableIndexIfExists(db *gorm.DB, tableName string, indexName string) error {
	columns, err := getLogIndexColumns(db, tableName, indexName)
	if err != nil {
		return fmt.Errorf("failed to inspect logs index %s on %s: %w", indexName, tableName, err)
	}
	if len(columns) == 0 {
		return nil
	}
	if err := db.Exec(buildDropLogTableIndexSQLForDB(db, tableName, indexName)).Error; err != nil {
		return fmt.Errorf("failed to drop logs index %s on %s: %w", indexName, tableName, err)
	}
	return nil
}

func ensureLogIndexColumns(db *gorm.DB, indexName string, expectedColumns []string) error {
	if db.Migrator().HasIndex(&Log{}, indexName) {
		columns, err := getLogIndexColumns(db, "logs", indexName)
		if err != nil {
			return fmt.Errorf("failed to inspect logs index %s: %w", indexName, err)
		}
		if sameColumns(columns, expectedColumns) {
			return nil
		}
		if err := db.Migrator().DropIndex(&Log{}, indexName); err != nil {
			return fmt.Errorf("failed to rebuild logs index %s: %w", indexName, err)
		}
	}
	if err := db.Migrator().CreateIndex(&Log{}, indexName); err != nil {
		return fmt.Errorf("failed to create logs index %s: %w", indexName, err)
	}
	return nil
}

type logIndexSpec struct {
	Suffix  string
	Columns []string
}

func logShardTargetIndexSpecs() []logIndexSpec {
	return []logIndexSpec{
		{Suffix: "created_at_id", Columns: []string{"created_at", "id"}},
		{Suffix: "type_created_id", Columns: []string{"type", "created_at", "id"}},
		{Suffix: "channel_created_id", Columns: []string{"channel_id", "created_at", "id"}},
		{Suffix: "vendor_profile_created", Columns: []string{"vendor_profile_id", "created_at"}},
		{Suffix: "biz_line_scene_created", Columns: []string{"biz_line", "biz_scene", "created_at"}},
		{Suffix: "request_id", Columns: []string{"request_id"}},
		{Suffix: "external_request_id", Columns: []string{"external_request_id"}},
		{Suffix: "upstream_request_id", Columns: []string{"upstream_request_id"}},
		{Suffix: "provider_key_id", Columns: []string{"provider_key_id"}},
		{Suffix: "token_id", Columns: []string{"token_id"}},
		{Suffix: "group_created", Columns: []string{"group", "created_at"}},
		{Suffix: "model_name", Columns: []string{"model_name"}},
		{Suffix: "username", Columns: []string{"username"}},
		{Suffix: "token_name", Columns: []string{"token_name"}},
	}
}

func logShardObsoleteIndexNames(db *gorm.DB, tableName string) []string {
	names := []string{
		"idx_user_id_id",
		"idx_created_at_type",
		"idx_logs_channel_type_created_id",
		"index_username_model_name",
	}
	for _, suffix := range []string{
		"user_created_id",
		"created_at_type",
		"channel_type_created_id",
		"username_model_name",
	} {
		names = append(names, logShardIndexNameForDB(db, tableName, suffix))
	}
	return names
}

func logShardIndexName(tableName string, suffix string) string {
	if logDatabaseType() == common.DatabaseTypeMySQL {
		return baseLogIndexNameForSuffix(suffix)
	}
	return "idx_" + tableName + "_" + suffix
}

func logShardIndexNameForDB(db *gorm.DB, tableName string, suffix string) string {
	if db != nil && db.Dialector.Name() == "mysql" {
		return baseLogIndexNameForSuffix(suffix)
	}
	return "idx_" + tableName + "_" + suffix
}

func baseLogIndexNameForSuffix(suffix string) string {
	switch suffix {
	case "created_at_id":
		return "idx_created_at_id"
	case "type_created_id":
		return "idx_logs_type_created_id"
	case "channel_created_id":
		return "idx_logs_channel_created_id"
	case "vendor_profile_created":
		return "idx_logs_vendor_profile_created"
	case "biz_line_scene_created":
		return "idx_logs_biz_line_scene_created"
	case "request_id":
		return "idx_logs_request_id"
	case "external_request_id":
		return "idx_logs_external_request_id"
	case "upstream_request_id":
		return "idx_logs_upstream_request_id"
	case "provider_key_id":
		return "idx_logs_provider_key_id"
	case "token_id":
		return "idx_logs_token_id"
	case "group_created":
		return "idx_logs_group_created"
	case "model_name":
		return "idx_logs_model_name"
	case "username":
		return "idx_logs_username"
	case "token_name":
		return "idx_logs_token_name"
	default:
		return "idx_logs_" + suffix
	}
}

func isMonthlyLogTable(tableName string) bool {
	if !strings.HasPrefix(tableName, logTablePrefix) {
		return false
	}
	suffix := strings.TrimPrefix(tableName, logTablePrefix)
	if len(suffix) != len("200601") {
		return false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func buildCreateLogTableIndexSQL(tableName string, indexName string, columns []string, ifNotExists bool) string {
	return buildCreateLogTableIndexSQLForType(logDatabaseType(), tableName, indexName, columns, ifNotExists)
}

func buildCreateLogTableIndexSQLForDB(db *gorm.DB, tableName string, indexName string, columns []string, ifNotExists bool) string {
	return buildCreateLogTableIndexSQLForType(logDatabaseTypeForDB(db), tableName, indexName, columns, ifNotExists)
}

func buildCreateLogTableIndexSQLForType(databaseType string, tableName string, indexName string, columns []string, ifNotExists bool) string {
	existsClause := ""
	if ifNotExists && databaseType != common.DatabaseTypeMySQL {
		existsClause = " IF NOT EXISTS"
	}
	return fmt.Sprintf(
		"CREATE INDEX%s %s ON %s (%s)",
		existsClause,
		quoteLogIdentifierForType(databaseType, indexName),
		quoteLogIdentifierForType(databaseType, tableName),
		quoteLogColumnListForType(databaseType, columns),
	)
}

func buildDropLogTableIndexSQLForDB(db *gorm.DB, tableName string, indexName string) string {
	databaseType := logDatabaseTypeForDB(db)
	if databaseType == common.DatabaseTypeMySQL {
		return fmt.Sprintf("DROP INDEX %s ON %s", quoteLogIdentifierForType(databaseType, indexName), quoteLogIdentifierForType(databaseType, tableName))
	}
	return fmt.Sprintf("DROP INDEX %s", quoteLogIdentifierForType(databaseType, indexName))
}

func quoteLogColumnListForType(databaseType string, columns []string) string {
	quoted := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted = append(quoted, quoteLogIdentifierForType(databaseType, column))
	}
	return strings.Join(quoted, ", ")
}

func quoteLogIdentifierForType(databaseType string, identifier string) string {
	if databaseType == common.DatabaseTypePostgreSQL {
		return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
	}
	return "`" + strings.ReplaceAll(identifier, "`", "``") + "`"
}

func logDatabaseTypeForDB(db *gorm.DB) string {
	if db == nil {
		return logDatabaseType()
	}
	switch db.Dialector.Name() {
	case "postgres":
		return common.DatabaseTypePostgreSQL
	case "mysql":
		return common.DatabaseTypeMySQL
	default:
		return common.DatabaseTypeSQLite
	}
}

type indexColumn struct {
	ColumnName string `gorm:"column:column_name"`
	Name       string `gorm:"column:name"`
}

func getLogIndexColumns(db *gorm.DB, tableName string, indexName string) ([]string, error) {
	var rows []indexColumn

	switch db.Dialector.Name() {
	case "mysql":
		err := db.Raw(`SELECT column_name FROM information_schema.statistics
			WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?
			ORDER BY seq_in_index`, tableName, indexName).Scan(&rows).Error
		return indexColumnNames(rows), err
	case "postgres":
		err := db.Raw(`SELECT a.attname AS column_name
			FROM pg_class t
			JOIN pg_namespace n ON n.oid = t.relnamespace
			JOIN pg_index i ON t.oid = i.indrelid
			JOIN pg_class ix ON ix.oid = i.indexrelid
			JOIN unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord) ON true
			JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
			WHERE n.nspname = current_schema() AND t.relname = ? AND ix.relname = ?
			ORDER BY k.ord`, tableName, indexName).Scan(&rows).Error
		return indexColumnNames(rows), err
	case "sqlite":
		err := db.Raw("PRAGMA index_info(" + quoteSQLiteIdentifier(indexName) + ")").Scan(&rows).Error
		return indexColumnNames(rows), err
	default:
		indexes, err := db.Migrator().GetIndexes(&Log{})
		if err != nil {
			return nil, err
		}
		for _, index := range indexes {
			if index.Name() == indexName {
				return index.Columns(), nil
			}
		}
		return nil, nil
	}
}

func indexColumnNames(rows []indexColumn) []string {
	columns := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ColumnName != "" {
			columns = append(columns, row.ColumnName)
			continue
		}
		columns = append(columns, row.Name)
	}
	return columns
}

func sameColumns(columns []string, expected []string) bool {
	if len(columns) != len(expected) {
		return false
	}
	for i := range columns {
		if !strings.EqualFold(columns[i], expected[i]) {
			return false
		}
	}
	return true
}

func quoteSQLiteIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

type sqliteColumnDef struct {
	Name string
	DDL  string
}

func ensureSubscriptionPlanTableSQLite() error {
	if !common.UsingSQLite {
		return nil
	}
	tableName := "subscription_plans"
	if !DB.Migrator().HasTable(tableName) {
		createSQL := `CREATE TABLE ` + "`" + tableName + "`" + ` (
` + "`id`" + ` integer,
` + "`title`" + ` varchar(128) NOT NULL,
` + "`subtitle`" + ` varchar(255) DEFAULT '',
` + "`price_amount`" + ` decimal(10,6) NOT NULL,
` + "`currency`" + ` varchar(8) NOT NULL DEFAULT 'USD',
` + "`duration_unit`" + ` varchar(16) NOT NULL DEFAULT 'month',
` + "`duration_value`" + ` integer NOT NULL DEFAULT 1,
` + "`custom_seconds`" + ` bigint NOT NULL DEFAULT 0,
` + "`enabled`" + ` numeric DEFAULT 1,
` + "`sort_order`" + ` integer DEFAULT 0,
` + "`allow_balance_pay`" + ` numeric DEFAULT 1,
` + "`stripe_price_id`" + ` varchar(128) DEFAULT '',
` + "`creem_product_id`" + ` varchar(128) DEFAULT '',
` + "`waffo_pancake_product_id`" + ` varchar(128) DEFAULT '',
` + "`max_purchase_per_user`" + ` integer DEFAULT 0,
` + "`upgrade_group`" + ` varchar(64) DEFAULT '',
` + "`total_amount`" + ` bigint NOT NULL DEFAULT 0,
` + "`quota_reset_period`" + ` varchar(16) DEFAULT 'never',
` + "`quota_reset_custom_seconds`" + ` bigint DEFAULT 0,
` + "`created_at`" + ` bigint,
` + "`updated_at`" + ` bigint,
PRIMARY KEY (` + "`id`" + `)
)`
		return DB.Exec(createSQL).Error
	}
	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := DB.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		existing[c.Name] = struct{}{}
	}
	required := []sqliteColumnDef{
		{Name: "title", DDL: "`title` varchar(128) NOT NULL"},
		{Name: "subtitle", DDL: "`subtitle` varchar(255) DEFAULT ''"},
		{Name: "price_amount", DDL: "`price_amount` decimal(10,6) NOT NULL"},
		{Name: "currency", DDL: "`currency` varchar(8) NOT NULL DEFAULT 'USD'"},
		{Name: "duration_unit", DDL: "`duration_unit` varchar(16) NOT NULL DEFAULT 'month'"},
		{Name: "duration_value", DDL: "`duration_value` integer NOT NULL DEFAULT 1"},
		{Name: "custom_seconds", DDL: "`custom_seconds` bigint NOT NULL DEFAULT 0"},
		{Name: "enabled", DDL: "`enabled` numeric DEFAULT 1"},
		{Name: "sort_order", DDL: "`sort_order` integer DEFAULT 0"},
		{Name: "allow_balance_pay", DDL: "`allow_balance_pay` numeric DEFAULT 1"},
		{Name: "stripe_price_id", DDL: "`stripe_price_id` varchar(128) DEFAULT ''"},
		{Name: "creem_product_id", DDL: "`creem_product_id` varchar(128) DEFAULT ''"},
		{Name: "waffo_pancake_product_id", DDL: "`waffo_pancake_product_id` varchar(128) DEFAULT ''"},
		{Name: "max_purchase_per_user", DDL: "`max_purchase_per_user` integer DEFAULT 0"},
		{Name: "upgrade_group", DDL: "`upgrade_group` varchar(64) DEFAULT ''"},
		{Name: "total_amount", DDL: "`total_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "quota_reset_period", DDL: "`quota_reset_period` varchar(16) DEFAULT 'never'"},
		{Name: "quota_reset_custom_seconds", DDL: "`quota_reset_custom_seconds` bigint DEFAULT 0"},
		{Name: "created_at", DDL: "`created_at` bigint"},
		{Name: "updated_at", DDL: "`updated_at` bigint"},
	}
	for _, col := range required {
		if _, ok := existing[col.Name]; ok {
			continue
		}
		if err := DB.Exec("ALTER TABLE `" + tableName + "` ADD COLUMN " + col.DDL).Error; err != nil {
			return err
		}
	}
	return nil
}

// migrateTokenModelLimitsToText migrates model_limits column from varchar(1024) to text
// This is safe to run multiple times - it checks the column type first
func migrateTokenModelLimitsToText() error {
	// SQLite uses type affinity, so TEXT and VARCHAR are effectively the same — no migration needed
	if common.UsingSQLite {
		return nil
	}

	tableName := "tokens"
	columnName := "model_limits"

	if !DB.Migrator().HasTable(tableName) {
		return nil
	}

	if !DB.Migrator().HasColumn(&Token{}, columnName) {
		return nil
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE text`, tableName, columnName)
	} else if common.UsingMySQL {
		var columnType string
		if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.ToLower(columnType) == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s text", tableName, columnName)
	} else {
		return nil
	}

	if alterSQL != "" {
		if err := DB.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("failed to migrate %s.%s to text: %w", tableName, columnName, err)
		}
		common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to text", tableName, columnName))
	}
	return nil
}

// migrateSubscriptionPlanPriceAmount migrates price_amount column from float/double to decimal(10,6)
// This is safe to run multiple times - it checks the column type first
func migrateSubscriptionPlanPriceAmount() {
	// SQLite doesn't support ALTER COLUMN, and its type affinity handles this automatically
	// Skip early to avoid GORM parsing the existing table DDL which may cause issues
	if common.UsingSQLite {
		return
	}

	tableName := "subscription_plans"
	columnName := "price_amount"

	// Check if table exists first
	if !DB.Migrator().HasTable(tableName) {
		return
	}

	// Check if column exists
	if !DB.Migrator().HasColumn(&SubscriptionPlan{}, columnName) {
		return
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		// PostgreSQL: Check if already decimal/numeric
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "numeric" {
			return // Already decimal/numeric
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE decimal(10,6) USING %s::decimal(10,6)`,
			tableName, columnName, columnName)
	} else if common.UsingMySQL {
		// MySQL: Check if already decimal
		var columnType string
		if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.HasPrefix(strings.ToLower(columnType), "decimal") {
			return // Already decimal
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s decimal(10,6) NOT NULL DEFAULT 0",
			tableName, columnName)
	} else {
		return
	}

	if alterSQL != "" {
		if err := DB.Exec(alterSQL).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to migrate %s.%s to decimal: %v", tableName, columnName, err))
		} else {
			common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to decimal(10,6)", tableName, columnName))
		}
	}
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func CloseDB() error {
	seen := make(map[*gorm.DB]struct{})
	for _, db := range []*gorm.DB{LOG_READ_DB, LOG_DB, READ_DB, DB} {
		if db == nil {
			continue
		}
		if _, ok := seen[db]; ok {
			continue
		}
		seen[db] = struct{}{}
		if err := closeDB(db); err != nil {
			return err
		}
	}
	return nil
}

// checkMySQLChineseSupport ensures the MySQL connection and current schema
// default charset/collation can store Chinese characters. It allows common
// Chinese-capable charsets (utf8mb4, utf8, gbk, big5, gb18030) and panics otherwise.
func checkMySQLChineseSupport(db *gorm.DB) error {
	// 仅检测：当前库默认字符集/排序规则 + 各表的排序规则（隐含字符集）

	// Read current schema defaults
	var schemaCharset, schemaCollation string
	err := db.Raw("SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = DATABASE()").Row().Scan(&schemaCharset, &schemaCollation)
	if err != nil {
		return fmt.Errorf("读取当前库默认字符集/排序规则失败 / Failed to read schema default charset/collation: %v", err)
	}

	toLower := func(s string) string { return strings.ToLower(s) }
	// Allowed charsets that can store Chinese text
	allowedCharsets := map[string]string{
		"utf8mb4": "utf8mb4_",
		"utf8":    "utf8_",
		"gbk":     "gbk_",
		"big5":    "big5_",
		"gb18030": "gb18030_",
	}
	isChineseCapable := func(cs, cl string) bool {
		csLower := toLower(cs)
		clLower := toLower(cl)
		if prefix, ok := allowedCharsets[csLower]; ok {
			if clLower == "" {
				return true
			}
			return strings.HasPrefix(clLower, prefix)
		}
		// 如果仅提供了排序规则，尝试按排序规则前缀判断
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(clLower, prefix) {
				return true
			}
		}
		return false
	}

	// 1) 当前库默认值必须支持中文
	if !isChineseCapable(schemaCharset, schemaCollation) {
		return fmt.Errorf("当前库默认字符集/排序规则不支持中文：schema(%s/%s)。请将库设置为 utf8mb4/utf8/gbk/big5/gb18030 / Schema default charset/collation is not Chinese-capable: schema(%s/%s). Please set to utf8mb4/utf8/gbk/big5/gb18030",
			schemaCharset, schemaCollation, schemaCharset, schemaCollation)
	}

	// 2) 所有物理表的排序规则（隐含字符集）必须支持中文
	type tableInfo struct {
		Name      string
		Collation *string
	}
	var tables []tableInfo
	if err := db.Raw("SELECT TABLE_NAME, TABLE_COLLATION FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'").Scan(&tables).Error; err != nil {
		return fmt.Errorf("读取表排序规则失败 / Failed to read table collations: %v", err)
	}

	var badTables []string
	for _, t := range tables {
		// NULL 或空表示继承库默认设置，已在上面校验库默认，视为通过
		if t.Collation == nil || *t.Collation == "" {
			continue
		}
		cl := *t.Collation
		// 仅凭排序规则判断是否中文可用
		ok := false
		lower := strings.ToLower(cl)
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(lower, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			badTables = append(badTables, fmt.Sprintf("%s(%s)", t.Name, cl))
		}
	}

	if len(badTables) > 0 {
		// 限制输出数量以避免日志过长
		maxShow := 20
		shown := badTables
		if len(shown) > maxShow {
			shown = shown[:maxShow]
		}
		return fmt.Errorf(
			"存在不支持中文的表，请修复其排序规则/字符集。示例（最多展示 %d 项）：%v / Found tables not Chinese-capable. Please fix their collation/charset. Examples (showing up to %d): %v",
			maxShow, shown, maxShow, shown,
		)
	}
	return nil
}

var (
	lastPingTime time.Time
	pingMutex    sync.Mutex
)

func PingDB() error {
	pingMutex.Lock()
	defer pingMutex.Unlock()

	if time.Since(lastPingTime) < time.Second*10 {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Error getting sql.DB from GORM: %v", err)
		return err
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Printf("Error pinging DB: %v", err)
		return err
	}

	lastPingTime = time.Now()
	common.SysLog("Database pinged successfully")
	return nil
}
