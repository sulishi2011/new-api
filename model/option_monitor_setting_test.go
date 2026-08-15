package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionMapUpdatesMonitorSettingRuntimeConfig(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	monitorSetting := operation_setting.GetMonitorSetting()
	originalMonitorSetting := *monitorSetting
	t.Cleanup(func() {
		*operation_setting.GetMonitorSetting() = originalMonitorSetting
	})

	updates := map[string]string{
		"monitor_setting.auto_test_channel_recovery_enabled":   "true",
		"monitor_setting.channel_failure_rate_disable_enabled": "true",
		"monitor_setting.channel_failure_rate_window_minutes":  "7",
		"monitor_setting.channel_failure_rate_threshold":       "42.5",
		"monitor_setting.channel_failure_rate_min_requests":    "11",
		"monitor_setting.request_failure_webhook_enabled":      "true",
		"monitor_setting.request_failure_webhook_url":          "https://example.com/request",
		"monitor_setting.request_failure_webhook_secret":       "request-secret",
		"monitor_setting.channel_disabled_webhook_enabled":     "true",
		"monitor_setting.channel_disabled_webhook_url":         "https://example.com/channel",
		"monitor_setting.channel_disabled_webhook_secret":      "channel-secret",
	}

	for key, value := range updates {
		require.NoError(t, updateOptionMap(key, value))
	}

	got := operation_setting.GetMonitorSetting()
	require.True(t, got.AutoTestChannelRecoveryEnabled)
	require.True(t, got.ChannelFailureRateDisableEnabled)
	require.Equal(t, 7, got.ChannelFailureRateWindowMinutes)
	require.Equal(t, 42.5, got.ChannelFailureRateThreshold)
	require.Equal(t, int64(11), got.ChannelFailureRateMinRequests)
	require.True(t, got.RequestFailureWebhookEnabled)
	require.Equal(t, "https://example.com/request", got.RequestFailureWebhookUrl)
	require.Equal(t, "request-secret", got.RequestFailureWebhookSecret)
	require.True(t, got.ChannelDisabledWebhookEnabled)
	require.Equal(t, "https://example.com/channel", got.ChannelDisabledWebhookUrl)
	require.Equal(t, "channel-secret", got.ChannelDisabledWebhookSecret)

	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	for key, value := range updates {
		require.Equal(t, value, common.OptionMap[key])
	}
}
