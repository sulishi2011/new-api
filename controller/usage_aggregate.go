package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func parseUsageAggregateQuery(c *gin.Context) model.UsageAggregateQuery {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	timezoneOffsetSeconds, _ := strconv.ParseInt(c.Query("timezone_offset"), 10, 64)
	timezoneOffsetSeconds = model.NormalizeUsageAggregateTimezoneOffset(timezoneOffsetSeconds)
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	if channelId == 0 {
		channelId, _ = strconv.Atoi(c.Query("channel"))
	}
	providerKeyId, _ := strconv.Atoi(c.Query("provider_key_id"))
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	pageInfo := common.GetPageQuery(c)

	return model.UsageAggregateQuery{
		Granularity:           c.DefaultQuery("granularity", model.UsageAggregateGranularityDay),
		GroupBy:               c.DefaultQuery("group_by", model.UsageAggregateGroupByDetail),
		Live:                  c.Query("live") == "true" || c.Query("include_live") == "true",
		StartTimestamp:        startTimestamp,
		EndTimestamp:          endTimestamp,
		TimezoneOffsetSeconds: timezoneOffsetSeconds,
		ChannelId:             channelId,
		ProviderKeyId:         providerKeyId,
		TokenId:               tokenId,
		RequestedModel:        c.Query("requested_model"),
		ActualModel:           c.Query("actual_model"),
		StartIdx:              pageInfo.GetStartIdx(),
		Limit:                 pageInfo.GetPageSize(),
		SortBy:                c.DefaultQuery("sort_by", "bucket_start"),
		SortOrder:             c.DefaultQuery("sort_order", "desc"),
	}
}

func GetUsageAggregates(c *gin.Context) {
	query := parseUsageAggregateQuery(c)
	rows, total, summary, err := model.QueryUsageAggregates(query)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo := common.GetPageQuery(c)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(rows)
	common.ApiSuccess(c, gin.H{
		"page":    pageInfo,
		"summary": summary,
	})
}

func ExportUsageAggregates(c *gin.Context) {
	query := parseUsageAggregateQuery(c)
	limit := common.UsageAggregationExportMaxRows
	rows, total, err := model.ExportUsageAggregates(query, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	filename := fmt.Sprintf("usage-aggregates-%s-%s.csv", query.Granularity, time.Now().UTC().Format("20060102150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("X-Total-Rows", strconv.FormatInt(total, 10))
	c.Status(http.StatusOK)

	writer := csv.NewWriter(c.Writer)
	_ = writer.Write(usageAggregateExportHeaders())
	for _, row := range rows {
		_ = writer.Write([]string{
			formatUsageAggregateBucketRange(row.BucketStart, query.Granularity, query.TimezoneOffsetSeconds),
			strconv.Itoa(row.ChannelId),
			row.ChannelName,
			row.VendorProfileCode,
			model.FormatVendorChannelName(row.VendorProfileCode, row.ChannelName),
			strconv.Itoa(row.ProviderKeyId),
			row.ProviderKeyPreview,
			strconv.Itoa(row.TokenId),
			row.TokenName,
			row.RequestedModel,
			row.ActualModel,
			strconv.Itoa(row.RequestCount),
			formatUsageAggregateQuotaAmount(row.Quota),
			formatUsageAggregateQuotaAmount(row.CostQuota),
			strconv.Itoa(row.InputTokens),
			strconv.Itoa(row.OutputTokens),
			strconv.Itoa(row.CacheReadTokens),
			strconv.Itoa(row.CacheWriteTokens),
			strconv.Itoa(row.TotalTokens),
		})
	}
	writer.Flush()
}

func formatUsageAggregateBucketRange(bucketStart int64, granularity string, timezoneOffsetSeconds int64) string {
	if bucketStart <= 0 {
		return "-"
	}

	start := time.Unix(bucketStart+timezoneOffsetSeconds, 0).UTC()
	var end time.Time
	if granularity == model.UsageAggregateGranularityHour {
		end = start.Add(time.Hour)
	} else {
		end = start.AddDate(0, 0, 1)
	}

	const layout = "2006-01-02 15:04"
	endText := end.Format(layout)
	if start.Year() == end.Year() && start.YearDay() == end.YearDay() {
		endText = end.Format("15:04")
	}

	return fmt.Sprintf("%s - %s %s", start.Format(layout), endText, usageAggregateTimezoneLabel(timezoneOffsetSeconds))
}

func usageAggregateTimezoneLabel(timezoneOffsetSeconds int64) string {
	if timezoneOffsetSeconds == 0 {
		return "UTC"
	}
	sign := "+"
	if timezoneOffsetSeconds < 0 {
		sign = "-"
		timezoneOffsetSeconds = -timezoneOffsetSeconds
	}
	hours := timezoneOffsetSeconds / 3600
	minutes := (timezoneOffsetSeconds % 3600) / 60
	return fmt.Sprintf("UTC%s%02d:%02d", sign, hours, minutes)
}

func usageAggregateExportHeaders() []string {
	amountUnit := usageAggregateQuotaAmountUnit()
	return []string{
		"时间桶",
		"渠道ID",
		"渠道名称",
		"供应商配置",
		"渠道显示名称",
		"供应商密钥ID",
		"供应商密钥预览",
		"令牌ID",
		"令牌名称",
		"请求模型",
		"实际模型",
		"请求数",
		fmt.Sprintf("原价消耗(%s)", amountUnit),
		fmt.Sprintf("成本消耗(%s)", amountUnit),
		"输入",
		"输出",
		"缓存读取",
		"缓存写入",
		"总Token",
	}
}

func usageAggregateQuotaAmountUnit() string {
	switch operation_setting.GetQuotaDisplayType() {
	case operation_setting.QuotaDisplayTypeCNY:
		return "CNY"
	case operation_setting.QuotaDisplayTypeCustom:
		if symbol := operation_setting.GetGeneralSetting().CustomCurrencySymbol; symbol != "" {
			return symbol
		}
		return "自定义货币"
	case operation_setting.QuotaDisplayTypeTokens:
		return "额度"
	default:
		return "USD"
	}
}

func formatUsageAggregateQuotaAmount(quota int) string {
	amount := decimal.NewFromInt(int64(quota))
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens || common.QuotaPerUnit <= 0 {
		return amount.String()
	}

	amount = amount.
		Div(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(decimal.NewFromFloat(operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)))
	return amount.String()
}
