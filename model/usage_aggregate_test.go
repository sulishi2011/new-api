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

func TestUsageAggregateBucketParamExpressionByDatabase(t *testing.T) {
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
	})

	tests := []struct {
		name            string
		usingSQLite     bool
		usingMySQL      bool
		usingPostgreSQL bool
		want            string
	}{
		{
			name:            "postgresql",
			usingPostgreSQL: true,
			want:            "CAST(? AS BIGINT)",
		},
		{
			name:       "mysql",
			usingMySQL: true,
			want:       "CAST(? AS SIGNED)",
		},
		{
			name:        "sqlite",
			usingSQLite: true,
			want:        "CAST(? AS INTEGER)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			common.UsingSQLite = tt.usingSQLite
			common.UsingMySQL = tt.usingMySQL
			common.UsingPostgreSQL = tt.usingPostgreSQL

			if got := usageAggregateBucketParamExpression(); got != tt.want {
				t.Fatalf("unexpected bucket param expression: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUsageAggregateBucketExpressionByDatabase(t *testing.T) {
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
	})

	tests := []struct {
		name            string
		usingSQLite     bool
		usingMySQL      bool
		usingPostgreSQL bool
		granularity     string
		want            string
	}{
		{
			name:            "postgresql hour",
			usingPostgreSQL: true,
			granularity:     UsageAggregateGranularityHour,
			want:            "((timestamp / 3600) * 3600)",
		},
		{
			name:        "sqlite day",
			usingSQLite: true,
			granularity: UsageAggregateGranularityDay,
			want:        "((timestamp / 86400) * 86400)",
		},
		{
			name:        "mysql hour",
			usingMySQL:  true,
			granularity: UsageAggregateGranularityHour,
			want:        "(FLOOR(timestamp / 3600) * 3600)",
		},
		{
			name:        "mysql day",
			usingMySQL:  true,
			granularity: UsageAggregateGranularityDay,
			want:        "(FLOOR(timestamp / 86400) * 86400)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			common.UsingSQLite = tt.usingSQLite
			common.UsingMySQL = tt.usingMySQL
			common.UsingPostgreSQL = tt.usingPostgreSQL

			if got := usageAggregateBucketExpression(tt.granularity, "timestamp"); got != tt.want {
				t.Fatalf("unexpected bucket expression: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUsageAggregateBucketExpressionWithTimezoneOffset(t *testing.T) {
	oldUsingSQLite := common.UsingSQLite
	oldUsingMySQL := common.UsingMySQL
	oldUsingPostgreSQL := common.UsingPostgreSQL
	t.Cleanup(func() {
		common.UsingSQLite = oldUsingSQLite
		common.UsingMySQL = oldUsingMySQL
		common.UsingPostgreSQL = oldUsingPostgreSQL
	})

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	got := usageAggregateBucketExpressionWithOffset(UsageAggregateGranularityDay, "timestamp", 8*3600)
	want := "((((timestamp + 28800) / 86400) * 86400) - 28800)"
	if got != want {
		t.Fatalf("unexpected timezone bucket expression: got %q, want %q", got, want)
	}
}

func TestQueryUsageAggregatesLiveUsesLedgerBeforeRollup(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	rows := []*UsageLedger{
		{
			NewapiLogId:        1,
			Timestamp:          3601,
			NewapiChannelId:    10,
			ChannelName:        "primary",
			VendorProfileCode:  "OPENAI-API",
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
			VendorProfileCode:  "OPENAI-API",
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
	if row.VendorProfileCode != "OPENAI-API" {
		t.Fatalf("expected vendor profile code to be preserved, got %q", row.VendorProfileCode)
	}
	if liveSummary.RequestCount != 2 || liveSummary.Quota != 120 || liveSummary.CostQuota != 50 || liveSummary.TotalTokens != 100 {
		t.Fatalf("unexpected live summary: %#v", liveSummary)
	}
}

func TestQueryUsageAggregatesUsesTimezoneOffsetForLiveBuckets(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	const (
		timezoneOffset = int64(8 * 3600)
		localDayStart  = int64(1704124800) // 2024-01-02 00:00:00 UTC+08:00
		localDayEnd    = int64(1704211200) // 2024-01-03 00:00:00 UTC+08:00
	)
	rows := []*UsageLedger{
		{
			NewapiLogId:     1,
			Timestamp:       localDayStart + 60,
			NewapiChannelId: 10,
			RequestedModel:  "gpt-4o",
			ActualModel:     "gpt-4o",
			Quota:           50,
			CostQuota:       20,
			InputTokens:     30,
			OutputTokens:    12,
			TotalTokens:     42,
		},
		{
			NewapiLogId:     2,
			Timestamp:       localDayEnd - 1,
			NewapiChannelId: 10,
			RequestedModel:  "gpt-4o",
			ActualModel:     "gpt-4o",
			Quota:           70,
			CostQuota:       30,
			InputTokens:     40,
			OutputTokens:    18,
			TotalTokens:     58,
		},
		{
			NewapiLogId:     3,
			Timestamp:       localDayEnd,
			NewapiChannelId: 10,
			RequestedModel:  "gpt-4o",
			ActualModel:     "gpt-4o",
			Quota:           90,
			CostQuota:       40,
			InputTokens:     50,
			OutputTokens:    20,
			TotalTokens:     70,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed usage ledger: %v", err)
	}

	liveRows, total, summary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:           UsageAggregateGranularityDay,
		Live:                  true,
		TimezoneOffsetSeconds: timezoneOffset,
		StartTimestamp:        localDayStart,
		EndTimestamp:          localDayEnd,
	})
	if err != nil {
		t.Fatalf("failed to query live usage aggregates: %v", err)
	}
	if total != 1 || len(liveRows) != 1 {
		t.Fatalf("expected one local-day aggregate row, got total=%d rows=%d", total, len(liveRows))
	}
	if liveRows[0].BucketStart != localDayStart || liveRows[0].RequestCount != 2 || liveRows[0].Quota != 120 {
		t.Fatalf("unexpected local-day aggregate row: %#v", liveRows[0])
	}
	if summary.RequestCount != 2 || summary.Quota != 120 || summary.CostQuota != 50 || summary.TotalTokens != 100 {
		t.Fatalf("unexpected local-day summary: %#v", summary)
	}
}

func TestQueryUsageAggregatesFallsBackToLedgerForTimezoneOffset(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	if err := db.Create(&UsageLedger{
		NewapiLogId:     1,
		Timestamp:       1704124860,
		NewapiChannelId: 10,
		RequestedModel:  "gpt-4o",
		ActualModel:     "gpt-4o",
		Quota:           50,
		CostQuota:       20,
		TotalTokens:     42,
	}).Error; err != nil {
		t.Fatalf("failed to seed usage ledger: %v", err)
	}

	rows, total, summary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:           UsageAggregateGranularityDay,
		TimezoneOffsetSeconds: 8 * 3600,
		StartTimestamp:        1704124800,
		EndTimestamp:          1704211200,
	})
	if err != nil {
		t.Fatalf("failed to query usage aggregates: %v", err)
	}
	if total != 1 || len(rows) != 1 || summary.Quota != 50 {
		t.Fatalf("expected non-live query with timezone offset to use ledger, got total=%d rows=%d summary=%#v", total, len(rows), summary)
	}
}

func TestQueryUsageAggregatesUsesMaterializedHourlyForAlignedHourTimezoneOffset(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	const (
		timezoneOffset = int64(8 * 3600)
		bucketStart    = int64(1704124800)
	)
	if err := db.Create(&UsageAggregateHourly{
		BucketStart:    bucketStart,
		ChannelId:      10,
		RequestedModel: "gpt-4o",
		ActualModel:    "gpt-4o",
		RequestCount:   1,
		Quota:          50,
		CostQuota:      20,
		TotalTokens:    42,
	}).Error; err != nil {
		t.Fatalf("failed to seed hourly usage aggregate: %v", err)
	}
	if err := db.Create(&UsageLedger{
		NewapiLogId:     10,
		Timestamp:       bucketStart + 60,
		NewapiChannelId: 10,
		RequestedModel:  "gpt-4o",
		ActualModel:     "gpt-4o",
		Quota:           70,
		CostQuota:       30,
		TotalTokens:     58,
	}).Error; err != nil {
		t.Fatalf("failed to seed usage ledger: %v", err)
	}
	if err := db.Create(&UsageAggregationJob{
		JobType:     "hourly",
		BucketStart: bucketStart,
		Status:      "failed",
	}).Error; err != nil {
		t.Fatalf("failed to seed failed hourly aggregation job: %v", err)
	}

	rows, total, summary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:           UsageAggregateGranularityHour,
		TimezoneOffsetSeconds: timezoneOffset,
		StartTimestamp:        bucketStart,
		EndTimestamp:          bucketStart + usageAggregateHourSeconds,
	})
	if err != nil {
		t.Fatalf("failed to query usage aggregates: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected one materialized hourly row, got total=%d rows=%d", total, len(rows))
	}
	if rows[0].BucketStart != bucketStart || rows[0].RequestCount != 1 || rows[0].Quota != 50 || rows[0].CostQuota != 20 || rows[0].TotalTokens != 42 {
		t.Fatalf("expected non-live aligned hour query to use materialized hourly aggregate, got %#v", rows[0])
	}
	if summary.RequestCount != 1 || summary.Quota != 50 || summary.CostQuota != 20 || summary.TotalTokens != 42 {
		t.Fatalf("unexpected materialized hourly summary: %#v", summary)
	}
}

func TestQueryUsageAggregatesGroupsLiveLedgerByChannel(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	const bucketStart = int64(1704124800)
	rows := []*UsageLedger{
		{
			NewapiLogId:        1,
			Timestamp:          bucketStart + 60,
			NewapiChannelId:    10,
			ChannelName:        "primary",
			VendorProfileCode:  "OPENAI-API",
			ProviderKeyId:      1001,
			ProviderKeyPreview: "sk-1",
			TokenId:            2001,
			TokenName:          "token-a",
			RequestedModel:     "gpt-4o",
			ActualModel:        "gpt-4o",
			Quota:              50,
			CostQuota:          20,
			TotalTokens:        42,
		},
		{
			NewapiLogId:        2,
			Timestamp:          bucketStart + 120,
			NewapiChannelId:    10,
			ChannelName:        "primary",
			VendorProfileCode:  "OPENAI-API",
			ProviderKeyId:      1002,
			ProviderKeyPreview: "sk-2",
			TokenId:            2002,
			TokenName:          "token-b",
			RequestedModel:     "gpt-4o-mini",
			ActualModel:        "gpt-4o-mini",
			Quota:              70,
			CostQuota:          30,
			TotalTokens:        58,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed usage ledger: %v", err)
	}

	groupedRows, total, summary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:    UsageAggregateGranularityDay,
		GroupBy:        UsageAggregateGroupByChannel,
		Live:           true,
		StartTimestamp: bucketStart,
		EndTimestamp:   bucketStart + usageAggregateDaySeconds,
	})
	if err != nil {
		t.Fatalf("failed to query grouped usage aggregates: %v", err)
	}
	if total != 1 || len(groupedRows) != 1 {
		t.Fatalf("expected one channel-grouped row, got total=%d rows=%d", total, len(groupedRows))
	}
	row := groupedRows[0]
	if row.ChannelId != 10 || row.RequestCount != 2 || row.Quota != 120 || row.CostQuota != 50 || row.TotalTokens != 100 {
		t.Fatalf("unexpected channel-grouped row: %#v", row)
	}
	if row.ProviderKeyId != 0 || row.ProviderKeyPreview != "" || row.TokenId != 0 || row.TokenName != "" || row.RequestedModel != "" || row.ActualModel != "" {
		t.Fatalf("expected non-channel dimensions to be collapsed, got %#v", row)
	}
	if summary.RequestCount != 2 || summary.Quota != 120 || summary.CostQuota != 50 || summary.TotalTokens != 100 {
		t.Fatalf("unexpected grouped summary: %#v", summary)
	}
}

func TestQueryUsageAggregatesGroupsMaterializedRowsByChannelWithFilters(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	const bucketStart = int64(1704067200)
	rows := []*UsageAggregateDaily{
		{
			BucketStart:    bucketStart,
			ChannelId:      10,
			ChannelName:    "primary",
			ProviderKeyId:  1001,
			TokenId:        2001,
			RequestedModel: "gpt-4o",
			ActualModel:    "gpt-4o",
			RequestCount:   1,
			Quota:          50,
			CostQuota:      20,
			TotalTokens:    42,
		},
		{
			BucketStart:    bucketStart,
			ChannelId:      10,
			ChannelName:    "primary",
			ProviderKeyId:  1002,
			TokenId:        2002,
			RequestedModel: "gpt-4o-mini",
			ActualModel:    "gpt-4o-mini",
			RequestCount:   1,
			Quota:          70,
			CostQuota:      30,
			TotalTokens:    58,
		},
		{
			BucketStart:    bucketStart,
			ChannelId:      20,
			ChannelName:    "secondary",
			ProviderKeyId:  1003,
			TokenId:        2003,
			RequestedModel: "claude-sonnet",
			ActualModel:    "claude-sonnet",
			RequestCount:   1,
			Quota:          90,
			CostQuota:      40,
			TotalTokens:    70,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed daily usage aggregates: %v", err)
	}

	groupedRows, total, summary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:    UsageAggregateGranularityDay,
		GroupBy:        UsageAggregateGroupByChannel,
		StartTimestamp: bucketStart,
		EndTimestamp:   bucketStart + usageAggregateDaySeconds,
		ChannelId:      10,
	})
	if err != nil {
		t.Fatalf("failed to query grouped materialized usage aggregates: %v", err)
	}
	if total != 1 || len(groupedRows) != 1 {
		t.Fatalf("expected one filtered channel row, got total=%d rows=%d", total, len(groupedRows))
	}
	if groupedRows[0].ChannelId != 10 || groupedRows[0].RequestCount != 2 || groupedRows[0].Quota != 120 || groupedRows[0].CostQuota != 50 || groupedRows[0].TotalTokens != 100 {
		t.Fatalf("unexpected filtered channel row: %#v", groupedRows[0])
	}
	if summary.RequestCount != 2 || summary.Quota != 120 || summary.CostQuota != 50 || summary.TotalTokens != 100 {
		t.Fatalf("unexpected filtered channel summary: %#v", summary)
	}
}

func TestQueryUsageAggregatesUsesLedgerForPartialHourTimezoneOffset(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	const (
		timezoneOffset = int64(8 * 3600)
		bucketStart    = int64(1704124800)
	)
	if err := db.Create(&UsageAggregateHourly{
		BucketStart:    bucketStart,
		ChannelId:      10,
		RequestedModel: "gpt-4o",
		ActualModel:    "gpt-4o",
		RequestCount:   1,
		Quota:          50,
		CostQuota:      20,
		TotalTokens:    42,
	}).Error; err != nil {
		t.Fatalf("failed to seed hourly usage aggregate: %v", err)
	}
	if err := db.Create(&UsageLedger{
		NewapiLogId:     10,
		Timestamp:       bucketStart + 60,
		NewapiChannelId: 10,
		RequestedModel:  "gpt-4o",
		ActualModel:     "gpt-4o",
		Quota:           70,
		CostQuota:       30,
		TotalTokens:     58,
	}).Error; err != nil {
		t.Fatalf("failed to seed usage ledger: %v", err)
	}

	rows, total, summary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:           UsageAggregateGranularityHour,
		TimezoneOffsetSeconds: timezoneOffset,
		StartTimestamp:        bucketStart + 1,
		EndTimestamp:          bucketStart + usageAggregateHourSeconds,
	})
	if err != nil {
		t.Fatalf("failed to query usage aggregates: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected one partial-hour ledger row, got total=%d rows=%d", total, len(rows))
	}
	if rows[0].BucketStart != bucketStart || rows[0].RequestCount != 1 || rows[0].Quota != 70 || rows[0].CostQuota != 30 || rows[0].TotalTokens != 58 {
		t.Fatalf("expected partial hour query to use ledger, got %#v", rows[0])
	}
	if summary.RequestCount != 1 || summary.Quota != 70 || summary.CostQuota != 30 || summary.TotalTokens != 58 {
		t.Fatalf("unexpected partial-hour ledger summary: %#v", summary)
	}
}

func TestQueryUsageAggregatesUsesHourlyRollupForTimezoneOffset(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	const (
		timezoneOffset = int64(8 * 3600)
		localDayStart  = int64(1704124800) // 2024-01-02 00:00:00 UTC+08:00
	)
	rows := []*UsageAggregateHourly{
		{
			BucketStart:    localDayStart,
			ChannelId:      10,
			RequestedModel: "gpt-4o",
			ActualModel:    "gpt-4o",
			RequestCount:   1,
			Quota:          50,
			CostQuota:      20,
			InputTokens:    30,
			OutputTokens:   12,
			TotalTokens:    42,
		},
		{
			BucketStart:    localDayStart + usageAggregateHourSeconds,
			ChannelId:      10,
			RequestedModel: "gpt-4o",
			ActualModel:    "gpt-4o",
			RequestCount:   1,
			Quota:          70,
			CostQuota:      30,
			InputTokens:    40,
			OutputTokens:   18,
			TotalTokens:    58,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to seed hourly usage aggregates: %v", err)
	}

	liveRows, total, summary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:           UsageAggregateGranularityDay,
		TimezoneOffsetSeconds: timezoneOffset,
		StartTimestamp:        localDayStart,
		EndTimestamp:          localDayStart + 2*usageAggregateHourSeconds,
	})
	if err != nil {
		t.Fatalf("failed to query usage aggregates: %v", err)
	}
	if total != 1 || len(liveRows) != 1 {
		t.Fatalf("expected one hourly-rollup local-day row, got total=%d rows=%d", total, len(liveRows))
	}
	if liveRows[0].BucketStart != localDayStart || liveRows[0].RequestCount != 2 || liveRows[0].Quota != 120 || liveRows[0].TotalTokens != 100 {
		t.Fatalf("unexpected hourly-rollup local-day row: %#v", liveRows[0])
	}
	if summary.RequestCount != 2 || summary.Quota != 120 || summary.CostQuota != 50 || summary.TotalTokens != 100 {
		t.Fatalf("unexpected hourly-rollup summary: %#v", summary)
	}
}

func TestQueryUsageAggregatesUsesLedgerOnlyForHourlyTail(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	const (
		timezoneOffset = int64(8 * 3600)
		localDayStart  = int64(1704124800) // 2024-01-02 00:00:00 UTC+08:00
	)
	if err := db.Create(&UsageAggregateHourly{
		BucketStart:    localDayStart,
		ChannelId:      10,
		RequestedModel: "gpt-4o",
		ActualModel:    "gpt-4o",
		RequestCount:   1,
		Quota:          50,
		CostQuota:      20,
		TotalTokens:    42,
	}).Error; err != nil {
		t.Fatalf("failed to seed hourly usage aggregate: %v", err)
	}
	if err := db.Create(&UsageLedger{
		NewapiLogId:     10,
		Timestamp:       localDayStart + usageAggregateHourSeconds + 60,
		NewapiChannelId: 10,
		RequestedModel:  "gpt-4o",
		ActualModel:     "gpt-4o",
		Quota:           70,
		CostQuota:       30,
		TotalTokens:     58,
	}).Error; err != nil {
		t.Fatalf("failed to seed usage ledger tail: %v", err)
	}

	liveRows, total, summary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:           UsageAggregateGranularityDay,
		Live:                  true,
		TimezoneOffsetSeconds: timezoneOffset,
		StartTimestamp:        localDayStart,
		EndTimestamp:          localDayStart + 2*usageAggregateHourSeconds,
	})
	if err != nil {
		t.Fatalf("failed to query usage aggregates: %v", err)
	}
	if total != 1 || len(liveRows) != 1 {
		t.Fatalf("expected one local-day row after merging hourly and ledger tail, got total=%d rows=%d", total, len(liveRows))
	}
	if liveRows[0].BucketStart != localDayStart || liveRows[0].RequestCount != 2 || liveRows[0].Quota != 120 || liveRows[0].TotalTokens != 100 {
		t.Fatalf("unexpected merged local-day row: %#v", liveRows[0])
	}
	if summary.RequestCount != 2 || summary.Quota != 120 || summary.CostQuota != 50 || summary.TotalTokens != 100 {
		t.Fatalf("unexpected merged summary: %#v", summary)
	}
}

func TestQueryUsageAggregatesUsesLedgerForHourlyCoverageGap(t *testing.T) {
	db := setupUsageAggregateTestDB(t)

	const (
		timezoneOffset = int64(8 * 3600)
		localDayStart  = int64(1704124800) // 2024-01-02 00:00:00 UTC+08:00
	)
	hourlyRows := []*UsageAggregateHourly{
		{
			BucketStart:    localDayStart,
			ChannelId:      10,
			RequestedModel: "gpt-4o",
			ActualModel:    "gpt-4o",
			RequestCount:   1,
			Quota:          50,
			CostQuota:      20,
			TotalTokens:    42,
		},
		{
			BucketStart:    localDayStart + 2*usageAggregateHourSeconds,
			ChannelId:      10,
			RequestedModel: "gpt-4o",
			ActualModel:    "gpt-4o",
			RequestCount:   1,
			Quota:          30,
			CostQuota:      10,
			TotalTokens:    24,
		},
	}
	if err := db.Create(&hourlyRows).Error; err != nil {
		t.Fatalf("failed to seed hourly usage aggregates: %v", err)
	}
	jobs := []*UsageAggregationJob{
		{JobType: "hourly", BucketStart: localDayStart, Status: "success"},
		{JobType: "hourly", BucketStart: localDayStart + usageAggregateHourSeconds, Status: "failed"},
		{JobType: "hourly", BucketStart: localDayStart + 2*usageAggregateHourSeconds, Status: "success"},
	}
	if err := db.Create(&jobs).Error; err != nil {
		t.Fatalf("failed to seed hourly aggregation jobs: %v", err)
	}
	if err := db.Create(&UsageLedger{
		NewapiLogId:     20,
		Timestamp:       localDayStart + usageAggregateHourSeconds + 60,
		NewapiChannelId: 10,
		RequestedModel:  "gpt-4o",
		ActualModel:     "gpt-4o",
		Quota:           70,
		CostQuota:       30,
		TotalTokens:     58,
	}).Error; err != nil {
		t.Fatalf("failed to seed usage ledger coverage gap: %v", err)
	}

	liveRows, total, summary, err := QueryUsageAggregates(UsageAggregateQuery{
		Granularity:           UsageAggregateGranularityDay,
		Live:                  true,
		TimezoneOffsetSeconds: timezoneOffset,
		StartTimestamp:        localDayStart,
		EndTimestamp:          localDayStart + 3*usageAggregateHourSeconds,
	})
	if err != nil {
		t.Fatalf("failed to query usage aggregates: %v", err)
	}
	if total != 1 || len(liveRows) != 1 {
		t.Fatalf("expected one local-day row after filling hourly coverage gap, got total=%d rows=%d", total, len(liveRows))
	}
	if liveRows[0].BucketStart != localDayStart || liveRows[0].RequestCount != 3 || liveRows[0].Quota != 150 || liveRows[0].CostQuota != 60 || liveRows[0].TotalTokens != 124 {
		t.Fatalf("unexpected coverage-gap row: %#v", liveRows[0])
	}
	if summary.RequestCount != 3 || summary.Quota != 150 || summary.CostQuota != 60 || summary.TotalTokens != 124 {
		t.Fatalf("unexpected coverage-gap summary: %#v", summary)
	}
}

func TestUsageAggregateRollupAndExport(t *testing.T) {
	db := setupUsageAggregateTestDB(t)
	dayStart := int64(172800)

	rows := []*UsageLedger{
		{
			NewapiLogId:       1,
			Timestamp:         dayStart + 7201,
			NewapiChannelId:   7,
			ChannelName:       "primary",
			VendorProfileCode: "ANTHROPIC-API",
			ProviderKeyId:     101,
			TokenId:           201,
			RequestedModel:    "claude-sonnet",
			ActualModel:       "claude-sonnet",
			Quota:             10,
			CostQuota:         4,
			InputTokens:       6,
			OutputTokens:      2,
			TotalTokens:       8,
		},
		{
			NewapiLogId:       2,
			Timestamp:         dayStart + 10801,
			NewapiChannelId:   8,
			ChannelName:       "secondary",
			VendorProfileCode: "XAI-API",
			ProviderKeyId:     102,
			TokenId:           202,
			RequestedModel:    "grok-4",
			ActualModel:       "grok-4-fast",
			Quota:             30,
			CostQuota:         12,
			InputTokens:       10,
			OutputTokens:      5,
			TotalTokens:       15,
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
	if dailyRows[0].VendorProfileCode != "XAI-API" {
		t.Fatalf("expected daily aggregate vendor profile code to roll up, got %#v", dailyRows[0])
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
