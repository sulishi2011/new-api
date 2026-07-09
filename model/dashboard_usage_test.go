package model

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDashboardUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
	oldLogReadDB := LOG_READ_DB
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db
	resetLogTableCaches()
	LOG_READ_DB = db

	if err := db.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("failed to migrate log table: %v", err)
	}

	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
		resetLogTableCaches()
		LOG_READ_DB = oldLogReadDB
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestGetDashboardUserQuotaDataFallsBackToPrimaryOnReadReplicaRecoveryConflict(t *testing.T) {
	db := setupDashboardUsageTestDB(t)

	logs := []*Log{
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        14401,
			Type:             LogTypeConsume,
			ModelName:        "gpt-4",
			Quota:            18,
			PromptTokens:     6,
			CompletionTokens: 4,
		},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed primary logs: %v", err)
	}

	readDB, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"_read?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open read sqlite db: %v", err)
	}
	readDB.Callback().Query().Before("gorm:query").Register("dashboard_usage_recovery_conflict", func(tx *gorm.DB) {
		tx.AddError(errors.New("ERROR: canceling statement due to conflict with recovery (SQLSTATE 40001)"))
	})
	LOG_READ_DB = readDB
	t.Cleanup(func() {
		sqlDB, err := readDB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	rows, err := GetDashboardUserQuotaData(DashboardUsageQuery{
		StartTimestamp: 14400,
		EndTimestamp:   14500,
	})
	if err != nil {
		t.Fatalf("expected primary fallback to succeed, got error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row from primary fallback, got %d", len(rows))
	}
	if rows[0].Username != "alice" || rows[0].Quota != 18 || rows[0].TokenUsed != 10 {
		t.Fatalf("unexpected fallback row: %#v", rows[0])
	}
}

func enableDashboardAggregateTestTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	oldUsageAggregationEnabled := common.UsageAggregationEnabled
	common.UsageAggregationEnabled = true
	t.Cleanup(func() {
		common.UsageAggregationEnabled = oldUsageAggregationEnabled
	})

	if err := db.AutoMigrate(&UsageLedger{}, &UsageAggregateHourly{}); err != nil {
		t.Fatalf("failed to migrate usage aggregate tables: %v", err)
	}
}

func TestGetDashboardQuotaDataGroupsByProviderKeyID(t *testing.T) {
	db := setupDashboardUsageTestDB(t)

	logs := []*Log{
		{
			UserId:           1,
			Username:         "admin",
			CreatedAt:        3601,
			Type:             LogTypeConsume,
			ModelName:        "grok-4",
			Quota:            10,
			PromptTokens:     5,
			CompletionTokens: 7,
			ChannelId:        3,
			TokenId:          11,
			ProviderKeyId:    101,
		},
		{
			UserId:           1,
			Username:         "admin",
			CreatedAt:        3659,
			Type:             LogTypeConsume,
			ModelName:        "grok-4",
			Quota:            20,
			PromptTokens:     2,
			CompletionTokens: 3,
			ChannelId:        3,
			TokenId:          12,
			ProviderKeyId:    101,
		},
		{
			UserId:           1,
			Username:         "admin",
			CreatedAt:        3670,
			Type:             LogTypeConsume,
			ModelName:        "grok-4",
			Quota:            30,
			PromptTokens:     11,
			CompletionTokens: 13,
			ChannelId:        4,
			TokenId:          12,
			ProviderKeyId:    202,
		},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	rows, err := GetDashboardQuotaData(DashboardUsageQuery{
		StartTimestamp: 3600,
		EndTimestamp:   4000,
		ModelName:      "grok-4",
		Dimension:      DashboardDimensionProviderKey,
		Metric:         DashboardMetricOriginal,
	})
	if err != nil {
		t.Fatalf("failed to query dashboard usage data: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 aggregated rows, got %d", len(rows))
	}
	rowMap := make(map[string]*QuotaData, len(rows))
	for _, row := range rows {
		rowMap[row.ModelName] = row
	}
	if rowMap["101"] == nil || rowMap["101"].CreatedAt != 3600 || rowMap["101"].Count != 2 || rowMap["101"].Quota != 30 || rowMap["101"].TokenUsed != 17 {
		t.Fatalf("unexpected row for provider key 101: %#v", rowMap["101"])
	}
	if rowMap["202"] == nil || rowMap["202"].CreatedAt != 3600 || rowMap["202"].Count != 1 || rowMap["202"].Quota != 30 || rowMap["202"].TokenUsed != 24 {
		t.Fatalf("unexpected row for provider key 202: %#v", rowMap["202"])
	}
}

func TestGetDashboardQuotaDataUsesHourlyAggregateTable(t *testing.T) {
	db := setupDashboardUsageTestDB(t)
	enableDashboardAggregateTestTables(t, db)

	bucketStart := time.Unix(common.GetTimestamp(), 0).UTC().Truncate(time.Hour).Add(-3 * time.Hour).Unix()
	aggregates := []*UsageAggregateHourly{
		{
			BucketStart:    bucketStart,
			RequestedModel: "gpt-4",
			RequestCount:   2,
			Quota:          30,
			TotalTokens:    70,
		},
		{
			BucketStart:    bucketStart,
			RequestedModel: "gpt-4o",
			RequestCount:   1,
			Quota:          12,
			TotalTokens:    20,
		},
	}
	if err := db.Create(&aggregates).Error; err != nil {
		t.Fatalf("failed to seed hourly aggregates: %v", err)
	}

	rows, err := GetDashboardQuotaData(DashboardUsageQuery{
		StartTimestamp: bucketStart,
		EndTimestamp:   bucketStart + usageAggregateHourSeconds - 1,
		Dimension:      DashboardDimensionModel,
		Metric:         DashboardMetricOriginal,
		Granularity:    UsageAggregateGranularityHour,
	})
	if err != nil {
		t.Fatalf("failed to query dashboard aggregate usage data: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 aggregate rows, got %d", len(rows))
	}
	rowMap := make(map[string]*QuotaData, len(rows))
	for _, row := range rows {
		rowMap[row.ModelName] = row
	}
	if rowMap["gpt-4"] == nil || rowMap["gpt-4"].CreatedAt != bucketStart || rowMap["gpt-4"].Count != 2 || rowMap["gpt-4"].Quota != 30 || rowMap["gpt-4"].TokenUsed != 70 {
		t.Fatalf("unexpected aggregate row for gpt-4: %#v", rowMap["gpt-4"])
	}
	if rowMap["gpt-4o"] == nil || rowMap["gpt-4o"].CreatedAt != bucketStart || rowMap["gpt-4o"].Count != 1 || rowMap["gpt-4o"].Quota != 12 || rowMap["gpt-4o"].TokenUsed != 20 {
		t.Fatalf("unexpected aggregate row for gpt-4o: %#v", rowMap["gpt-4o"])
	}
}

func TestGetDashboardQuotaDataMergesHourlyAggregateAndLedgerRows(t *testing.T) {
	db := setupDashboardUsageTestDB(t)
	enableDashboardAggregateTestTables(t, db)

	now := time.Unix(common.GetTimestamp(), 0).UTC()
	aggregateBucket := now.Truncate(time.Hour).Add(-3 * time.Hour).Unix()
	recentTimestamp := now.Add(-20 * time.Minute).Unix()
	recentBucket := UsageAggregateHourBucket(recentTimestamp)

	aggregate := &UsageAggregateHourly{
		BucketStart:    aggregateBucket,
		RequestedModel: "gpt-4",
		RequestCount:   2,
		Quota:          30,
		TotalTokens:    70,
	}
	if err := db.Create(aggregate).Error; err != nil {
		t.Fatalf("failed to seed hourly aggregate: %v", err)
	}

	ledger := &UsageLedger{
		Timestamp:      recentTimestamp,
		RequestedModel: "gpt-4",
		Quota:          11,
		TotalTokens:    13,
	}
	if err := db.Create(ledger).Error; err != nil {
		t.Fatalf("failed to seed usage ledger: %v", err)
	}

	rows, err := GetDashboardQuotaData(DashboardUsageQuery{
		StartTimestamp: aggregateBucket,
		EndTimestamp:   recentTimestamp,
		Dimension:      DashboardDimensionModel,
		Metric:         DashboardMetricOriginal,
		Granularity:    UsageAggregateGranularityHour,
	})
	if err != nil {
		t.Fatalf("failed to query merged dashboard usage data: %v", err)
	}

	rowMap := make(map[int64]*QuotaData, len(rows))
	for _, row := range rows {
		rowMap[row.CreatedAt] = row
	}
	if rowMap[aggregateBucket] == nil || rowMap[aggregateBucket].Count != 2 || rowMap[aggregateBucket].Quota != 30 || rowMap[aggregateBucket].TokenUsed != 70 {
		t.Fatalf("unexpected aggregate bucket row: %#v", rowMap[aggregateBucket])
	}
	if rowMap[recentBucket] == nil || rowMap[recentBucket].Count != 1 || rowMap[recentBucket].Quota != 11 || rowMap[recentBucket].TokenUsed != 13 {
		t.Fatalf("unexpected recent ledger bucket row: %#v", rowMap[recentBucket])
	}
}

func TestGetDashboardQuotaDataSupportsCostMetric(t *testing.T) {
	db := setupDashboardUsageTestDB(t)

	costOne := 50
	costZero := 0
	logs := []*Log{
		{
			UserId:           1,
			Username:         "admin",
			CreatedAt:        10801,
			Type:             LogTypeConsume,
			ModelName:        "claude-sonnet",
			Quota:            100,
			CostQuota:        &costOne,
			PromptTokens:     20,
			CompletionTokens: 10,
			ChannelId:        7,
			TokenId:          33,
			ProviderKeyId:    3001,
		},
		{
			UserId:           1,
			Username:         "admin",
			CreatedAt:        10820,
			Type:             LogTypeConsume,
			ModelName:        "claude-sonnet",
			Quota:            40,
			CostQuota:        nil,
			PromptTokens:     4,
			CompletionTokens: 6,
			ChannelId:        7,
			TokenId:          33,
			ProviderKeyId:    3001,
		},
		{
			UserId:           1,
			Username:         "admin",
			CreatedAt:        10830,
			Type:             LogTypeConsume,
			ModelName:        "grok-4",
			Quota:            20,
			CostQuota:        &costZero,
			PromptTokens:     2,
			CompletionTokens: 3,
			ChannelId:        8,
			TokenId:          34,
			ProviderKeyId:    3002,
		},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	rows, err := GetDashboardQuotaData(DashboardUsageQuery{
		StartTimestamp: 10800,
		EndTimestamp:   10900,
		Dimension:      DashboardDimensionModel,
		Metric:         DashboardMetricCost,
	})
	if err != nil {
		t.Fatalf("failed to query dashboard cost usage data: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 aggregated rows, got %d", len(rows))
	}
	rowMap := make(map[string]*QuotaData, len(rows))
	for _, row := range rows {
		rowMap[row.ModelName] = row
	}
	if rowMap["claude-sonnet"] == nil || rowMap["claude-sonnet"].Quota != 90 {
		t.Fatalf("unexpected cost row for claude-sonnet: %#v", rowMap["claude-sonnet"])
	}
	if rowMap["grok-4"] == nil || rowMap["grok-4"].Quota != 0 {
		t.Fatalf("unexpected cost row for grok-4: %#v", rowMap["grok-4"])
	}
}

func TestGetDashboardUserQuotaDataAppliesFilters(t *testing.T) {
	db := setupDashboardUsageTestDB(t)

	logs := []*Log{
		{
			UserId:           1,
			Username:         "alice",
			CreatedAt:        7201,
			Type:             LogTypeConsume,
			ModelName:        "claude-sonnet",
			Quota:            18,
			PromptTokens:     6,
			CompletionTokens: 4,
			ChannelId:        9,
			TokenId:          77,
			ProviderKeyId:    5001,
		},
		{
			UserId:           2,
			Username:         "bob",
			CreatedAt:        7220,
			Type:             LogTypeConsume,
			ModelName:        "claude-sonnet",
			Quota:            28,
			PromptTokens:     8,
			CompletionTokens: 9,
			ChannelId:        9,
			TokenId:          88,
			ProviderKeyId:    5001,
		},
		{
			UserId:           2,
			Username:         "bob",
			CreatedAt:        7250,
			Type:             LogTypeConsume,
			ModelName:        "grok-4",
			Quota:            40,
			PromptTokens:     9,
			CompletionTokens: 9,
			ChannelId:        9,
			TokenId:          88,
			ProviderKeyId:    5001,
		},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("failed to seed logs: %v", err)
	}

	rows, err := GetDashboardUserQuotaData(DashboardUsageQuery{
		StartTimestamp: 7200,
		EndTimestamp:   7600,
		ModelName:      "claude-sonnet",
		ProviderKeyID:  5001,
		Metric:         DashboardMetricOriginal,
	})
	if err != nil {
		t.Fatalf("failed to query dashboard user usage data: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 user rows, got %d", len(rows))
	}
	rowMap := make(map[string]*QuotaData, len(rows))
	for _, row := range rows {
		rowMap[row.Username] = row
	}
	if rowMap["alice"] == nil || rowMap["alice"].Quota != 18 || rowMap["alice"].TokenUsed != 10 {
		t.Fatalf("unexpected user row for alice: %#v", rowMap["alice"])
	}
	if rowMap["bob"] == nil || rowMap["bob"].Quota != 28 || rowMap["bob"].TokenUsed != 17 {
		t.Fatalf("unexpected user row for bob: %#v", rowMap["bob"])
	}
}
