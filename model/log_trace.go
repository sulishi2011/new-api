package model

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/tracestore"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

const (
	LogTraceStatusPending      = "pending"
	LogTraceStatusStored       = "stored"
	LogTraceStatusUploadFailed = "upload_failed"
	LogTraceStatusExpired      = "expired"
)

var ErrLogTraceNotFound = errors.New("log trace not found")

type LogTrace struct {
	Id                int    `json:"id"`
	LogId             int    `json:"log_id" gorm:"uniqueIndex;not null"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt         int64  `json:"updated_at" gorm:"bigint"`
	ExpiresAt         int64  `json:"expires_at" gorm:"bigint;index:idx_log_traces_status_expires,priority:2"`
	Status            string `json:"status" gorm:"type:varchar(32);index:idx_log_traces_status_expires,priority:1;default:'pending'"`
	Backend           string `json:"backend" gorm:"type:varchar(32);default:'s3'"`
	Bucket            string `json:"bucket" gorm:"type:varchar(255);default:''"`
	ObjectKey         string `json:"object_key" gorm:"type:text"`
	ObjectSize        int64  `json:"object_size" gorm:"default:0"`
	RawSize           int64  `json:"raw_size" gorm:"default:0"`
	RequestBodySize   int64  `json:"request_body_size" gorm:"default:0"`
	ResponseBodySize  int64  `json:"response_body_size" gorm:"default:0"`
	RequestTruncated  bool   `json:"request_truncated" gorm:"default:false"`
	ResponseTruncated bool   `json:"response_truncated" gorm:"default:false"`
	StatusCode        int    `json:"status_code" gorm:"default:0"`
	ErrorMessage      string `json:"error_message" gorm:"type:text"`
}

type logTraceArchive struct {
	Version           int         `json:"version"`
	LogId             int         `json:"log_id"`
	RequestId         string      `json:"request_id,omitempty"`
	ExternalRequestId string      `json:"external_request_id,omitempty"`
	UpstreamRequestId string      `json:"upstream_request_id,omitempty"`
	CreatedAt         int64       `json:"created_at"`
	StoredAt          int64       `json:"stored_at"`
	Trace             interface{} `json:"trace"`
}

type logTraceBatchArchive struct {
	Version  int               `json:"version"`
	Format   string            `json:"format"`
	StoredAt int64             `json:"stored_at"`
	Count    int               `json:"count"`
	Traces   []logTraceArchive `json:"traces"`
}

type traceMetadata struct {
	RequestBodySize   int64
	ResponseBodySize  int64
	RequestTruncated  bool
	ResponseTruncated bool
	StatusCode        int
	UpstreamRequestId string
}

type logTraceBatchItem struct {
	LogId             int
	RequestId         string
	ExternalRequestId string
	UpstreamRequestId string
	CreatedAt         int64
	Trace             interface{}
}

type preparedLogTraceBatchItem struct {
	Archive logTraceArchive
	Record  LogTrace
	RawSize int64
}

var (
	logTraceBatchOnce    sync.Once
	logTraceBatchQueue   chan logTraceBatchItem
	logTraceBatchDropped atomic.Int64
)

func prepareLogTrace(logType int, other map[string]interface{}) interface{} {
	if other == nil {
		return nil
	}
	trace, ok := other["trace"]
	if !ok || trace == nil {
		return nil
	}
	if !common.TraceStorageEnabled || !tracestore.IsConfigured() {
		return nil
	}

	delete(other, "trace")
	if !shouldCaptureLogTrace(logType) {
		return nil
	}
	other["trace_ref"] = map[string]interface{}{
		"backend": "s3",
		"status":  LogTraceStatusPending,
	}
	return trace
}

func shouldCaptureLogTrace(logType int) bool {
	switch strings.ToLower(strings.TrimSpace(common.TraceCaptureMode)) {
	case "off", "none", "disabled":
		return false
	case "all":
		return true
	case "sample":
		if logType == LogTypeError {
			return true
		}
		rate := common.TraceSuccessSampleRate
		if rate <= 0 {
			return false
		}
		if rate >= 100 {
			return true
		}
		return rand.Intn(100) < rate
	case "", "error", "errors":
		return logType == LogTypeError
	default:
		return logType == LogTypeError
	}
}

func persistLogTrace(ctx context.Context, log *Log, trace interface{}) {
	if log == nil || log.Id == 0 || trace == nil || !common.TraceStorageEnabled || !tracestore.IsConfigured() {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if common.TraceUploadBatchEnabled {
		enqueueLogTraceBatch(log, trace)
		return
	}

	run := func() {
		uploadCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := persistLogTraceSync(uploadCtx, log, trace); err != nil {
			common.SysLog(fmt.Sprintf("failed to persist log trace: log_id=%d, error=%v", log.Id, err))
		}
	}
	if common.TraceUploadAsync && !(log.Type == LogTypeError && common.TraceUploadSyncForError) {
		gopool.Go(run)
		return
	}

	uploadCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := persistLogTraceSync(uploadCtx, log, trace); err != nil {
		common.SysLog(fmt.Sprintf("failed to persist log trace: log_id=%d, error=%v", log.Id, err))
	}
}

func enqueueLogTraceBatch(log *Log, trace interface{}) {
	queue := getLogTraceBatchQueue()
	if queue == nil {
		return
	}
	item := logTraceBatchItem{
		LogId:             log.Id,
		RequestId:         log.RequestId,
		ExternalRequestId: log.ExternalRequestId,
		UpstreamRequestId: log.UpstreamRequestId,
		CreatedAt:         log.CreatedAt,
		Trace:             trace,
	}
	select {
	case queue <- item:
	default:
		dropped := logTraceBatchDropped.Add(1)
		if dropped == 1 || dropped%1000 == 0 {
			common.SysLog(fmt.Sprintf("trace batch queue full, dropped trace count=%d", dropped))
		}
	}
}

func getLogTraceBatchQueue() chan logTraceBatchItem {
	logTraceBatchOnce.Do(func() {
		size := common.TraceUploadQueueSize
		if size <= 0 {
			size = 1000
		}
		logTraceBatchQueue = make(chan logTraceBatchItem, size)
		gopool.Go(runLogTraceBatchWorker)
	})
	return logTraceBatchQueue
}

func runLogTraceBatchWorker() {
	batchSize := common.TraceUploadBatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	flushInterval := time.Duration(common.TraceUploadBatchFlushIntervalSeconds) * time.Second
	if flushInterval <= 0 {
		flushInterval = 2 * time.Second
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]logTraceBatchItem, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		items := batch
		batch = make([]logTraceBatchItem, 0, batchSize)
		uploadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := persistLogTraceBatchSync(uploadCtx, items); err != nil {
			common.SysLog(fmt.Sprintf("failed to persist log trace batch: count=%d, error=%v", len(items), err))
		}
	}

	for {
		select {
		case item := <-logTraceBatchQueue:
			batch = append(batch, item)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func persistLogTraceBatchSync(ctx context.Context, items []logTraceBatchItem) error {
	if len(items) == 0 {
		return nil
	}
	maxBytes := common.TraceUploadBatchMaxBytes
	if maxBytes <= 0 {
		maxBytes = 8 << 20
	}

	var chunk []preparedLogTraceBatchItem
	chunkBytes := 0
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		if err := persistPreparedLogTraceBatch(ctx, chunk); err != nil {
			return err
		}
		chunk = nil
		chunkBytes = 0
		return nil
	}

	for _, item := range items {
		prepared, err := prepareLogTraceBatchItem(item)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to prepare log trace batch item: log_id=%d, error=%v", item.LogId, err))
			continue
		}
		itemBytes := int(prepared.RawSize)
		if len(chunk) > 0 && chunkBytes+itemBytes > maxBytes {
			if err := flush(); err != nil {
				return err
			}
		}
		chunk = append(chunk, prepared)
		chunkBytes += itemBytes
	}
	return flush()
}

func prepareLogTraceBatchItem(item logTraceBatchItem) (preparedLogTraceBatchItem, error) {
	now := common.GetTimestamp()
	metadata := extractTraceMetadata(item.Trace)
	upstreamRequestId := item.UpstreamRequestId
	if upstreamRequestId == "" && metadata.UpstreamRequestId != "" {
		upstreamRequestId = metadata.UpstreamRequestId
	}
	archive := logTraceArchive{
		Version:           1,
		LogId:             item.LogId,
		RequestId:         item.RequestId,
		ExternalRequestId: item.ExternalRequestId,
		UpstreamRequestId: upstreamRequestId,
		CreatedAt:         item.CreatedAt,
		StoredAt:          now,
		Trace:             item.Trace,
	}
	raw, err := common.Marshal(archive)
	if err != nil {
		return preparedLogTraceBatchItem{}, err
	}
	record := LogTrace{
		LogId:             item.LogId,
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         item.CreatedAt + int64(common.TraceRetentionDays)*86400,
		Status:            LogTraceStatusPending,
		Backend:           strings.ToLower(strings.TrimSpace(common.TraceStorageBackend)),
		Bucket:            common.TraceS3Bucket,
		RawSize:           int64(len(raw)),
		RequestBodySize:   metadata.RequestBodySize,
		ResponseBodySize:  metadata.ResponseBodySize,
		RequestTruncated:  metadata.RequestTruncated,
		ResponseTruncated: metadata.ResponseTruncated,
		StatusCode:        metadata.StatusCode,
	}
	if record.Backend == "" {
		record.Backend = "s3"
	}
	return preparedLogTraceBatchItem{
		Archive: archive,
		Record:  record,
		RawSize: int64(len(raw)),
	}, nil
}

func persistPreparedLogTraceBatch(ctx context.Context, items []preparedLogTraceBatchItem) error {
	if len(items) == 0 {
		return nil
	}
	now := common.GetTimestamp()
	objectKey := buildLogTraceBatchObjectKey(time.Now().UTC())
	archives := make([]logTraceArchive, 0, len(items))
	records := make([]LogTrace, 0, len(items))
	logIds := make([]int, 0, len(items))
	for _, item := range items {
		archive := item.Archive
		archive.StoredAt = now
		record := item.Record
		record.ObjectKey = objectKey
		record.UpdatedAt = now
		archives = append(archives, archive)
		records = append(records, record)
		logIds = append(logIds, record.LogId)
	}
	raw, err := common.Marshal(logTraceBatchArchive{
		Version:  1,
		Format:   "log_trace_batch",
		StoredAt: now,
		Count:    len(archives),
		Traces:   archives,
	})
	if err != nil {
		return err
	}
	compressed, err := gzipBytes(raw)
	if err != nil {
		return err
	}
	if err := LOG_DB.Create(&records).Error; err != nil {
		return err
	}
	if err := tracestore.Put(ctx, objectKey, compressed); err != nil {
		updateErr := LOG_DB.Model(&LogTrace{}).Where("log_id IN ?", logIds).Updates(map[string]interface{}{
			"status":        LogTraceStatusUploadFailed,
			"updated_at":    common.GetTimestamp(),
			"error_message": truncateLogTraceError(err.Error()),
		}).Error
		if updateErr != nil {
			return fmt.Errorf("%w; failed to update trace batch status: %v", err, updateErr)
		}
		return err
	}
	return LOG_DB.Model(&LogTrace{}).Where("log_id IN ?", logIds).Updates(map[string]interface{}{
		"status":      LogTraceStatusStored,
		"updated_at":  common.GetTimestamp(),
		"object_size": int64(len(compressed)),
	}).Error
}

func persistLogTraceSync(ctx context.Context, log *Log, trace interface{}) error {
	now := common.GetTimestamp()
	metadata := extractTraceMetadata(trace)
	if log.UpstreamRequestId == "" && metadata.UpstreamRequestId != "" {
		log.UpstreamRequestId = metadata.UpstreamRequestId
	}
	archive := logTraceArchive{
		Version:           1,
		LogId:             log.Id,
		RequestId:         log.RequestId,
		ExternalRequestId: log.ExternalRequestId,
		UpstreamRequestId: log.UpstreamRequestId,
		CreatedAt:         log.CreatedAt,
		StoredAt:          now,
		Trace:             trace,
	}
	raw, err := common.Marshal(archive)
	if err != nil {
		return err
	}
	compressed, err := gzipBytes(raw)
	if err != nil {
		return err
	}

	record := &LogTrace{
		LogId:             log.Id,
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         log.CreatedAt + int64(common.TraceRetentionDays)*86400,
		Status:            LogTraceStatusPending,
		Backend:           strings.ToLower(strings.TrimSpace(common.TraceStorageBackend)),
		Bucket:            common.TraceS3Bucket,
		ObjectKey:         buildLogTraceObjectKey(log),
		RawSize:           int64(len(raw)),
		RequestBodySize:   metadata.RequestBodySize,
		ResponseBodySize:  metadata.ResponseBodySize,
		RequestTruncated:  metadata.RequestTruncated,
		ResponseTruncated: metadata.ResponseTruncated,
		StatusCode:        metadata.StatusCode,
	}
	if record.Backend == "" {
		record.Backend = "s3"
	}
	if err := LOG_DB.Create(record).Error; err != nil {
		return err
	}

	if err := tracestore.Put(ctx, record.ObjectKey, compressed); err != nil {
		updateErr := LOG_DB.Model(&LogTrace{}).Where("id = ?", record.Id).Updates(map[string]interface{}{
			"status":        LogTraceStatusUploadFailed,
			"updated_at":    common.GetTimestamp(),
			"error_message": truncateLogTraceError(err.Error()),
		}).Error
		if updateErr != nil {
			return fmt.Errorf("%w; failed to update trace status: %v", err, updateErr)
		}
		return err
	}

	return LOG_DB.Model(&LogTrace{}).Where("id = ?", record.Id).Updates(map[string]interface{}{
		"status":      LogTraceStatusStored,
		"updated_at":  common.GetTimestamp(),
		"object_size": int64(len(compressed)),
	}).Error
}

func gzipBytes(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(raw); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gunzipBytes(raw []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func buildLogTraceObjectKey(log *Log) string {
	t := time.Unix(log.CreatedAt, 0).UTC()
	name := fmt.Sprintf("%d", log.Id)
	if part := sanitizeTraceKeyPart(log.RequestId); part != "" {
		name += "-" + part
	}
	key := fmt.Sprintf("%04d/%02d/%02d/%s.json.gz", t.Year(), t.Month(), t.Day(), name)
	prefix := strings.Trim(common.TraceS3Prefix, "/")
	if prefix == "" {
		return key
	}
	return path.Join(prefix, key)
}

func buildLogTraceBatchObjectKey(t time.Time) string {
	name := fmt.Sprintf("%02d%02d%02d-%09d-%s", t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), common.GetUUID())
	key := fmt.Sprintf("%04d/%02d/%02d/batch/%s.json.gz", t.Year(), t.Month(), t.Day(), name)
	prefix := strings.Trim(common.TraceS3Prefix, "/")
	if prefix == "" {
		return key
	}
	return path.Join(prefix, key)
}

func sanitizeTraceKeyPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
		if builder.Len() >= 96 {
			break
		}
	}
	return builder.String()
}

func truncateLogTraceError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 1024 {
		return value
	}
	return value[:1024]
}

func extractTraceMetadata(trace interface{}) traceMetadata {
	var metadata traceMetadata
	raw, err := common.Marshal(trace)
	if err != nil {
		return metadata
	}
	var data map[string]interface{}
	if err := common.Unmarshal(raw, &data); err != nil {
		return metadata
	}
	metadata.StatusCode = intFromInterface(data["status_code"])
	metadata.UpstreamRequestId, _ = data["upstream_request_id"].(string)
	if request, ok := data["request"].(map[string]interface{}); ok {
		metadata.RequestBodySize = int64FromInterface(request["body_size"])
		metadata.RequestTruncated = boolFromInterface(request["truncated"])
	}
	if response, ok := data["response"].(map[string]interface{}); ok {
		metadata.ResponseBodySize = int64FromInterface(response["body_size"])
		metadata.ResponseTruncated = boolFromInterface(response["truncated"])
	}
	return metadata
}

func int64FromInterface(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case uint:
		return int64(v)
	case uint64:
		return int64(v)
	case uint32:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return 0
	}
}

func boolFromInterface(value interface{}) bool {
	v, _ := value.(bool)
	return v
}

func GetLogTracePayload(ctx context.Context, logId int) (map[string]interface{}, error) {
	if logId <= 0 {
		return nil, ErrLogTraceNotFound
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var trace LogTrace
	err := logReadDB().WithContext(ctx).Where("log_id = ?", logId).First(&trace).Error
	if err == nil {
		if trace.Status != LogTraceStatusStored || trace.ObjectKey == "" {
			return nil, fmt.Errorf("log trace status is %s", trace.Status)
		}
		body, err := tracestore.Get(ctx, trace.ObjectKey)
		if err != nil {
			return nil, err
		}
		raw, err := gunzipBytes(body)
		if err != nil {
			return nil, err
		}
		return decodeStoredLogTracePayload(raw, logId)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var log Log
	if err := logReadDB().WithContext(ctx).Select("id", "created_at", "request_id", "external_request_id", "upstream_request_id", "other").Where("id = ?", logId).First(&log).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLogTraceNotFound
		}
		return nil, err
	}
	other, err := common.StrToMap(log.Other)
	if err != nil {
		return nil, err
	}
	tracePayload, ok := other["trace"]
	if !ok || tracePayload == nil {
		return nil, ErrLogTraceNotFound
	}
	return map[string]interface{}{
		"version":             1,
		"log_id":              log.Id,
		"request_id":          log.RequestId,
		"external_request_id": log.ExternalRequestId,
		"upstream_request_id": log.UpstreamRequestId,
		"created_at":          log.CreatedAt,
		"storage":             "inline",
		"trace":               tracePayload,
	}, nil
}

func decodeStoredLogTracePayload(raw []byte, logId int) (map[string]interface{}, error) {
	var payload map[string]interface{}
	if err := common.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if traces, ok := payload["traces"].([]interface{}); ok {
		for _, item := range traces {
			tracePayload, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if intFromInterface(tracePayload["log_id"]) == logId {
				return tracePayload, nil
			}
		}
		return nil, ErrLogTraceNotFound
	}
	return payload, nil
}

func CleanupExpiredLogTraces(ctx context.Context, now int64, limit int) (int64, error) {
	if limit <= 0 {
		limit = 100
	}
	var ids []int
	err := LOG_DB.WithContext(ctx).Model(&LogTrace{}).
		Where("expires_at > 0 AND expires_at < ? AND status IN ?", now, []string{LogTraceStatusStored, LogTraceStatusPending, LogTraceStatusUploadFailed}).
		Order("expires_at asc, id asc").
		Limit(limit).
		Pluck("id", &ids).Error
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := LOG_DB.WithContext(ctx).Model(&LogTrace{}).Where("id IN ?", ids).Updates(map[string]interface{}{
		"status":     LogTraceStatusExpired,
		"updated_at": common.GetTimestamp(),
	})
	return result.RowsAffected, result.Error
}
