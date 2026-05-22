package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	UsageAggregateGranularityHour = "hour"
	UsageAggregateGranularityDay  = "day"

	usageAggregateHourSeconds = int64(3600)
	usageAggregateDaySeconds  = int64(86400)
)

type UsageAggregateHourly struct {
	Id                 int    `json:"id"`
	BucketStart        int64  `json:"bucket_start" gorm:"bigint;uniqueIndex:idx_usage_aggregate_hourly_dim,priority:1;index"`
	ChannelId          int    `json:"channel_id" gorm:"default:0;uniqueIndex:idx_usage_aggregate_hourly_dim,priority:2;index"`
	ChannelName        string `json:"channel_name" gorm:"type:varchar(191);default:''"`
	VendorProfileCode  string `json:"vendor_profile_code" gorm:"type:varchar(64);default:''"`
	ProviderKeyId      int    `json:"provider_key_id" gorm:"default:0;uniqueIndex:idx_usage_aggregate_hourly_dim,priority:3;index"`
	ProviderKeyPreview string `json:"provider_key_preview" gorm:"type:varchar(255);default:''"`
	TokenId            int    `json:"token_id" gorm:"default:0;uniqueIndex:idx_usage_aggregate_hourly_dim,priority:4;index"`
	TokenName          string `json:"token_name" gorm:"type:varchar(191);default:''"`
	RequestedModel     string `json:"requested_model" gorm:"type:varchar(128);uniqueIndex:idx_usage_aggregate_hourly_dim,priority:5;index"`
	ActualModel        string `json:"actual_model" gorm:"type:varchar(128);uniqueIndex:idx_usage_aggregate_hourly_dim,priority:6;index"`
	RequestCount       int    `json:"request_count" gorm:"default:0"`
	Quota              int    `json:"quota" gorm:"default:0"`
	CostQuota          int    `json:"cost_quota" gorm:"default:0"`
	InputTokens        int    `json:"input_tokens" gorm:"default:0"`
	OutputTokens       int    `json:"output_tokens" gorm:"default:0"`
	CacheReadTokens    int    `json:"cache_read_tokens" gorm:"default:0"`
	CacheWriteTokens   int    `json:"cache_write_tokens" gorm:"default:0"`
	TotalTokens        int    `json:"total_tokens" gorm:"default:0"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint"`
}

func (UsageAggregateHourly) TableName() string {
	return "usage_aggregate_hourly"
}

type UsageAggregateDaily struct {
	Id                 int    `json:"id"`
	BucketStart        int64  `json:"bucket_start" gorm:"bigint;uniqueIndex:idx_usage_aggregate_daily_dim,priority:1;index"`
	ChannelId          int    `json:"channel_id" gorm:"default:0;uniqueIndex:idx_usage_aggregate_daily_dim,priority:2;index"`
	ChannelName        string `json:"channel_name" gorm:"type:varchar(191);default:''"`
	VendorProfileCode  string `json:"vendor_profile_code" gorm:"type:varchar(64);default:''"`
	ProviderKeyId      int    `json:"provider_key_id" gorm:"default:0;uniqueIndex:idx_usage_aggregate_daily_dim,priority:3;index"`
	ProviderKeyPreview string `json:"provider_key_preview" gorm:"type:varchar(255);default:''"`
	TokenId            int    `json:"token_id" gorm:"default:0;uniqueIndex:idx_usage_aggregate_daily_dim,priority:4;index"`
	TokenName          string `json:"token_name" gorm:"type:varchar(191);default:''"`
	RequestedModel     string `json:"requested_model" gorm:"type:varchar(128);uniqueIndex:idx_usage_aggregate_daily_dim,priority:5;index"`
	ActualModel        string `json:"actual_model" gorm:"type:varchar(128);uniqueIndex:idx_usage_aggregate_daily_dim,priority:6;index"`
	RequestCount       int    `json:"request_count" gorm:"default:0"`
	Quota              int    `json:"quota" gorm:"default:0"`
	CostQuota          int    `json:"cost_quota" gorm:"default:0"`
	InputTokens        int    `json:"input_tokens" gorm:"default:0"`
	OutputTokens       int    `json:"output_tokens" gorm:"default:0"`
	CacheReadTokens    int    `json:"cache_read_tokens" gorm:"default:0"`
	CacheWriteTokens   int    `json:"cache_write_tokens" gorm:"default:0"`
	TotalTokens        int    `json:"total_tokens" gorm:"default:0"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint"`
}

func (UsageAggregateDaily) TableName() string {
	return "usage_aggregate_daily"
}

type UsageAggregationJob struct {
	Id           int    `json:"id"`
	JobType      string `json:"job_type" gorm:"type:varchar(32);index:idx_usage_aggregation_jobs_type_bucket,priority:1"`
	BucketStart  int64  `json:"bucket_start" gorm:"bigint;index:idx_usage_aggregation_jobs_type_bucket,priority:2"`
	Status       string `json:"status" gorm:"type:varchar(32);default:'running';index"`
	StartedAt    int64  `json:"started_at" gorm:"bigint"`
	FinishedAt   int64  `json:"finished_at" gorm:"bigint"`
	ErrorMessage string `json:"error" gorm:"type:text"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

func (UsageAggregationJob) TableName() string {
	return "usage_aggregation_jobs"
}

type UsageAggregateQuery struct {
	Granularity    string
	Live           bool
	StartTimestamp int64
	EndTimestamp   int64
	ChannelId      int
	ProviderKeyId  int
	TokenId        int
	RequestedModel string
	ActualModel    string
	StartIdx       int
	Limit          int
	SortBy         string
	SortOrder      string
}

type UsageAggregateSummary struct {
	RequestCount     int `json:"request_count" gorm:"column:request_count"`
	Quota            int `json:"quota" gorm:"column:quota"`
	CostQuota        int `json:"cost_quota" gorm:"column:cost_quota"`
	InputTokens      int `json:"input_tokens" gorm:"column:input_tokens"`
	OutputTokens     int `json:"output_tokens" gorm:"column:output_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens" gorm:"column:cache_read_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens" gorm:"column:cache_write_tokens"`
	TotalTokens      int `json:"total_tokens" gorm:"column:total_tokens"`
}

type UsageAggregateRow struct {
	BucketStart        int64  `json:"bucket_start" gorm:"column:bucket_start"`
	ChannelId          int    `json:"channel_id" gorm:"column:channel_id"`
	ChannelName        string `json:"channel_name" gorm:"column:channel_name"`
	VendorProfileCode  string `json:"vendor_profile_code" gorm:"column:vendor_profile_code"`
	ProviderKeyId      int    `json:"provider_key_id" gorm:"column:provider_key_id"`
	ProviderKeyPreview string `json:"provider_key_preview" gorm:"column:provider_key_preview"`
	TokenId            int    `json:"token_id" gorm:"column:token_id"`
	TokenName          string `json:"token_name" gorm:"column:token_name"`
	RequestedModel     string `json:"requested_model" gorm:"column:requested_model"`
	ActualModel        string `json:"actual_model" gorm:"column:actual_model"`
	RequestCount       int    `json:"request_count" gorm:"column:request_count"`
	Quota              int    `json:"quota" gorm:"column:quota"`
	CostQuota          int    `json:"cost_quota" gorm:"column:cost_quota"`
	InputTokens        int    `json:"input_tokens" gorm:"column:input_tokens"`
	OutputTokens       int    `json:"output_tokens" gorm:"column:output_tokens"`
	CacheReadTokens    int    `json:"cache_read_tokens" gorm:"column:cache_read_tokens"`
	CacheWriteTokens   int    `json:"cache_write_tokens" gorm:"column:cache_write_tokens"`
	TotalTokens        int    `json:"total_tokens" gorm:"column:total_tokens"`
}

func normalizeUsageAggregateGranularity(granularity string) string {
	switch granularity {
	case UsageAggregateGranularityHour:
		return UsageAggregateGranularityHour
	default:
		return UsageAggregateGranularityDay
	}
}

func UsageAggregateHourBucket(ts int64) int64 {
	if ts <= 0 {
		return 0
	}
	return ts / usageAggregateHourSeconds * usageAggregateHourSeconds
}

func UsageAggregateDayBucket(ts int64) int64 {
	if ts <= 0 {
		return 0
	}
	return time.Unix(ts, 0).UTC().Truncate(24 * time.Hour).Unix()
}

func usageAggregateTable(granularity string) string {
	if normalizeUsageAggregateGranularity(granularity) == UsageAggregateGranularityHour {
		return "usage_aggregate_hourly"
	}
	return "usage_aggregate_daily"
}

func usageAggregateBucketExpression(granularity string, column string) string {
	bucketSeconds := usageAggregateDaySeconds
	if normalizeUsageAggregateGranularity(granularity) == UsageAggregateGranularityHour {
		bucketSeconds = usageAggregateHourSeconds
	}
	if common.UsingMySQL {
		return fmt.Sprintf("(FLOOR(%s / %d) * %d)", column, bucketSeconds, bucketSeconds)
	}
	return fmt.Sprintf("((%s / %d) * %d)", column, bucketSeconds, bucketSeconds)
}

func aggregateSelectFields(bucketExpr string) string {
	return strings.Join([]string{
		bucketExpr + " AS bucket_start",
		"newapi_channel_id AS channel_id",
		"COALESCE(MAX(channel_name), '') AS channel_name",
		"COALESCE(MAX(vendor_profile_code), '') AS vendor_profile_code",
		"provider_key_id",
		"COALESCE(MAX(provider_key_preview), '') AS provider_key_preview",
		"token_id",
		"COALESCE(MAX(token_name), '') AS token_name",
		"requested_model",
		"actual_model",
		"COUNT(*) AS request_count",
		"COALESCE(SUM(quota), 0) AS quota",
		"COALESCE(SUM(cost_quota), 0) AS cost_quota",
		"COALESCE(SUM(input_tokens), 0) AS input_tokens",
		"COALESCE(SUM(output_tokens), 0) AS output_tokens",
		"COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens",
		"COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens",
		"COALESCE(SUM(total_tokens), 0) AS total_tokens",
	}, ", ")
}

func usageAggregateBucketParamExpression() string {
	if common.UsingPostgreSQL {
		return "CAST(? AS BIGINT)"
	}
	if common.UsingMySQL {
		return "CAST(? AS SIGNED)"
	}
	return "CAST(? AS INTEGER)"
}

func hourlyToDailySelectFields() string {
	return strings.Join([]string{
		usageAggregateBucketParamExpression() + " AS bucket_start",
		"channel_id",
		"COALESCE(MAX(channel_name), '') AS channel_name",
		"COALESCE(MAX(vendor_profile_code), '') AS vendor_profile_code",
		"provider_key_id",
		"COALESCE(MAX(provider_key_preview), '') AS provider_key_preview",
		"token_id",
		"COALESCE(MAX(token_name), '') AS token_name",
		"requested_model",
		"actual_model",
		"COALESCE(SUM(request_count), 0) AS request_count",
		"COALESCE(SUM(quota), 0) AS quota",
		"COALESCE(SUM(cost_quota), 0) AS cost_quota",
		"COALESCE(SUM(input_tokens), 0) AS input_tokens",
		"COALESCE(SUM(output_tokens), 0) AS output_tokens",
		"COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens",
		"COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens",
		"COALESCE(SUM(total_tokens), 0) AS total_tokens",
	}, ", ")
}

func usageAggregateGroupFields() string {
	return "newapi_channel_id, provider_key_id, token_id, requested_model, actual_model"
}

func hourlyUsageAggregateGroupFields() string {
	return "channel_id, provider_key_id, token_id, requested_model, actual_model"
}

func rowsToHourlyAggregates(rows []UsageAggregateRow, now int64) []UsageAggregateHourly {
	aggregates := make([]UsageAggregateHourly, 0, len(rows))
	for _, row := range rows {
		aggregates = append(aggregates, UsageAggregateHourly{
			BucketStart:        row.BucketStart,
			ChannelId:          row.ChannelId,
			ChannelName:        row.ChannelName,
			VendorProfileCode:  row.VendorProfileCode,
			ProviderKeyId:      row.ProviderKeyId,
			ProviderKeyPreview: row.ProviderKeyPreview,
			TokenId:            row.TokenId,
			TokenName:          row.TokenName,
			RequestedModel:     row.RequestedModel,
			ActualModel:        row.ActualModel,
			RequestCount:       row.RequestCount,
			Quota:              row.Quota,
			CostQuota:          row.CostQuota,
			InputTokens:        row.InputTokens,
			OutputTokens:       row.OutputTokens,
			CacheReadTokens:    row.CacheReadTokens,
			CacheWriteTokens:   row.CacheWriteTokens,
			TotalTokens:        row.TotalTokens,
			CreatedAt:          now,
			UpdatedAt:          now,
		})
	}
	return aggregates
}

func rowsToDailyAggregates(rows []UsageAggregateRow, now int64) []UsageAggregateDaily {
	aggregates := make([]UsageAggregateDaily, 0, len(rows))
	for _, row := range rows {
		aggregates = append(aggregates, UsageAggregateDaily{
			BucketStart:        row.BucketStart,
			ChannelId:          row.ChannelId,
			ChannelName:        row.ChannelName,
			VendorProfileCode:  row.VendorProfileCode,
			ProviderKeyId:      row.ProviderKeyId,
			ProviderKeyPreview: row.ProviderKeyPreview,
			TokenId:            row.TokenId,
			TokenName:          row.TokenName,
			RequestedModel:     row.RequestedModel,
			ActualModel:        row.ActualModel,
			RequestCount:       row.RequestCount,
			Quota:              row.Quota,
			CostQuota:          row.CostQuota,
			InputTokens:        row.InputTokens,
			OutputTokens:       row.OutputTokens,
			CacheReadTokens:    row.CacheReadTokens,
			CacheWriteTokens:   row.CacheWriteTokens,
			TotalTokens:        row.TotalTokens,
			CreatedAt:          now,
			UpdatedAt:          now,
		})
	}
	return aggregates
}

func RollupUsageAggregateHourly(bucketStart int64) error {
	bucketStart = UsageAggregateHourBucket(bucketStart)
	if bucketStart <= 0 {
		return errors.New("invalid hourly aggregate bucket")
	}
	bucketEnd := bucketStart + usageAggregateHourSeconds

	var rows []UsageAggregateRow
	err := readDB().Model(&UsageLedger{}).
		Select(aggregateSelectFields(usageAggregateBucketParamExpression()), bucketStart).
		Where("timestamp >= ? AND timestamp < ?", bucketStart, bucketEnd).
		Group(usageAggregateGroupFields()).
		Scan(&rows).Error
	if err != nil {
		return err
	}

	now := common.GetTimestamp()
	aggregates := rowsToHourlyAggregates(rows, now)
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bucket_start = ?", bucketStart).Delete(&UsageAggregateHourly{}).Error; err != nil {
			return err
		}
		if len(aggregates) == 0 {
			return nil
		}
		return tx.CreateInBatches(aggregates, 500).Error
	})
}

func RollupUsageAggregateDaily(dayStart int64) error {
	dayStart = UsageAggregateDayBucket(dayStart)
	if dayStart <= 0 {
		return errors.New("invalid daily aggregate bucket")
	}
	dayEnd := dayStart + usageAggregateDaySeconds

	var rows []UsageAggregateRow
	err := DB.Model(&UsageAggregateHourly{}).
		Select(hourlyToDailySelectFields(), dayStart).
		Where("bucket_start >= ? AND bucket_start < ?", dayStart, dayEnd).
		Group(hourlyUsageAggregateGroupFields()).
		Scan(&rows).Error
	if err != nil {
		return err
	}

	now := common.GetTimestamp()
	aggregates := rowsToDailyAggregates(rows, now)
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bucket_start = ?", dayStart).Delete(&UsageAggregateDaily{}).Error; err != nil {
			return err
		}
		if len(aggregates) == 0 {
			return nil
		}
		return tx.CreateInBatches(aggregates, 500).Error
	})
}

func CleanupUsageAggregateHourly(cutoffBucketStart int64, batchHours int) (int64, error) {
	cutoffBucketStart = UsageAggregateHourBucket(cutoffBucketStart)
	if cutoffBucketStart <= 0 {
		return 0, errors.New("invalid hourly aggregate cleanup cutoff")
	}
	if batchHours <= 0 {
		batchHours = 24
	}

	var buckets []int64
	if err := DB.Model(&UsageAggregateHourly{}).
		Where("bucket_start < ?", cutoffBucketStart).
		Distinct("bucket_start").
		Order("bucket_start asc").
		Limit(batchHours).
		Pluck("bucket_start", &buckets).Error; err != nil {
		return 0, err
	}
	if len(buckets) == 0 {
		return 0, nil
	}

	result := DB.Where("bucket_start IN ?", buckets).Delete(&UsageAggregateHourly{})
	return result.RowsAffected, result.Error
}

func CleanupUsageAggregateDaily(cutoffBucketStart int64, batchDays int) (int64, error) {
	cutoffBucketStart = UsageAggregateDayBucket(cutoffBucketStart)
	if cutoffBucketStart <= 0 {
		return 0, errors.New("invalid daily aggregate cleanup cutoff")
	}
	if batchDays <= 0 {
		batchDays = 30
	}

	var buckets []int64
	if err := DB.Model(&UsageAggregateDaily{}).
		Where("bucket_start < ?", cutoffBucketStart).
		Distinct("bucket_start").
		Order("bucket_start asc").
		Limit(batchDays).
		Pluck("bucket_start", &buckets).Error; err != nil {
		return 0, err
	}
	if len(buckets) == 0 {
		return 0, nil
	}

	result := DB.Where("bucket_start IN ?", buckets).Delete(&UsageAggregateDaily{})
	return result.RowsAffected, result.Error
}

func RecordUsageAggregationJob(jobType string, bucketStart int64, status string, startedAt int64, finishedAt int64, err error) {
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
		if len(errorMessage) > 4096 {
			errorMessage = errorMessage[:4096]
		}
	}
	now := common.GetTimestamp()
	job := &UsageAggregationJob{
		JobType:      jobType,
		BucketStart:  bucketStart,
		Status:       status,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		ErrorMessage: errorMessage,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if createErr := DB.Create(job).Error; createErr != nil {
		common.SysLog(fmt.Sprintf("failed to record usage aggregation job: %v", createErr))
	}
}

func buildUsageAggregateBaseQuery(query UsageAggregateQuery) *gorm.DB {
	tx := DB.Table(usageAggregateTable(query.Granularity))
	if query.StartTimestamp > 0 {
		tx = tx.Where("bucket_start >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp > 0 {
		tx = tx.Where("bucket_start < ?", query.EndTimestamp)
	}
	if query.ChannelId != 0 {
		tx = tx.Where("channel_id = ?", query.ChannelId)
	}
	if query.ProviderKeyId != 0 {
		tx = tx.Where("provider_key_id = ?", query.ProviderKeyId)
	}
	if query.TokenId != 0 {
		tx = tx.Where("token_id = ?", query.TokenId)
	}
	if query.RequestedModel != "" {
		tx = tx.Where("requested_model = ?", query.RequestedModel)
	}
	if query.ActualModel != "" {
		tx = tx.Where("actual_model = ?", query.ActualModel)
	}
	return tx
}

func buildUsageAggregateLedgerQuery(query UsageAggregateQuery) *gorm.DB {
	query.Granularity = normalizeUsageAggregateGranularity(query.Granularity)
	bucketExpr := usageAggregateBucketExpression(query.Granularity, "timestamp")
	tx := DB.Model(&UsageLedger{}).Select(aggregateSelectFields(bucketExpr))
	if query.StartTimestamp > 0 {
		tx = tx.Where("timestamp >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp > 0 {
		tx = tx.Where("timestamp < ?", query.EndTimestamp)
	}
	if query.ChannelId != 0 {
		tx = tx.Where("newapi_channel_id = ?", query.ChannelId)
	}
	if query.ProviderKeyId != 0 {
		tx = tx.Where("provider_key_id = ?", query.ProviderKeyId)
	}
	if query.TokenId != 0 {
		tx = tx.Where("token_id = ?", query.TokenId)
	}
	if query.RequestedModel != "" {
		tx = tx.Where("requested_model = ?", query.RequestedModel)
	}
	if query.ActualModel != "" {
		tx = tx.Where("actual_model = ?", query.ActualModel)
	}
	return tx.Group(bucketExpr + ", " + usageAggregateGroupFields())
}

func usageAggregateSortClause(sortBy string, sortOrder string) string {
	allowed := map[string]string{
		"bucket_start":       "bucket_start",
		"request_count":      "request_count",
		"quota":              "quota",
		"cost_quota":         "cost_quota",
		"input_tokens":       "input_tokens",
		"output_tokens":      "output_tokens",
		"cache_read_tokens":  "cache_read_tokens",
		"cache_write_tokens": "cache_write_tokens",
		"total_tokens":       "total_tokens",
	}
	column := allowed[sortBy]
	if column == "" {
		column = "bucket_start"
	}
	order := "desc"
	if strings.EqualFold(sortOrder, "asc") {
		order = "asc"
	}
	return column + " " + order
}

func usageAggregateSummarySelect() string {
	return strings.Join([]string{
		"COALESCE(SUM(request_count), 0) AS request_count",
		"COALESCE(SUM(quota), 0) AS quota",
		"COALESCE(SUM(cost_quota), 0) AS cost_quota",
		"COALESCE(SUM(input_tokens), 0) AS input_tokens",
		"COALESCE(SUM(output_tokens), 0) AS output_tokens",
		"COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens",
		"COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens",
		"COALESCE(SUM(total_tokens), 0) AS total_tokens",
	}, ", ")
}

func usageAggregateLimit(limit int) int {
	if limit <= 0 {
		limit = common.ItemsPerPage
	}
	if limit > 100 {
		limit = 100
	}
	return limit
}

func QueryUsageAggregates(query UsageAggregateQuery) ([]UsageAggregateRow, int64, UsageAggregateSummary, error) {
	query.Granularity = normalizeUsageAggregateGranularity(query.Granularity)
	if query.Live {
		return queryUsageAggregatesFromLedger(query)
	}
	base := buildUsageAggregateBaseQuery(query)

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, UsageAggregateSummary{}, err
	}

	var summary UsageAggregateSummary
	if err := base.Session(&gorm.Session{}).Select(usageAggregateSummarySelect()).Scan(&summary).Error; err != nil {
		return nil, 0, UsageAggregateSummary{}, err
	}

	limit := usageAggregateLimit(query.Limit)

	var rows []UsageAggregateRow
	err := base.Session(&gorm.Session{}).
		Order(usageAggregateSortClause(query.SortBy, query.SortOrder)).
		Limit(limit).
		Offset(query.StartIdx).
		Find(&rows).Error
	return rows, total, summary, err
}

func queryUsageAggregatesFromLedger(query UsageAggregateQuery) ([]UsageAggregateRow, int64, UsageAggregateSummary, error) {
	base := buildUsageAggregateLedgerQuery(query)
	subquery := func() *gorm.DB {
		return DB.Table("(?) AS usage_agg", base.Session(&gorm.Session{}))
	}

	var total int64
	if err := subquery().Count(&total).Error; err != nil {
		return nil, 0, UsageAggregateSummary{}, err
	}

	var summary UsageAggregateSummary
	if err := subquery().Select(usageAggregateSummarySelect()).Scan(&summary).Error; err != nil {
		return nil, 0, UsageAggregateSummary{}, err
	}

	var rows []UsageAggregateRow
	err := subquery().
		Order(usageAggregateSortClause(query.SortBy, query.SortOrder)).
		Limit(usageAggregateLimit(query.Limit)).
		Offset(query.StartIdx).
		Scan(&rows).Error
	return rows, total, summary, err
}

func ExportUsageAggregates(query UsageAggregateQuery, limit int) ([]UsageAggregateRow, int64, error) {
	query.Granularity = normalizeUsageAggregateGranularity(query.Granularity)
	if query.Live {
		return exportUsageAggregatesFromLedger(query, limit)
	}
	base := buildUsageAggregateBaseQuery(query)

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = common.UsageAggregationExportMaxRows
	}
	if total > int64(limit) {
		return nil, total, fmt.Errorf("export rows exceed limit: %d > %d", total, limit)
	}

	var rows []UsageAggregateRow
	err := base.Session(&gorm.Session{}).
		Order(usageAggregateSortClause(query.SortBy, query.SortOrder)).
		Limit(limit).
		Find(&rows).Error
	return rows, total, err
}

func exportUsageAggregatesFromLedger(query UsageAggregateQuery, limit int) ([]UsageAggregateRow, int64, error) {
	base := buildUsageAggregateLedgerQuery(query)
	subquery := func() *gorm.DB {
		return DB.Table("(?) AS usage_agg", base.Session(&gorm.Session{}))
	}

	var total int64
	if err := subquery().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = common.UsageAggregationExportMaxRows
	}
	if total > int64(limit) {
		return nil, total, fmt.Errorf("export rows exceed limit: %d > %d", total, limit)
	}

	var rows []UsageAggregateRow
	err := subquery().
		Order(usageAggregateSortClause(query.SortBy, query.SortOrder)).
		Limit(limit).
		Scan(&rows).Error
	return rows, total, err
}
