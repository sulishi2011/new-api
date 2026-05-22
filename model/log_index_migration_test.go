package model

import (
	"reflect"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMigrateLogIndexesRebuildsAndDropsOldIndexes(t *testing.T) {
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("failed to migrate log table: %v", err)
	}

	execSQL(t, db, "DROP INDEX `idx_created_at_id`")
	execSQL(t, db, "CREATE INDEX `idx_created_at_id` ON `logs`(`id`,`created_at`)")
	execSQL(t, db, "DROP INDEX `idx_logs_channel_created_id`")
	execSQL(t, db, "CREATE INDEX `idx_user_id_id` ON `logs`(`user_id`,`id`)")
	execSQL(t, db, "CREATE INDEX `idx_created_at_type` ON `logs`(`created_at`,`type`)")
	execSQL(t, db, "CREATE INDEX `idx_logs_channel_type_created_id` ON `logs`(`channel_id`,`type`,`created_at`,`id`)")
	execSQL(t, db, "CREATE INDEX `index_username_model_name` ON `logs`(`model_name`,`username`)")

	if err := migrateLogIndexes(db); err != nil {
		t.Fatalf("failed to migrate log indexes: %v", err)
	}

	assertLogTableIndexColumns(t, db, "logs", "idx_created_at_id", []string{"created_at", "id"})
	assertLogTableIndexColumns(t, db, "logs", "idx_logs_channel_created_id", []string{"channel_id", "created_at", "id"})

	for _, indexName := range []string{
		"idx_user_id_id",
		"idx_created_at_type",
		"idx_logs_channel_type_created_id",
		"index_username_model_name",
	} {
		if db.Migrator().HasIndex(&Log{}, indexName) {
			t.Fatalf("expected obsolete index %s to be dropped", indexName)
		}
	}
}

func TestMigrateLogIndexesUpdatesMonthlyShardIndexes(t *testing.T) {
	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("failed to migrate log table: %v", err)
	}

	tableName := "logs_202605"
	execSQL(t, db, "CREATE TABLE `"+tableName+"` AS SELECT * FROM `logs` WHERE 0")
	execSQL(t, db, "CREATE INDEX `idx_logs_202605_user_created_id` ON `"+tableName+"`(`user_id`,`created_at`,`id`)")
	execSQL(t, db, "CREATE INDEX `idx_logs_202605_channel_type_created_id` ON `"+tableName+"`(`channel_id`,`type`,`created_at`,`id`)")
	execSQL(t, db, "CREATE INDEX `idx_logs_202605_created_at_id` ON `"+tableName+"`(`created_at`,`id`)")

	if err := migrateLogIndexes(db); err != nil {
		t.Fatalf("failed to migrate log indexes: %v", err)
	}

	assertLogTableIndexColumns(t, db, tableName, "idx_logs_202605_created_at_id", []string{"created_at", "id"})
	assertLogTableIndexColumns(t, db, tableName, "idx_logs_202605_type_created_id", []string{"type", "created_at", "id"})
	assertLogTableIndexColumns(t, db, tableName, "idx_logs_202605_channel_created_id", []string{"channel_id", "created_at", "id"})
	assertLogTableIndexColumns(t, db, tableName, "idx_logs_202605_vendor_profile_created", []string{"vendor_profile_id", "created_at"})
	assertLogTableIndexColumns(t, db, tableName, "idx_logs_202605_biz_line_scene_created", []string{"biz_line", "biz_scene", "created_at"})

	for _, indexName := range []string{
		"idx_logs_202605_user_created_id",
		"idx_logs_202605_channel_type_created_id",
	} {
		got, err := getLogIndexColumns(db, tableName, indexName)
		if err != nil {
			t.Fatalf("failed to inspect obsolete index %s: %v", indexName, err)
		}
		if len(got) != 0 {
			t.Fatalf("expected obsolete shard index %s to be dropped, got columns %#v", indexName, got)
		}
	}
}

func execSQL(t *testing.T, db *gorm.DB, query string) {
	t.Helper()
	if err := db.Exec(query).Error; err != nil {
		t.Fatalf("failed to execute %q: %v", query, err)
	}
}

func assertLogTableIndexColumns(t *testing.T, db *gorm.DB, tableName string, indexName string, want []string) {
	t.Helper()
	got, err := getLogIndexColumns(db, tableName, indexName)
	if err != nil {
		t.Fatalf("failed to inspect index %s: %v", indexName, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("index %s columns = %#v, want %#v", indexName, got, want)
	}
}
