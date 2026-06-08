package model

import (
	"errors"
	"testing"
	"time"
)

func seedMonthlyLog(t *testing.T, idSuffix int64, log *Log) {
	t.Helper()

	tableName, err := ensureLogTableForTimestamp(log.CreatedAt)
	if err != nil {
		t.Fatalf("failed to ensure monthly log table: %v", err)
	}
	if log.Id == 0 {
		log.Id = log.CreatedAt*logIDTimeFactor + idSuffix
	}
	if err := LOG_DB.Table(tableName).Create(log).Error; err != nil {
		t.Fatalf("failed to seed monthly log: %v", err)
	}
}

func TestGetAllLogsSupportsAdjacentMonthlyShardRange(t *testing.T) {
	setupDashboardUsageTestDB(t)

	june30 := time.Date(2026, 6, 30, 23, 59, 50, 0, time.UTC).Unix()
	july1 := time.Date(2026, 7, 1, 0, 0, 10, 0, time.UTC).Unix()
	may31 := time.Date(2026, 5, 31, 23, 59, 50, 0, time.UTC).Unix()

	seedMonthlyLog(t, 1, &Log{
		UserId:           1,
		Username:         "admin",
		CreatedAt:        june30,
		Type:             LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            10,
		PromptTokens:     3,
		CompletionTokens: 4,
	})
	seedMonthlyLog(t, 2, &Log{
		UserId:           1,
		Username:         "admin",
		CreatedAt:        july1,
		Type:             LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            20,
		PromptTokens:     5,
		CompletionTokens: 6,
	})

	logs, total, err := GetAllLogs(LogTypeConsume, june30-10, july1+10, "gpt-4o", "", "", 0, 10, 0, "", "", 0)
	if err != nil {
		t.Fatalf("failed to query adjacent monthly logs: %v", err)
	}
	if total != 2 || len(logs) != 2 {
		t.Fatalf("expected two logs, got total=%d len=%d", total, len(logs))
	}
	if logs[0].CreatedAt != july1 || logs[1].CreatedAt != june30 {
		t.Fatalf("logs are not globally sorted across shards: %#v", logs)
	}

	logs, total, err = GetAllLogs(LogTypeConsume, june30-10, july1+10, "gpt-4o", "", "", 1, 1, 0, "", "", 0)
	if err != nil {
		t.Fatalf("failed to query adjacent monthly logs with offset: %v", err)
	}
	if total != 2 || len(logs) != 1 || logs[0].CreatedAt != june30 {
		t.Fatalf("unexpected paged result total=%d logs=%#v", total, logs)
	}

	stat, err := SumUsedQuota(LogTypeConsume, june30-10, july1+10, "gpt-4o", "", "", 0, "", "", 0, 0, "", "", "", "")
	if err != nil {
		t.Fatalf("failed to sum adjacent monthly quota: %v", err)
	}
	if stat.Quota != 30 {
		t.Fatalf("expected quota sum 30, got %#v", stat)
	}
	if token := SumUsedToken(LogTypeConsume, june30-10, july1+10, "gpt-4o", "", ""); token != 18 {
		t.Fatalf("expected token sum 18, got %d", token)
	}

	_, _, err = GetAllLogs(LogTypeConsume, may31, july1+10, "gpt-4o", "", "", 0, 10, 0, "", "", 0)
	if !errors.Is(err, ErrLogCrossMonthQuery) {
		t.Fatalf("expected cross-month limit error, got %v", err)
	}
}

func TestDashboardUsageFallbackMergesAdjacentMonthlyShardRows(t *testing.T) {
	setupDashboardUsageTestDB(t)

	june30 := time.Date(2026, 6, 30, 23, 30, 0, 0, time.UTC).Unix()
	july1 := time.Date(2026, 7, 1, 0, 15, 0, 0, time.UTC).Unix()
	juneBucket := time.Date(2026, 6, 30, 23, 0, 0, 0, time.UTC).Unix()
	julyBucket := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).Unix()

	seedMonthlyLog(t, 1, &Log{
		UserId:           1,
		Username:         "admin",
		CreatedAt:        june30,
		Type:             LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            10,
		PromptTokens:     3,
		CompletionTokens: 4,
	})
	seedMonthlyLog(t, 2, &Log{
		UserId:           1,
		Username:         "admin",
		CreatedAt:        july1,
		Type:             LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            20,
		PromptTokens:     5,
		CompletionTokens: 6,
	})

	rows, err := GetDashboardQuotaData(DashboardUsageQuery{
		StartTimestamp: june30 - 60,
		EndTimestamp:   july1 + 60,
		ModelName:      "gpt-4o",
		Dimension:      DashboardDimensionModel,
		Metric:         DashboardMetricOriginal,
	})
	if err != nil {
		t.Fatalf("failed to query dashboard rows across adjacent shards: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two dashboard rows, got %d: %#v", len(rows), rows)
	}

	rowMap := make(map[int64]*QuotaData, len(rows))
	for _, row := range rows {
		rowMap[row.CreatedAt] = row
	}
	if rowMap[juneBucket] == nil || rowMap[juneBucket].Quota != 10 || rowMap[juneBucket].TokenUsed != 7 {
		t.Fatalf("unexpected June bucket row: %#v", rowMap[juneBucket])
	}
	if rowMap[julyBucket] == nil || rowMap[julyBucket].Quota != 20 || rowMap[julyBucket].TokenUsed != 11 {
		t.Fatalf("unexpected July bucket row: %#v", rowMap[julyBucket])
	}
}
