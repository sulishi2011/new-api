package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUsageAggregateTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	oldDB := DB
	oldLogDB := LOG_DB
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

	if err := db.AutoMigrate(&UsageLedger{}, &UsageAggregateHourly{}, &UsageAggregateDaily{}, &UsageAggregationJob{}); err != nil {
		t.Fatalf("failed to migrate usage aggregate tables: %v", err)
	}

	t.Cleanup(func() {
		DB = oldDB
		LOG_DB = oldLogDB
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

func TestQueryUsageAggregatesLiveUsesLedgerBeforeRollup(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	rows := []*UsageLedger{
		{
			NewapiLogId:        1,
			Timestamp:          3601,
			NewapiChannelId:    10,
			ChannelName:        "primary",
			ProviderKeyId:      1001,
			ProviderKeyPreview: "sk-1",
			TokenId:            2001,
			TokenName:          "main",
			RequestedModel:     "gpt-4o",
			ActualModel:        "gpt-4o-2024",
			Quota:              50,
			CostQuota:          20,
			InputTokens:        30,
			OutputTokens:       12,
			CacheReadTokens:    5,
			CacheWriteTokens:   7,
			TotalTokens:        42,
		},
		{
			NewapiLogId:        2,
			Timestamp:          3650,
			NewapiChannelId:    10,
			ChannelName:        "primary",
			ProviderKeyId:      1001,
			ProviderKeyPreview: "sk-1",
			TokenId:            2001,
			TokenName:          "main",
			RequestedModel:     "gpt-4o",
			ActualModel:        "gpt-4o-2024",
			Quota:              70,
			CostQuota:          30,
			InputTokens:        40,
			OutputTokens:       18,
			CacheReadTokens:    6,
			CacheWriteTokens:   8,
			TotalTokens:        58,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed usage ledger: %v", err)
	}

	materializedRows, materializedTotal, materializedSummary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:    UsageAggregateGranularityHour,
		StartTimestamp: 3600,
		EndTimestamp:   7200,
	})
	if err != nil {
		t.Fatalf("failed to query materialized usage aggregates: %v", err)
	}
	if materializedTotal != 0 || len(materializedRows) != 0 || materializedSummary.RequestCount != 0 {
		t.Fatalf("expected empty materialized query before rollup, got total=%d rows=%d summary=%#v", materializedTotal, len(materializedRows), materializedSummary)
	}

	liveRows, liveTotal, liveSummary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:    UsageAggregateGranularityHour,
		Live:           true,
		StartTimestamp: 3600,
		EndTimestamp:   7200,
	})
	if err != nil {
		t.Fatalf("failed to query live usage aggregates: %v", err)
	}
	if liveTotal != 1 || len(liveRows) != 1 {
		t.Fatalf("expected one live aggregate row, got total=%d rows=%d", liveTotal, len(liveRows))
	}
	row := liveRows[0]
	if row.BucketStart != 3600 || row.RequestCount != 2 || row.Quota != 120 || row.CostQuota != 50 || row.InputTokens != 70 || row.OutputTokens != 30 || row.CacheReadTokens != 11 || row.CacheWriteTokens != 15 || row.TotalTokens != 100 {
		t.Fatalf("unexpected live aggregate row: %#v", row)
	}
	if liveSummary.RequestCount != 2 || liveSummary.Quota != 120 || liveSummary.CostQuota != 50 || liveSummary.TotalTokens != 100 {
		t.Fatalf("unexpected live summary: %#v", liveSummary)
	}
}

func TestUsageAggregateRollupAndExport(t *testing.T) {
	db := setupUsageAggregateTestDB(t)
	dayStart := int64(172800)

	rows := []*UsageLedger{
		{
			NewapiLogId:     1,
			Timestamp:       dayStart + 7201,
			NewapiChannelId: 7,
			ProviderKeyId:   101,
			TokenId:         201,
			RequestedModel:  "claude-sonnet",
			ActualModel:     "claude-sonnet",
			Quota:           10,
			CostQuota:       4,
			InputTokens:     6,
			OutputTokens:    2,
			TotalTokens:     8,
		},
		{
			NewapiLogId:     2,
			Timestamp:       dayStart + 10801,
			NewapiChannelId: 8,
			ProviderKeyId:   102,
			TokenId:         202,
			RequestedModel:  "grok-4",
			ActualModel:     "grok-4-fast",
			Quota:           30,
			CostQuota:       12,
			InputTokens:     10,
			OutputTokens:    5,
			TotalTokens:     15,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed usage ledger: %v", err)
	}

	if err := RollupUsageAggregateHourly(dayStart + 7200); err != nil {
		t.Fatalf("failed to roll up hourly usage: %v", err)
	}
	if err := RollupUsageAggregateHourly(dayStart + 10800); err != nil {
		t.Fatalf("failed to roll up second hourly usage: %v", err)
	}
	if err := RollupUsageAggregateDaily(dayStart); err != nil {
		t.Fatalf("failed to roll up daily usage: %v", err)
	}

	dailyRows, total, summary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:    UsageAggregateGranularityDay,
		StartTimestamp: dayStart,
		EndTimestamp:   dayStart + 86400,
		SortBy:         "quota",
		SortOrder:      "desc",
	})
	if err != nil {
		t.Fatalf("failed to query daily aggregates: %v", err)
	}
	if total != 2 || len(dailyRows) != 2 {
		t.Fatalf("expected two daily rows, got total=%d rows=%d", total, len(dailyRows))
	}
	if dailyRows[0].ActualModel != "grok-4-fast" || dailyRows[0].Quota != 30 {
		t.Fatalf("expected quota desc order with grok first, got %#v", dailyRows[0])
	}
	if summary.RequestCount != 2 || summary.Quota != 40 || summary.CostQuota != 16 || summary.TotalTokens != 23 {
		t.Fatalf("unexpected daily summary: %#v", summary)
	}

	exportRows, exportTotal, err := ExportUsageAggregates(UsageAggregateQuery{
		Granularity: UsageAggregateGranularityDay,
		Live:        true,
		SortBy:      "quota",
		SortOrder:   "desc",
	}, 10)
	if err != nil {
		t.Fatalf("failed to export live aggregates: %v", err)
	}
	if exportTotal != 2 || len(exportRows) != 2 {
		t.Fatalf("expected two exported live rows, got total=%d rows=%d", exportTotal, len(exportRows))
	}
}
