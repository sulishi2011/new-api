package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func parseUsageAggregateQuery(c *gin.Context) model.UsageAggregateQuery {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	if channelId == 0 {
		channelId, _ = strconv.Atoi(c.Query("channel"))
	}
	providerKeyId, _ := strconv.Atoi(c.Query("provider_key_id"))
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	pageInfo := common.GetPageQuery(c)

	return model.UsageAggregateQuery{
		Granularity:    c.DefaultQuery("granularity", model.UsageAggregateGranularityDay),
		Live:           c.Query("live") == "true" || c.Query("include_live") == "true",
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ChannelId:      channelId,
		ProviderKeyId:  providerKeyId,
		TokenId:        tokenId,
		RequestedModel: c.Query("requested_model"),
		ActualModel:    c.Query("actual_model"),
		StartIdx:       pageInfo.GetStartIdx(),
		Limit:          pageInfo.GetPageSize(),
		SortBy:         c.DefaultQuery("sort_by", "bucket_start"),
		SortOrder:      c.DefaultQuery("sort_order", "desc"),
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
	_ = writer.Write([]string{
		"bucket_start",
		"channel_id",
		"channel_name",
		"vendor_profile_code",
		"channel_display_name",
		"provider_key_id",
		"provider_key_preview",
		"token_id",
		"token_name",
		"requested_model",
		"actual_model",
		"request_count",
		"quota",
		"cost_quota",
		"input_tokens",
		"output_tokens",
		"cache_read_tokens",
		"cache_write_tokens",
		"total_tokens",
	})
	for _, row := range rows {
		_ = writer.Write([]string{
			strconv.FormatInt(row.BucketStart, 10),
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
			strconv.Itoa(row.Quota),
			strconv.Itoa(row.CostQuota),
			strconv.Itoa(row.InputTokens),
			strconv.Itoa(row.OutputTokens),
			strconv.Itoa(row.CacheReadTokens),
			strconv.Itoa(row.CacheWriteTokens),
			strconv.Itoa(row.TotalTokens),
		})
	}
	writer.Flush()
}
