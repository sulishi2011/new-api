package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type DashboardDimension string
type DashboardMetric string

const (
	DashboardDimensionModel         DashboardDimension = "model_name"
	DashboardDimensionProviderKey   DashboardDimension = "provider_key_id"
	DashboardDimensionChannel       DashboardDimension = "channel_id"
	DashboardDimensionToken         DashboardDimension = "token_id"
	DashboardDimensionUsername      DashboardDimension = "username"
	DashboardDimensionVendorProfile DashboardDimension = "vendor_profile_id"
	DashboardDimensionGroup         DashboardDimension = "group"
	DashboardDimensionBizLine       DashboardDimension = "biz_line"
	DashboardDimensionBizScene      DashboardDimension = "biz_scene"

	DashboardMetricOriginal DashboardMetric = "original_quota"
	DashboardMetricCost     DashboardMetric = "cost_quota"
)

type DashboardUsageQuery struct {
	UserID          int
	Username        string
	StartTimestamp  int64
	EndTimestamp    int64
	ModelName       string
	ChannelID       int
	ProviderKeyID   int
	TokenID         int
	VendorProfileID int
	Group           string
	BizLine         string
	BizScene        string
	Dimension       DashboardDimension
	Metric          DashboardMetric
	Granularity     string
}

type dashboardUsageAggregate struct {
	CreatedAt   int64  `gorm:"column:created_at"`
	ModelName   string `gorm:"column:model_name"`
	Username    string `gorm:"column:username"`
	GroupName   string `gorm:"column:group_name"`
	BizLine     string `gorm:"column:biz_line"`
	BizScene    string `gorm:"column:biz_scene"`
	DimensionID int    `gorm:"column:dimension_id"`
	Count       int    `gorm:"column:count"`
	Quota       int    `gorm:"column:quota"`
	TokenUsed   int    `gorm:"column:token_used"`
}

type dashboardNamedEntity struct {
	ID   int    `gorm:"column:id"`
	Name string `gorm:"column:name"`
}

func normalizeDashboardDimension(input string) DashboardDimension {
	switch DashboardDimension(input) {
	case DashboardDimensionModel,
		DashboardDimensionProviderKey,
		DashboardDimensionChannel,
		DashboardDimensionToken,
		DashboardDimensionUsername,
		DashboardDimensionVendorProfile,
		DashboardDimensionGroup,
		DashboardDimensionBizLine,
		DashboardDimensionBizScene:
		return DashboardDimension(input)
	default:
		return DashboardDimensionModel
	}
}

func buildDashboardUsageBaseQuery(db *gorm.DB, tableName string, query DashboardUsageQuery, modelNamePattern string) *gorm.DB {
	tx := db.Table(logRawTableExpr(tableName)).Where("type = ?", LogTypeConsume)

	if query.UserID > 0 {
		tx = tx.Where("user_id = ?", query.UserID)
	}
	if query.Username != "" {
		tx = tx.Where("username = ?", query.Username)
	}
	if query.StartTimestamp != 0 {
		tx = tx.Where("created_at >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp != 0 {
		tx = tx.Where("created_at <= ?", query.EndTimestamp)
	}
	if query.ModelName != "" {
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if query.ChannelID != 0 {
		tx = tx.Where("channel_id = ?", query.ChannelID)
	}
	if query.ProviderKeyID != 0 {
		tx = tx.Where("provider_key_id = ?", query.ProviderKeyID)
	}
	if query.TokenID != 0 {
		tx = tx.Where("token_id = ?", query.TokenID)
	}
	if query.VendorProfileID != 0 {
		tx = tx.Where("vendor_profile_id = ?", query.VendorProfileID)
	}
	if query.Group != "" {
		tx = tx.Where(logGroupCol+" = ?", query.Group)
	}
	if query.BizLine != "" {
		tx = tx.Where("biz_line = ?", query.BizLine)
	}
	if query.BizScene != "" {
		tx = tx.Where("biz_scene = ?", query.BizScene)
	}
	return tx
}

func normalizeDashboardMetric(input string) DashboardMetric {
	switch DashboardMetric(input) {
	case DashboardMetricCost:
		return DashboardMetricCost
	default:
		return DashboardMetricOriginal
	}
}

func normalizeDashboardAggregateGranularity(input string) string {
	switch input {
	case UsageAggregateGranularityHour:
		return UsageAggregateGranularityHour
	default:
		return UsageAggregateGranularityDay
	}
}

func isDashboardAggregateDimensionSupported(dimension DashboardDimension) bool {
	switch dimension {
	case DashboardDimensionModel,
		DashboardDimensionProviderKey,
		DashboardDimensionChannel,
		DashboardDimensionToken:
		return true
	default:
		return false
	}
}

func canUseDashboardAggregates(query DashboardUsageQuery) bool {
	if !common.UsageAggregationEnabled {
		return false
	}
	if query.StartTimestamp <= 0 || query.EndTimestamp <= 0 || query.EndTimestamp < query.StartTimestamp {
		return false
	}
	if query.UserID > 0 || query.Username != "" || query.VendorProfileID != 0 || query.Group != "" || query.BizLine != "" || query.BizScene != "" {
		return false
	}
	if normalizeDashboardMetric(string(query.Metric)) != DashboardMetricOriginal {
		return false
	}
	if strings.Contains(query.ModelName, "%") {
		return false
	}
	if !isDashboardAggregateDimensionSupported(normalizeDashboardDimension(string(query.Dimension))) {
		return false
	}
	db := readDB()
	return db.Migrator().HasTable(usageAggregateTable(normalizeDashboardAggregateGranularity(query.Granularity))) &&
		db.Migrator().HasTable(&UsageLedger{})
}

func dashboardAggregateDimensionParts(dimension DashboardDimension, fromLedger bool) (selectPart string, groupPart string) {
	switch dimension {
	case DashboardDimensionProviderKey:
		return "provider_key_id AS dimension_id", "provider_key_id"
	case DashboardDimensionChannel:
		if fromLedger {
			return "newapi_channel_id AS dimension_id", "newapi_channel_id"
		}
		return "channel_id AS dimension_id", "channel_id"
	case DashboardDimensionToken:
		return "token_id AS dimension_id", "token_id"
	default:
		return "requested_model AS model_name", "requested_model"
	}
}

func dashboardAggregateQuotaExpr(metric DashboardMetric, fromLog bool) string {
	if normalizeDashboardMetric(string(metric)) != DashboardMetricCost {
		return "COALESCE(SUM(quota), 0)"
	}
	if fromLog {
		return "COALESCE(SUM(CASE WHEN cost_quota IS NULL THEN quota ELSE cost_quota END), 0)"
	}
	return "COALESCE(SUM(cost_quota), 0)"
}

type dashboardUsageTimeRange struct {
	start int64
	end   int64
}

func dashboardAggregationBucketSeconds(granularity string) int64 {
	if normalizeDashboardAggregateGranularity(granularity) == UsageAggregateGranularityHour {
		return usageAggregateHourSeconds
	}
	return usageAggregateDaySeconds
}

func ceilDashboardBucket(ts int64, bucketSeconds int64) int64 {
	if ts <= 0 {
		return 0
	}
	return ((ts + bucketSeconds - 1) / bucketSeconds) * bucketSeconds
}

func floorDashboardBucket(ts int64, bucketSeconds int64) int64 {
	if ts <= 0 {
		return 0
	}
	return (ts / bucketSeconds) * bucketSeconds
}

func trustedDashboardAggregateEnd(granularity string) int64 {
	now := time.Unix(common.GetTimestamp(), 0).UTC()
	if normalizeDashboardAggregateGranularity(granularity) == UsageAggregateGranularityHour {
		return now.Truncate(time.Hour).Add(-time.Hour).Unix()
	}
	return now.Truncate(24 * time.Hour).Unix()
}

func appendDashboardUsageRange(ranges []dashboardUsageTimeRange, start int64, end int64) []dashboardUsageTimeRange {
	if start < end {
		ranges = append(ranges, dashboardUsageTimeRange{start: start, end: end})
	}
	return ranges
}

func minInt64(left int64, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func maxInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}

func buildDashboardAggregatePlan(query DashboardUsageQuery, granularity string) (int64, int64, []dashboardUsageTimeRange) {
	start := query.StartTimestamp
	endExclusive := query.EndTimestamp + 1
	if start <= 0 || endExclusive <= start {
		return 0, 0, []dashboardUsageTimeRange{{start: start, end: endExclusive}}
	}

	bucketSeconds := dashboardAggregationBucketSeconds(granularity)
	fullBucketStart := ceilDashboardBucket(start, bucketSeconds)
	fullBucketEnd := floorDashboardBucket(endExclusive, bucketSeconds)
	trustedEnd := trustedDashboardAggregateEnd(granularity)
	if fullBucketEnd > trustedEnd {
		fullBucketEnd = trustedEnd
	}
	if fullBucketStart >= fullBucketEnd {
		return 0, 0, []dashboardUsageTimeRange{{start: start, end: endExclusive}}
	}

	ranges := make([]dashboardUsageTimeRange, 0, 2)
	ranges = appendDashboardUsageRange(ranges, start, minInt64(endExclusive, fullBucketStart))
	ranges = appendDashboardUsageRange(ranges, maxInt64(start, fullBucketEnd), endExclusive)
	return fullBucketStart, fullBucketEnd, ranges
}

func applyDashboardAggregateFilters(tx *gorm.DB, query DashboardUsageQuery, fromLedger bool) *gorm.DB {
	if query.ModelName != "" {
		tx = tx.Where("requested_model = ?", query.ModelName)
	}
	if query.ChannelID != 0 {
		if fromLedger {
			tx = tx.Where("newapi_channel_id = ?", query.ChannelID)
		} else {
			tx = tx.Where("channel_id = ?", query.ChannelID)
		}
	}
	if query.ProviderKeyID != 0 {
		tx = tx.Where("provider_key_id = ?", query.ProviderKeyID)
	}
	if query.TokenID != 0 {
		tx = tx.Where("token_id = ?", query.TokenID)
	}
	return tx
}

func queryDashboardAggregateRows(query DashboardUsageQuery, granularity string, start int64, end int64) ([]dashboardUsageAggregate, error) {
	selectPart, groupPart := dashboardAggregateDimensionParts(query.Dimension, false)
	selectFields := []string{
		"bucket_start AS created_at",
		selectPart,
		"COALESCE(SUM(request_count), 0) AS count",
		dashboardAggregateQuotaExpr(query.Metric, false) + " AS quota",
		"COALESCE(SUM(total_tokens), 0) AS token_used",
	}

	tx := DB.Table(usageAggregateTable(granularity)).
		Where("bucket_start >= ? AND bucket_start < ?", start, end)
	tx = applyDashboardAggregateFilters(tx, query, false)

	var rows []dashboardUsageAggregate
	err := tx.Select(strings.Join(selectFields, ", ")).
		Group("bucket_start, " + groupPart).
		Order("created_at ASC").
		Scan(&rows).Error
	return rows, err
}

func queryDashboardLedgerRows(query DashboardUsageQuery, granularity string, ranges []dashboardUsageTimeRange) ([]dashboardUsageAggregate, error) {
	if len(ranges) == 0 {
		return nil, nil
	}

	var rows []dashboardUsageAggregate
	for _, timeRange := range ranges {
		bucketExpr := usageAggregateBucketExpression(granularity, "timestamp")
		selectPart, groupPart := dashboardAggregateDimensionParts(query.Dimension, true)
		selectFields := []string{
			bucketExpr + " AS created_at",
			selectPart,
			"COUNT(*) AS count",
			dashboardAggregateQuotaExpr(query.Metric, false) + " AS quota",
			"COALESCE(SUM(total_tokens), 0) AS token_used",
		}
		tx := DB.Model(&UsageLedger{}).
			Where("timestamp >= ? AND timestamp < ?", timeRange.start, timeRange.end)
		tx = applyDashboardAggregateFilters(tx, query, true)

		var rangeRows []dashboardUsageAggregate
		err := tx.Select(strings.Join(selectFields, ", ")).
			Group(bucketExpr + ", " + groupPart).
			Order("created_at ASC").
			Scan(&rangeRows).Error
		if err != nil {
			return nil, err
		}
		rows = append(rows, rangeRows...)
	}
	return rows, nil
}

func dashboardUsageRowKey(row dashboardUsageAggregate, dimension DashboardDimension) string {
	switch dimension {
	case DashboardDimensionProviderKey, DashboardDimensionChannel, DashboardDimensionToken:
		return fmt.Sprintf("%d|%d", row.CreatedAt, row.DimensionID)
	default:
		return fmt.Sprintf("%d|%s", row.CreatedAt, row.ModelName)
	}
}

func mergeDashboardUsageRows(dimension DashboardDimension, rowGroups ...[]dashboardUsageAggregate) []dashboardUsageAggregate {
	rowMap := make(map[string]*dashboardUsageAggregate)
	for _, rows := range rowGroups {
		for _, row := range rows {
			key := dashboardUsageRowKey(row, dimension)
			if existing, ok := rowMap[key]; ok {
				existing.Count += row.Count
				existing.Quota += row.Quota
				existing.TokenUsed += row.TokenUsed
				continue
			}
			rowCopy := row
			rowMap[key] = &rowCopy
		}
	}

	results := make([]dashboardUsageAggregate, 0, len(rowMap))
	for _, row := range rowMap {
		results = append(results, *row)
	}
	sort.Slice(results, func(i int, j int) bool {
		if results[i].CreatedAt == results[j].CreatedAt {
			return dashboardUsageRowKey(results[i], dimension) < dashboardUsageRowKey(results[j], dimension)
		}
		return results[i].CreatedAt < results[j].CreatedAt
	})
	return results
}

func listDashboardUsageRowsFromAggregates(query DashboardUsageQuery) ([]dashboardUsageAggregate, bool, error) {
	query.Dimension = normalizeDashboardDimension(string(query.Dimension))
	if !canUseDashboardAggregates(query) {
		return nil, false, nil
	}

	granularity := normalizeDashboardAggregateGranularity(query.Granularity)
	aggregateStart, aggregateEnd, ledgerRanges := buildDashboardAggregatePlan(query, granularity)
	var aggregateRows []dashboardUsageAggregate
	var err error
	if aggregateStart < aggregateEnd {
		aggregateRows, err = queryDashboardAggregateRows(query, granularity, aggregateStart, aggregateEnd)
		if err != nil {
			return nil, true, err
		}
	}

	ledgerRows, err := queryDashboardLedgerRows(query, granularity, ledgerRanges)
	if err != nil {
		return nil, true, err
	}
	return mergeDashboardUsageRows(query.Dimension, aggregateRows, ledgerRows), true, nil
}

func getDashboardDimensionParts(dimension DashboardDimension) (selectPart string, groupPart string) {
	switch dimension {
	case DashboardDimensionProviderKey:
		return "provider_key_id AS dimension_id", "provider_key_id"
	case DashboardDimensionChannel:
		return "channel_id AS dimension_id", "channel_id"
	case DashboardDimensionToken:
		return "token_id AS dimension_id", "token_id"
	case DashboardDimensionUsername:
		return "username", "username"
	case DashboardDimensionVendorProfile:
		return "vendor_profile_id AS dimension_id", "vendor_profile_id"
	case DashboardDimensionGroup:
		return logGroupCol + " AS group_name", logGroupCol
	case DashboardDimensionBizLine:
		return "biz_line", "biz_line"
	case DashboardDimensionBizScene:
		return "biz_scene", "biz_scene"
	default:
		return "model_name", "model_name"
	}
}

func listDashboardUsageRows(query DashboardUsageQuery) ([]dashboardUsageAggregate, error) {
	query.Dimension = normalizeDashboardDimension(string(query.Dimension))
	if rows, handled, err := listDashboardUsageRowsFromAggregates(query); handled || err != nil {
		return rows, err
	}

	tableName, err := resolveLogReadTable(query.StartTimestamp, query.EndTimestamp)
	if err != nil {
		return nil, err
	}
	modelNamePattern := ""
	if query.ModelName != "" {
		modelNamePattern, err = sanitizeLikePattern(query.ModelName)
		if err != nil {
			return nil, err
		}
	}

	bucketExpr := "((created_at / 3600) * 3600)"
	selectPart, groupPart := getDashboardDimensionParts(query.Dimension)
	selectFields := []string{
		bucketExpr + " AS created_at",
		selectPart,
		"COUNT(*) AS count",
		dashboardAggregateQuotaExpr(query.Metric, true) + " AS quota",
		"COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS token_used",
	}

	var rows []dashboardUsageAggregate
	buildQuery := func(db *gorm.DB) *gorm.DB {
		return buildDashboardUsageBaseQuery(db, tableName, query, modelNamePattern).
			Select(strings.Join(selectFields, ", ")).
			Group(bucketExpr + ", " + groupPart).
			Order("created_at ASC")
	}
	if err := scanLogReadWithPrimaryFallback("dashboard usage rows", buildQuery, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func loadNamedEntityMap(table string, ids []int) map[int]string {
	if len(ids) == 0 {
		return map[int]string{}
	}
	nameColumn := "name"
	if table == "vendor_profiles" {
		nameColumn = "code"
	}
	var rows []dashboardNamedEntity
	if err := readDB().Table(table).Select("id, "+nameColumn+" AS name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		common.SysLog(fmt.Sprintf("failed to load %s names: %v", table, err))
		return map[int]string{}
	}
	result := make(map[int]string, len(rows))
	for _, row := range rows {
		result[row.ID] = row.Name
	}
	return result
}

func collectDashboardDimensionIDs(rows []dashboardUsageAggregate) []int {
	idSet := make(map[int]struct{})
	for _, row := range rows {
		if row.DimensionID <= 0 {
			continue
		}
		idSet[row.DimensionID] = struct{}{}
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	return ids
}

func resolveDashboardDimensionLabel(row dashboardUsageAggregate, dimension DashboardDimension, nameMap map[int]string) string {
	switch dimension {
	case DashboardDimensionProviderKey:
		return strconv.Itoa(row.DimensionID)
	case DashboardDimensionChannel, DashboardDimensionToken, DashboardDimensionVendorProfile:
		if name := nameMap[row.DimensionID]; name != "" {
			return fmt.Sprintf("%d - %s", row.DimensionID, name)
		}
		return strconv.Itoa(row.DimensionID)
	case DashboardDimensionUsername:
		if row.Username != "" {
			return row.Username
		}
		return "-"
	case DashboardDimensionGroup:
		if row.GroupName != "" {
			return row.GroupName
		}
		return "-"
	case DashboardDimensionBizLine:
		if row.BizLine != "" {
			return row.BizLine
		}
		return "-"
	case DashboardDimensionBizScene:
		if row.BizScene != "" {
			return row.BizScene
		}
		return "-"
	default:
		if row.ModelName != "" {
			return row.ModelName
		}
		return "-"
	}
}

func mapDashboardRowsToQuotaData(rows []dashboardUsageAggregate, dimension DashboardDimension) []*QuotaData {
	nameMap := map[int]string{}
	switch dimension {
	case DashboardDimensionChannel:
		nameMap = loadNamedEntityMap("channels", collectDashboardDimensionIDs(rows))
	case DashboardDimensionToken:
		nameMap = loadNamedEntityMap("tokens", collectDashboardDimensionIDs(rows))
	case DashboardDimensionVendorProfile:
		nameMap = loadNamedEntityMap("vendor_profiles", collectDashboardDimensionIDs(rows))
	}

	results := make([]*QuotaData, 0, len(rows))
	for _, row := range rows {
		label := resolveDashboardDimensionLabel(row, dimension, nameMap)
		item := &QuotaData{
			Username:  row.Username,
			ModelName: label,
			CreatedAt: row.CreatedAt,
			TokenUsed: row.TokenUsed,
			Count:     row.Count,
			Quota:     row.Quota,
		}
		if dimension == DashboardDimensionUsername {
			item.Username = label
		}
		results = append(results, item)
	}
	return results
}

func GetDashboardQuotaData(query DashboardUsageQuery) ([]*QuotaData, error) {
	rows, err := listDashboardUsageRows(query)
	if err != nil {
		return nil, err
	}
	return mapDashboardRowsToQuotaData(rows, normalizeDashboardDimension(string(query.Dimension))), nil
}

func GetDashboardUserQuotaData(query DashboardUsageQuery) ([]*QuotaData, error) {
	query.Dimension = DashboardDimensionUsername
	rows, err := listDashboardUsageRows(query)
	if err != nil {
		return nil, err
	}
	return mapDashboardRowsToQuotaData(rows, DashboardDimensionUsername), nil
}
