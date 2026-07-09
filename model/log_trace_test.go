package model

import (
	"context"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLogTraceTestDB(t *testing.T) *gorm.DB {
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

	if err := db.AutoMigrate(&Log{}, &LogTrace{}); err != nil {
		t.Fatalf("failed to migrate log tables: %v", err)
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

func TestPrepareLogTraceKeepsInlineWhenStoreDisabled(t *testing.T) {
	oldEnabled := common.TraceStorageEnabled
	t.Cleanup(func() {
		common.TraceStorageEnabled = oldEnabled
	})
	common.TraceStorageEnabled = false

	other := map[string]interface{}{
		"trace": map[string]interface{}{"request": "body"},
	}
	trace := prepareLogTrace(LogTypeError, other)
	if trace != nil {
		t.Fatalf("expected nil trace when trace storage is disabled, got %#v", trace)
	}
	if _, ok := other["trace"]; !ok {
		t.Fatalf("expected inline trace to remain when store is disabled")
	}
}

func TestShouldCaptureLogTraceModes(t *testing.T) {
	oldMode := common.TraceCaptureMode
	oldRate := common.TraceSuccessSampleRate
	t.Cleanup(func() {
		common.TraceCaptureMode = oldMode
		common.TraceSuccessSampleRate = oldRate
	})

	common.TraceCaptureMode = "error"
	common.TraceSuccessSampleRate = 0
	if !shouldCaptureLogTrace(LogTypeError) {
		t.Fatalf("expected error trace to be captured in error mode")
	}
	if shouldCaptureLogTrace(LogTypeConsume) {
		t.Fatalf("expected consume trace to be skipped in error mode")
	}

	common.TraceCaptureMode = "all"
	if !shouldCaptureLogTrace(LogTypeConsume) {
		t.Fatalf("expected consume trace to be captured in all mode")
	}

	common.TraceCaptureMode = "off"
	if shouldCaptureLogTrace(LogTypeError) {
		t.Fatalf("expected trace to be skipped in off mode")
	}

	common.TraceCaptureMode = "sample"
	common.TraceSuccessSampleRate = 100
	if !shouldCaptureLogTrace(LogTypeConsume) {
		t.Fatalf("expected consume trace to be captured with 100 percent sample rate")
	}
	common.TraceSuccessSampleRate = 0
	if shouldCaptureLogTrace(LogTypeConsume) {
		t.Fatalf("expected consume trace to be skipped with 0 percent sample rate")
	}
}

func TestDecodeStoredLogTracePayloadSupportsBatch(t *testing.T) {
	batch := logTraceBatchArchive{
		Version:  1,
		Format:   "log_trace_batch",
		StoredAt: 300,
		Count:    2,
		Traces: []logTraceArchive{
			{
				Version:   1,
				LogId:     10,
				RequestId: "req-10",
				CreatedAt: 100,
				StoredAt:  300,
				Trace:     map[string]interface{}{"status_code": 200},
			},
			{
				Version:   1,
				LogId:     11,
				RequestId: "req-11",
				CreatedAt: 101,
				StoredAt:  300,
				Trace: map[string]interface{}{
					"status_code": float64(500),
					"response": map[string]interface{}{
						"body": "failed",
					},
				},
			},
		},
	}
	raw, err := common.Marshal(batch)
	if err != nil {
		t.Fatalf("failed to marshal batch: %v", err)
	}

	payload, err := decodeStoredLogTracePayload(raw, 11)
	if err != nil {
		t.Fatalf("failed to decode batch payload: %v", err)
	}
	if payload["log_id"] != float64(11) {
		t.Fatalf("unexpected log id: %#v", payload["log_id"])
	}
	tracePayload, ok := payload["trace"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected trace payload, got %#v", payload["trace"])
	}
	if intFromInterface(tracePayload["status_code"]) != 500 {
		t.Fatalf("unexpected status code: %#v", tracePayload["status_code"])
	}

	if _, err := decodeStoredLogTracePayload(raw, 12); err != ErrLogTraceNotFound {
		t.Fatalf("expected ErrLogTraceNotFound for missing batch item, got %v", err)
	}
}

func TestDecodeStoredLogTracePayloadSupportsSingleArchive(t *testing.T) {
	archive := logTraceArchive{
		Version:   1,
		LogId:     12,
		RequestId: "req-12",
		CreatedAt: 100,
		StoredAt:  300,
		Trace: map[string]interface{}{
			"status_code": 200,
		},
	}
	raw, err := common.Marshal(archive)
	if err != nil {
		t.Fatalf("failed to marshal archive: %v", err)
	}

	payload, err := decodeStoredLogTracePayload(raw, 12)
	if err != nil {
		t.Fatalf("failed to decode archive payload: %v", err)
	}
	if payload["request_id"] != "req-12" {
		t.Fatalf("unexpected request id: %#v", payload["request_id"])
	}
}

func TestPrepareLogTraceBatchItemPreservesMetadata(t *testing.T) {
	item := logTraceBatchItem{
		LogId:     20,
		RequestId: "req-20",
		CreatedAt: 100,
		Trace: map[string]interface{}{
			"status_code":         429,
			"upstream_request_id": "upstream-20",
			"request": map[string]interface{}{
				"body_size": float64(10),
				"truncated": true,
			},
			"response": map[string]interface{}{
				"body_size": float64(20),
			},
		},
	}

	prepared, err := prepareLogTraceBatchItem(item)
	if err != nil {
		t.Fatalf("failed to prepare batch item: %v", err)
	}
	if prepared.Archive.UpstreamRequestId != "upstream-20" {
		t.Fatalf("unexpected upstream request id: %s", prepared.Archive.UpstreamRequestId)
	}
	if prepared.Record.StatusCode != 429 {
		t.Fatalf("unexpected status code: %d", prepared.Record.StatusCode)
	}
	if prepared.Record.RequestBodySize != 10 || prepared.Record.ResponseBodySize != 20 {
		t.Fatalf("unexpected body sizes: %#v", prepared.Record)
	}
	if !prepared.Record.RequestTruncated {
		t.Fatalf("expected request truncation metadata")
	}
}

func TestCleanupExpiredLogTracesDeletesRows(t *testing.T) {
	db := setupLogTraceTestDB(t)

	traces := []LogTrace{
		{LogId: 1, ExpiresAt: 500, Status: LogTraceStatusStored},
		{LogId: 2, ExpiresAt: 600, Status: LogTraceStatusExpired},
		{LogId: 3, ExpiresAt: 1500, Status: LogTraceStatusStored},
		{LogId: 4, ExpiresAt: 0, Status: LogTraceStatusPending},
	}
	if err := db.Create(&traces).Error; err != nil {
		t.Fatalf("failed to seed log traces: %v", err)
	}

	count, err := CleanupExpiredLogTraces(context.Background(), 1000, 10)
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 deleted rows, got %d", count)
	}

	var remaining []LogTrace
	if err := db.Order("log_id asc").Find(&remaining).Error; err != nil {
		t.Fatalf("failed to query remaining traces: %v", err)
	}
	if len(remaining) != 2 || remaining[0].LogId != 3 || remaining[1].LogId != 4 {
		t.Fatalf("unexpected remaining traces: %#v", remaining)
	}

	count, err = CleanupExpiredLogTraces(context.Background(), 1000, 10)
	if err != nil {
		t.Fatalf("second cleanup failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no rows on second cleanup, got %d", count)
	}
}

func TestDeleteLogsByTypeBeforeDisabledForShardedLogs(t *testing.T) {
	setupLogTraceTestDB(t)

	count, err := DeleteLogsByTypeBefore(context.Background(), LogTypeConsume, 100, 10)
	if err == nil {
		t.Fatal("expected sharded log cleanup to be disabled")
	}
	if count != 0 {
		t.Fatalf("expected no deleted logs, got %d", count)
	}
}
