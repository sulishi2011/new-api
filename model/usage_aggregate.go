package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	UsageAggregateGranularityHour = "hour"
	UsageAggregateGranularityDay  = "day"
	UsageAggregateGroupByDetail   = "detail"
	UsageAggregateGroupByChannel  = "channel"

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
	Granularity           string
	GroupBy               string
	Live                  bool
	StartTimestamp        int64
	EndTimestamp          int64
	TimezoneOffsetSeconds int64
	ChannelId             int
	ProviderKeyId         int
	TokenId               int
	RequestedModel        string
	ActualModel           string
	StartIdx              int
	Limit                 int
	SortBy                string
	SortOrder             string
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

func normalizeUsageAggregateGroupBy(groupBy string) string {
	switch groupBy {
	case UsageAggregateGroupByChannel:
		return UsageAggregateGroupByChannel
	default:
		return UsageAggregateGroupByDetail
	}
}

func NormalizeUsageAggregateTimezoneOffset(offsetSeconds int64) int64 {
	const maxTimezoneOffsetSeconds = 14 * 3600
	if offsetSeconds < -maxTimezoneOffsetSeconds || offsetSeconds > maxTimezoneOffsetSeconds {
		return 0
	}
	return offsetSeconds
}

func normalizeUsageAggregateTimezoneOffset(offsetSeconds int64) int64 {
	return NormalizeUsageAggregateTimezoneOffset(offsetSeconds)
}

func usageAggregateBucketSeconds(granularity string) int64 {
	if normalizeUsageAggregateGranularity(granularity) == UsageAggregateGranularityHour {
		return usageAggregateHourSeconds
	}
	return usageAggregateDaySeconds
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
	return usageAggregateBucketExpressionWithOffset(granularity, column, 0)
}

func usageAggregateBucketExpressionWithOffset(granularity string, column string, offsetSeconds int64) string {
	bucketSeconds := usageAggregateBucketSeconds(granularity)
	offsetSeconds = normalizeUsageAggregateTimezoneOffset(offsetSeconds)
	shiftedColumn := column
	if offsetSeconds > 0 {
		shiftedColumn = fmt.Sprintf("(%s + %d)", column, offsetSeconds)
	} else if offsetSeconds < 0 {
		shiftedColumn = fmt.Sprintf("(%s - %d)", column, -offsetSeconds)
	}

	unshift := func(expr string) string {
		if offsetSeconds > 0 {
			return fmt.Sprintf("(%s - %d)", expr, offsetSeconds)
		}
		if offsetSeconds < 0 {
			return fmt.Sprintf("(%s + %d)", expr, -offsetSeconds)
		}
		return expr
	}

	if common.UsingMySQL {
		return unshift(fmt.Sprintf("(FLOOR(%s / %d) * %d)", shiftedColumn, bucketSeconds, bucketSeconds))
	}
	return unshift(fmt.Sprintf("((%s / %d) * %d)", shiftedColumn, bucketSeconds, bucketSeconds))
}

func aggregateDimensionSelectFields(groupBy string) []string {
	if normalizeUsageAggregateGroupBy(groupBy) == UsageAggregateGroupByChannel {
		return []string{
			"newapi_channel_id AS channel_id",
			"COALESCE(MAX(channel_name), '') AS channel_name",
			"COALESCE(MAX(vendor_profile_code), '') AS vendor_profile_code",
			"0 AS provider_key_id",
			"'' AS provider_key_preview",
			"0 AS token_id",
			"'' AS token_name",
			"'' AS requested_model",
			"'' AS actual_model",
		}
	}
	return []string{
		"newapi_channel_id AS channel_id",
		"COALESCE(MAX(channel_name), '') AS channel_name",
		"COALESCE(MAX(vendor_profile_code), '') AS vendor_profile_code",
		"provider_key_id",
		"COALESCE(MAX(provider_key_preview), '') AS provider_key_preview",
		"token_id",
		"COALESCE(MAX(token_name), '') AS token_name",
		"requested_model",
		"actual_model",
	}
}

func hourlyAggregateDimensionSelectFields(groupBy string) []string {
	if normalizeUsageAggregateGroupBy(groupBy) == UsageAggregateGroupByChannel {
		return []string{
			"channel_id",
			"COALESCE(MAX(channel_name), '') AS channel_name",
			"COALESCE(MAX(vendor_profile_code), '') AS vendor_profile_code",
			"0 AS provider_key_id",
			"'' AS provider_key_preview",
			"0 AS token_id",
			"'' AS token_name",
			"'' AS requested_model",
			"'' AS actual_model",
		}
	}
	return []string{
		"channel_id",
		"COALESCE(MAX(channel_name), '') AS channel_name",
		"COALESCE(MAX(vendor_profile_code), '') AS vendor_profile_code",
		"provider_key_id",
		"COALESCE(MAX(provider_key_preview), '') AS provider_key_preview",
		"token_id",
		"COALESCE(MAX(token_name), '') AS token_name",
		"requested_model",
		"actual_model",
	}
}

func aggregateSelectFields(bucketExpr string, groupBy string) string {
	fields := []string{bucketExpr + " AS bucket_start"}
	fields = append(fields, aggregateDimensionSelectFields(groupBy)...)
	fields = append(fields,
		"COUNT(*) AS request_count",
		"COALESCE(SUM(quota), 0) AS quota",
		"COALESCE(SUM(cost_quota), 0) AS cost_quota",
		"COALESCE(SUM(input_tokens), 0) AS input_tokens",
		"COALESCE(SUM(output_tokens), 0) AS output_tokens",
		"COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens",
		"COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens",
		"COALESCE(SUM(total_tokens), 0) AS total_tokens",
	)
	return strings.Join(fields, ", ")
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

func hourlyAggregateSelectFields(bucketExpr string, groupBy string) string {
	fields := []string{bucketExpr + " AS bucket_start"}
	fields = append(fields, hourlyAggregateDimensionSelectFields(groupBy)...)
	fields = append(fields,
		"COALESCE(SUM(request_count), 0) AS request_count",
		"COALESCE(SUM(quota), 0) AS quota",
		"COALESCE(SUM(cost_quota), 0) AS cost_quota",
		"COALESCE(SUM(input_tokens), 0) AS input_tokens",
		"COALESCE(SUM(output_tokens), 0) AS output_tokens",
		"COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens",
		"COALESCE(SUM(cache_write_tokens), 0) AS cache_write_tokens",
		"COALESCE(SUM(total_tokens), 0) AS total_tokens",
	)
	return strings.Join(fields, ", ")
}

func usageAggregateGroupFields(groupBy string) string {
	if normalizeUsageAggregateGroupBy(groupBy) == UsageAggregateGroupByChannel {
		return "newapi_channel_id"
	}
	return "newapi_channel_id, provider_key_id, token_id, requested_model, actual_model"
}

func hourlyUsageAggregateGroupFields(groupBy string) string {
	if normalizeUsageAggregateGroupBy(groupBy) == UsageAggregateGroupByChannel {
		return "channel_id"
	}
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
		Select(aggregateSelectFields(usageAggregateBucketParamExpression(), UsageAggregateGroupByDetail), bucketStart).
		Where("timestamp >= ? AND timestamp < ?", bucketStart, bucketEnd).
		Group(usageAggregateGroupFields(UsageAggregateGroupByDetail)).
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
		Group(hourlyUsageAggregateGroupFields(UsageAggregateGroupByDetail)).
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

func buildUsageAggregateMaterializedGroupedQuery(query UsageAggregateQuery) *gorm.DB {
	query.Granularity = normalizeUsageAggregateGranularity(query.Granularity)
	query.GroupBy = normalizeUsageAggregateGroupBy(query.GroupBy)
	bucketExpr := "bucket_start"
	tx := buildUsageAggregateBaseQuery(query).
		Select(hourlyAggregateSelectFields(bucketExpr, query.GroupBy)).
		Group(bucketExpr + ", " + hourlyUsageAggregateGroupFields(query.GroupBy))
	return tx
}

func buildUsageAggregateLedgerQuery(query UsageAggregateQuery) *gorm.DB {
	query.Granularity = normalizeUsageAggregateGranularity(query.Granularity)
	query.GroupBy = normalizeUsageAggregateGroupBy(query.GroupBy)
	query.TimezoneOffsetSeconds = normalizeUsageAggregateTimezoneOffset(query.TimezoneOffsetSeconds)
	bucketExpr := usageAggregateBucketExpressionWithOffset(query.Granularity, "timestamp", query.TimezoneOffsetSeconds)
	tx := DB.Model(&UsageLedger{}).Select(aggregateSelectFields(bucketExpr, query.GroupBy))
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
	return tx.Group(bucketExpr + ", " + usageAggregateGroupFields(query.GroupBy))
}

func buildUsageAggregateHourlyQuery(query UsageAggregateQuery, startTimestamp int64, endTimestamp int64) *gorm.DB {
	query.Granularity = normalizeUsageAggregateGranularity(query.Granularity)
	query.GroupBy = normalizeUsageAggregateGroupBy(query.GroupBy)
	query.TimezoneOffsetSeconds = normalizeUsageAggregateTimezoneOffset(query.TimezoneOffsetSeconds)
	bucketExpr := usageAggregateBucketExpressionWithOffset(query.Granularity, "bucket_start", query.TimezoneOffsetSeconds)
	tx := DB.Model(&UsageAggregateHourly{}).Select(hourlyAggregateSelectFields(bucketExpr, query.GroupBy))
	if startTimestamp > 0 {
		tx = tx.Where("bucket_start >= ?", startTimestamp)
	}
	if endTimestamp > 0 {
		tx = tx.Where("bucket_start < ?", endTimestamp)
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
	return tx.Group(bucketExpr + ", " + hourlyUsageAggregateGroupFields(query.GroupBy))
}

func usageAggregateBucketStart(ts int64, granularity string, offsetSeconds int64) int64 {
	if ts <= 0 {
		return 0
	}
	bucketSeconds := usageAggregateBucketSeconds(granularity)
	offsetSeconds = normalizeUsageAggregateTimezoneOffset(offsetSeconds)
	return ((ts + offsetSeconds) / bucketSeconds * bucketSeconds) - offsetSeconds
}

func usageAggregateHourCeil(ts int64) int64 {
	if ts <= 0 {
		return 0
	}
	if ts%usageAggregateHourSeconds == 0 {
		return ts
	}
	return (ts/usageAggregateHourSeconds + 1) * usageAggregateHourSeconds
}

func usageAggregateHourFloor(ts int64) int64 {
	if ts <= 0 {
		return 0
	}
	return ts / usageAggregateHourSeconds * usageAggregateHourSeconds
}

func usageAggregateRangeAlignedToMaterializedBuckets(query UsageAggregateQuery) bool {
	if query.TimezoneOffsetSeconds != 0 {
		return false
	}
	if query.StartTimestamp > 0 && usageAggregateBucketStart(query.StartTimestamp, query.Granularity, 0) != query.StartTimestamp {
		return false
	}
	if query.EndTimestamp > 0 && usageAggregateBucketStart(query.EndTimestamp, query.Granularity, 0) != query.EndTimestamp {
		return false
	}
	return true
}

func usageAggregateCanUseHourlyRollup(query UsageAggregateQuery) bool {
	return query.TimezoneOffsetSeconds%usageAggregateHourSeconds == 0
}

func usageAggregateCanUseMaterializedHourly(query UsageAggregateQuery) bool {
	if normalizeUsageAggregateGranularity(query.Granularity) != UsageAggregateGranularityHour {
		return false
	}
	if !usageAggregateCanUseHourlyRollup(query) {
		return false
	}
	if query.StartTimestamp > 0 && usageAggregateBucketStart(query.StartTimestamp, UsageAggregateGranularityHour, query.TimezoneOffsetSeconds) != query.StartTimestamp {
		return false
	}
	if query.EndTimestamp > 0 && usageAggregateBucketStart(query.EndTimestamp, UsageAggregateGranularityHour, query.TimezoneOffsetSeconds) != query.EndTimestamp {
		return false
	}
	return true
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

type usageAggregateMergeKey struct {
	BucketStart    int64
	ChannelId      int
	ProviderKeyId  int
	TokenId        int
	RequestedModel string
	ActualModel    string
}

func usageAggregateRowMergeKey(row UsageAggregateRow) usageAggregateMergeKey {
	return usageAggregateMergeKey{
		BucketStart:    row.BucketStart,
		ChannelId:      row.ChannelId,
		ProviderKeyId:  row.ProviderKeyId,
		TokenId:        row.TokenId,
		RequestedModel: row.RequestedModel,
		ActualModel:    row.ActualModel,
	}
}

func mergeUsageAggregateRows(rowSets ...[]UsageAggregateRow) []UsageAggregateRow {
	merged := make(map[usageAggregateMergeKey]*UsageAggregateRow)
	for _, rows := range rowSets {
		for _, row := range rows {
			key := usageAggregateRowMergeKey(row)
			existing := merged[key]
			if existing == nil {
				rowCopy := row
				merged[key] = &rowCopy
				continue
			}
			if existing.ChannelName == "" {
				existing.ChannelName = row.ChannelName
			}
			if existing.VendorProfileCode == "" {
				existing.VendorProfileCode = row.VendorProfileCode
			}
			if existing.ProviderKeyPreview == "" {
				existing.ProviderKeyPreview = row.ProviderKeyPreview
			}
			if existing.TokenName == "" {
				existing.TokenName = row.TokenName
			}
			existing.RequestCount += row.RequestCount
			existing.Quota += row.Quota
			existing.CostQuota += row.CostQuota
			existing.InputTokens += row.InputTokens
			existing.OutputTokens += row.OutputTokens
			existing.CacheReadTokens += row.CacheReadTokens
			existing.CacheWriteTokens += row.CacheWriteTokens
			existing.TotalTokens += row.TotalTokens
		}
	}

	rows := make([]UsageAggregateRow, 0, len(merged))
	for _, row := range merged {
		rows = append(rows, *row)
	}
	return rows
}

func usageAggregateRowSortValue(row UsageAggregateRow, sortBy string) int64 {
	switch sortBy {
	case "request_count":
		return int64(row.RequestCount)
	case "quota":
		return int64(row.Quota)
	case "cost_quota":
		return int64(row.CostQuota)
	case "input_tokens":
		return int64(row.InputTokens)
	case "output_tokens":
		return int64(row.OutputTokens)
	case "cache_read_tokens":
		return int64(row.CacheReadTokens)
	case "cache_write_tokens":
		return int64(row.CacheWriteTokens)
	case "total_tokens":
		return int64(row.TotalTokens)
	default:
		return row.BucketStart
	}
}

func sortUsageAggregateRows(rows []UsageAggregateRow, sortBy string, sortOrder string) {
	desc := !strings.EqualFold(sortOrder, "asc")
	sort.SliceStable(rows, func(i, j int) bool {
		left := usageAggregateRowSortValue(rows[i], sortBy)
		right := usageAggregateRowSortValue(rows[j], sortBy)
		if left != right {
			if desc {
				return left > right
			}
			return left < right
		}
		if rows[i].BucketStart != rows[j].BucketStart {
			if desc {
				return rows[i].BucketStart > rows[j].BucketStart
			}
			return rows[i].BucketStart < rows[j].BucketStart
		}
		if rows[i].ChannelId != rows[j].ChannelId {
			return rows[i].ChannelId < rows[j].ChannelId
		}
		if rows[i].ProviderKeyId != rows[j].ProviderKeyId {
			return rows[i].ProviderKeyId < rows[j].ProviderKeyId
		}
		if rows[i].TokenId != rows[j].TokenId {
			return rows[i].TokenId < rows[j].TokenId
		}
		if rows[i].RequestedModel != rows[j].RequestedModel {
			return rows[i].RequestedModel < rows[j].RequestedModel
		}
		return rows[i].ActualModel < rows[j].ActualModel
	})
}

func summarizeUsageAggregateRows(rows []UsageAggregateRow) UsageAggregateSummary {
	var summary UsageAggregateSummary
	for _, row := range rows {
		summary.RequestCount += row.RequestCount
		summary.Quota += row.Quota
		summary.CostQuota += row.CostQuota
		summary.InputTokens += row.InputTokens
		summary.OutputTokens += row.OutputTokens
		summary.CacheReadTokens += row.CacheReadTokens
		summary.CacheWriteTokens += row.CacheWriteTokens
		summary.TotalTokens += row.TotalTokens
	}
	return summary
}

func paginateUsageAggregateRows(rows []UsageAggregateRow, startIdx int, limit int) []UsageAggregateRow {
	limit = usageAggregateLimit(limit)
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(rows) {
		return []UsageAggregateRow{}
	}
	end := startIdx + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[startIdx:end]
}

type usageAggregateLedgerInterval struct {
	Start int64
	End   int64
}

func appendUsageAggregateInterval(intervals []usageAggregateLedgerInterval, start int64, end int64) []usageAggregateLedgerInterval {
	if end <= start {
		return intervals
	}
	if len(intervals) > 0 && intervals[len(intervals)-1].End == start {
		intervals[len(intervals)-1].End = end
		return intervals
	}
	return append(intervals, usageAggregateLedgerInterval{Start: start, End: end})
}

func usageAggregateIntervalsFromHourlyBuckets(buckets []int64, start int64, end int64) []usageAggregateLedgerInterval {
	if len(buckets) == 0 {
		return nil
	}
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i] < buckets[j]
	})

	intervals := make([]usageAggregateLedgerInterval, 0, len(buckets))
	var currentStart int64
	var currentEnd int64
	var previousBucket int64
	hasCurrent := false
	for _, bucket := range buckets {
		bucket = UsageAggregateHourBucket(bucket)
		if bucket < start || bucket >= end {
			continue
		}
		if hasCurrent && bucket == previousBucket {
			continue
		}
		previousBucket = bucket
		bucketEnd := bucket + usageAggregateHourSeconds
		if !hasCurrent {
			currentStart = bucket
			currentEnd = bucketEnd
			hasCurrent = true
			continue
		}
		if bucket == currentEnd {
			currentEnd = bucketEnd
			continue
		}
		intervals = append(intervals, usageAggregateLedgerInterval{Start: currentStart, End: currentEnd})
		currentStart = bucket
		currentEnd = bucketEnd
	}
	if hasCurrent {
		intervals = append(intervals, usageAggregateLedgerInterval{Start: currentStart, End: currentEnd})
	}
	return intervals
}

func usageAggregateHourlyCoveredIntervals(start int64, end int64) ([]usageAggregateLedgerInterval, error) {
	if end <= start {
		return nil, nil
	}

	var successfulJobBuckets []int64
	if err := DB.Model(&UsageAggregationJob{}).
		Distinct("bucket_start").
		Where("job_type = ? AND status = ? AND bucket_start >= ? AND bucket_start < ?", "hourly", "success", start, end).
		Order("bucket_start asc").
		Pluck("bucket_start", &successfulJobBuckets).Error; err != nil {
		return nil, err
	}
	if len(successfulJobBuckets) > 0 {
		return usageAggregateIntervalsFromHourlyBuckets(successfulJobBuckets, start, end), nil
	}

	var hourlyJobCount int64
	if err := DB.Model(&UsageAggregationJob{}).
		Where("job_type = ? AND bucket_start >= ? AND bucket_start < ?", "hourly", start, end).
		Count(&hourlyJobCount).Error; err != nil {
		return nil, err
	}
	if hourlyJobCount > 0 {
		return nil, nil
	}

	var aggregateBuckets []int64
	if err := DB.Model(&UsageAggregateHourly{}).
		Distinct("bucket_start").
		Where("bucket_start >= ? AND bucket_start < ?", start, end).
		Order("bucket_start asc").
		Pluck("bucket_start", &aggregateBuckets).Error; err != nil {
		return nil, err
	}
	return usageAggregateIntervalsFromHourlyBuckets(aggregateBuckets, start, end), nil
}

func usageAggregateSourceRanges(query UsageAggregateQuery) ([]usageAggregateLedgerInterval, []usageAggregateLedgerInterval, error) {
	effectiveStart := query.StartTimestamp
	if effectiveStart < 0 {
		effectiveStart = 0
	}
	effectiveEnd := query.EndTimestamp
	if effectiveEnd <= 0 {
		effectiveEnd = common.GetTimestamp()
	}
	if effectiveEnd <= effectiveStart {
		return nil, nil, nil
	}

	hourlyStart := usageAggregateHourCeil(effectiveStart)
	hourlyEnd := usageAggregateHourFloor(effectiveEnd)
	if hourlyEnd <= hourlyStart {
		return nil, []usageAggregateLedgerInterval{{Start: effectiveStart, End: effectiveEnd}}, nil
	}

	hourlyIntervals, err := usageAggregateHourlyCoveredIntervals(hourlyStart, hourlyEnd)
	if err != nil {
		return nil, nil, err
	}
	if len(hourlyIntervals) == 0 {
		return nil, []usageAggregateLedgerInterval{{Start: effectiveStart, End: effectiveEnd}}, nil
	}

	intervals := make([]usageAggregateLedgerInterval, 0, 2)
	if effectiveStart < hourlyStart {
		intervals = appendUsageAggregateInterval(intervals, effectiveStart, hourlyStart)
	}
	cursor := hourlyStart
	for _, interval := range hourlyIntervals {
		intervals = appendUsageAggregateInterval(intervals, cursor, interval.Start)
		if interval.End > cursor {
			cursor = interval.End
		}
	}
	intervals = appendUsageAggregateInterval(intervals, cursor, hourlyEnd)
	intervals = appendUsageAggregateInterval(intervals, hourlyEnd, effectiveEnd)
	return hourlyIntervals, intervals, nil
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

func scanUsageAggregateRows(tx *gorm.DB) ([]UsageAggregateRow, error) {
	var rows []UsageAggregateRow
	if err := tx.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func collectUsageAggregateRowsFromHourlyAndLedger(query UsageAggregateQuery) ([]UsageAggregateRow, error) {
	hourlyIntervals, ledgerIntervals, err := usageAggregateSourceRanges(query)
	if err != nil {
		return nil, err
	}
	rowSets := make([][]UsageAggregateRow, 0, len(hourlyIntervals)+len(ledgerIntervals))

	for _, interval := range hourlyIntervals {
		hourlyRows, err := scanUsageAggregateRows(buildUsageAggregateHourlyQuery(query, interval.Start, interval.End))
		if err != nil {
			return nil, err
		}
		rowSets = append(rowSets, hourlyRows)
	}

	for _, interval := range ledgerIntervals {
		if interval.End <= interval.Start {
			continue
		}
		ledgerQuery := query
		ledgerQuery.StartTimestamp = interval.Start
		ledgerQuery.EndTimestamp = interval.End
		ledgerRows, err := scanUsageAggregateRows(buildUsageAggregateLedgerQuery(ledgerQuery))
		if err != nil {
			return nil, err
		}
		rowSets = append(rowSets, ledgerRows)
	}

	return mergeUsageAggregateRows(rowSets...), nil
}

func queryUsageAggregatesFromHourlyAndLedger(query UsageAggregateQuery) ([]UsageAggregateRow, int64, UsageAggregateSummary, error) {
	rows, err := collectUsageAggregateRowsFromHourlyAndLedger(query)
	if err != nil {
		return nil, 0, UsageAggregateSummary{}, err
	}
	summary := summarizeUsageAggregateRows(rows)
	sortUsageAggregateRows(rows, query.SortBy, query.SortOrder)
	total := int64(len(rows))
	return paginateUsageAggregateRows(rows, query.StartIdx, query.Limit), total, summary, nil
}

func QueryUsageAggregates(query UsageAggregateQuery) ([]UsageAggregateRow, int64, UsageAggregateSummary, error) {
	query.Granularity = normalizeUsageAggregateGranularity(query.Granularity)
	query.GroupBy = normalizeUsageAggregateGroupBy(query.GroupBy)
	query.TimezoneOffsetSeconds = normalizeUsageAggregateTimezoneOffset(query.TimezoneOffsetSeconds)
	if query.Live && usageAggregateCanUseHourlyRollup(query) {
		return queryUsageAggregatesFromHourlyAndLedger(query)
	}
	if !query.Live && usageAggregateCanUseMaterializedHourly(query) {
		query.TimezoneOffsetSeconds = 0
	}
	if !usageAggregateRangeAlignedToMaterializedBuckets(query) {
		if usageAggregateCanUseHourlyRollup(query) {
			return queryUsageAggregatesFromHourlyAndLedger(query)
		}
		return queryUsageAggregatesFromLedger(query)
	}
	if query.GroupBy != UsageAggregateGroupByDetail {
		return queryUsageAggregatesFromMaterializedGrouped(query)
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

func queryUsageAggregatesFromGroupedQuery(base *gorm.DB, query UsageAggregateQuery) ([]UsageAggregateRow, int64, UsageAggregateSummary, error) {
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

func queryUsageAggregatesFromMaterializedGrouped(query UsageAggregateQuery) ([]UsageAggregateRow, int64, UsageAggregateSummary, error) {
	return queryUsageAggregatesFromGroupedQuery(buildUsageAggregateMaterializedGroupedQuery(query), query)
}

func queryUsageAggregatesFromLedger(query UsageAggregateQuery) ([]UsageAggregateRow, int64, UsageAggregateSummary, error) {
	return queryUsageAggregatesFromGroupedQuery(buildUsageAggregateLedgerQuery(query), query)
}

func ExportUsageAggregates(query UsageAggregateQuery, limit int) ([]UsageAggregateRow, int64, error) {
	query.Granularity = normalizeUsageAggregateGranularity(query.Granularity)
	query.GroupBy = normalizeUsageAggregateGroupBy(query.GroupBy)
	query.TimezoneOffsetSeconds = normalizeUsageAggregateTimezoneOffset(query.TimezoneOffsetSeconds)
	if query.Live && usageAggregateCanUseHourlyRollup(query) {
		return exportUsageAggregatesFromHourlyAndLedger(query, limit)
	}
	if !query.Live && usageAggregateCanUseMaterializedHourly(query) {
		query.TimezoneOffsetSeconds = 0
	}
	if !usageAggregateRangeAlignedToMaterializedBuckets(query) {
		if usageAggregateCanUseHourlyRollup(query) {
			return exportUsageAggregatesFromHourlyAndLedger(query, limit)
		}
		return exportUsageAggregatesFromLedger(query, limit)
	}
	if query.GroupBy != UsageAggregateGroupByDetail {
		return exportUsageAggregatesFromMaterializedGrouped(query, limit)
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

func exportUsageAggregatesFromHourlyAndLedger(query UsageAggregateQuery, limit int) ([]UsageAggregateRow, int64, error) {
	rows, err := collectUsageAggregateRowsFromHourlyAndLedger(query)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(rows))
	if limit <= 0 {
		limit = common.UsageAggregationExportMaxRows
	}
	if total > int64(limit) {
		return nil, total, fmt.Errorf("export rows exceed limit: %d > %d", total, limit)
	}

	sortUsageAggregateRows(rows, query.SortBy, query.SortOrder)
	return rows, total, nil
}

func exportUsageAggregatesFromGroupedQuery(base *gorm.DB, query UsageAggregateQuery, limit int) ([]UsageAggregateRow, int64, error) {
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

func exportUsageAggregatesFromMaterializedGrouped(query UsageAggregateQuery, limit int) ([]UsageAggregateRow, int64, error) {
	return exportUsageAggregatesFromGroupedQuery(buildUsageAggregateMaterializedGroupedQuery(query), query, limit)
}

func exportUsageAggregatesFromLedger(query UsageAggregateQuery, limit int) ([]UsageAggregateRow, int64, error) {
	return exportUsageAggregatesFromGroupedQuery(buildUsageAggregateLedgerQuery(query), query, limit)
}
