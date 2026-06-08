package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestFormatUsageAggregateQuotaAmount(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalUSDExchangeRate := operation_setting.USDExchangeRate

	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
		operation_setting.USDExchangeRate = originalUSDExchangeRate
	})

	common.QuotaPerUnit = 500000

	t.Run("usd display divides by quota per unit", func(t *testing.T) {
		operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
		require.Equal(t, "0.5", formatUsageAggregateQuotaAmount(250000))
	})

	t.Run("cny display applies exchange rate", func(t *testing.T) {
		operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
		operation_setting.USDExchangeRate = 7.2
		require.Equal(t, "3.6", formatUsageAggregateQuotaAmount(250000))
	})

	t.Run("tokens display keeps raw quota", func(t *testing.T) {
		operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
		require.Equal(t, "250000", formatUsageAggregateQuotaAmount(250000))
	})
}

func TestUsageAggregateExportHeadersAreChinese(t *testing.T) {
	originalQuotaDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalQuotaDisplayType
	})

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD

	headers := usageAggregateExportHeaders()

	require.Contains(t, headers, "时间桶")
	require.Contains(t, headers, "原价消耗(USD)")
	require.Contains(t, headers, "成本消耗(USD)")
	require.Contains(t, headers, "输入")
	require.Contains(t, headers, "输出")
	require.NotContains(t, headers, "quota")
	require.NotContains(t, headers, "cost_quota")
}

func TestFormatUsageAggregateBucketRange(t *testing.T) {
	require.Equal(t, "-", formatUsageAggregateBucketRange(0, model.UsageAggregateGranularityDay, 0))
	require.Equal(
		t,
		"2024-01-02 03:00 - 04:00 UTC",
		formatUsageAggregateBucketRange(1704164400, model.UsageAggregateGranularityHour, 0),
	)
	require.Equal(
		t,
		"2024-01-02 00:00 - 2024-01-03 00:00 UTC",
		formatUsageAggregateBucketRange(1704153600, model.UsageAggregateGranularityDay, 0),
	)
	require.Equal(
		t,
		"2024-01-02 00:00 - 2024-01-03 00:00 UTC+08:00",
		formatUsageAggregateBucketRange(1704124800, model.UsageAggregateGranularityDay, 8*3600),
	)
}
