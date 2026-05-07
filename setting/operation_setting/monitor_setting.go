package operation_setting

import (
	"os"
	"strconv"

	"github.com/QuantumNous/new-api/setting/config"
)

type MonitorSetting struct {
	AutoTestChannelEnabled           bool    `json:"auto_test_channel_enabled"`
	AutoTestChannelMinutes           float64 `json:"auto_test_channel_minutes"`
	ChannelFailureRateDisableEnabled bool    `json:"channel_failure_rate_disable_enabled"`
	ChannelFailureRateWindowMinutes  int     `json:"channel_failure_rate_window_minutes"`
	ChannelFailureRateThreshold      float64 `json:"channel_failure_rate_threshold"`
	ChannelFailureRateMinRequests    int64   `json:"channel_failure_rate_min_requests"`
	RequestFailureWebhookEnabled     bool    `json:"request_failure_webhook_enabled"`
	RequestFailureWebhookUrl         string  `json:"request_failure_webhook_url"`
	RequestFailureWebhookSecret      string  `json:"request_failure_webhook_secret"`
	ChannelDisabledWebhookEnabled    bool    `json:"channel_disabled_webhook_enabled"`
	ChannelDisabledWebhookUrl        string  `json:"channel_disabled_webhook_url"`
	ChannelDisabledWebhookSecret     string  `json:"channel_disabled_webhook_secret"`
}

const (
	defaultAutoTestChannelMinutes          = 10
	defaultChannelFailureRateWindowMinutes = 5
	defaultChannelFailureRateThreshold     = 50
	defaultChannelFailureRateMinRequests   = 20
)

// 默认配置
var monitorSetting = MonitorSetting{
	AutoTestChannelEnabled:          false,
	AutoTestChannelMinutes:          defaultAutoTestChannelMinutes,
	ChannelFailureRateWindowMinutes: defaultChannelFailureRateWindowMinutes,
	ChannelFailureRateThreshold:     defaultChannelFailureRateThreshold,
	ChannelFailureRateMinRequests:   defaultChannelFailureRateMinRequests,
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("monitor_setting", &monitorSetting)
}

func GetMonitorSetting() *MonitorSetting {
	if os.Getenv("CHANNEL_TEST_FREQUENCY") != "" {
		frequency, err := strconv.Atoi(os.Getenv("CHANNEL_TEST_FREQUENCY"))
		if err == nil && frequency > 0 {
			monitorSetting.AutoTestChannelEnabled = true
			monitorSetting.AutoTestChannelMinutes = float64(frequency)
		}
	}
	if monitorSetting.AutoTestChannelMinutes < 1 {
		monitorSetting.AutoTestChannelMinutes = defaultAutoTestChannelMinutes
	}
	if monitorSetting.ChannelFailureRateWindowMinutes < 1 {
		monitorSetting.ChannelFailureRateWindowMinutes = defaultChannelFailureRateWindowMinutes
	}
	if monitorSetting.ChannelFailureRateThreshold <= 0 || monitorSetting.ChannelFailureRateThreshold > 100 {
		monitorSetting.ChannelFailureRateThreshold = defaultChannelFailureRateThreshold
	}
	if monitorSetting.ChannelFailureRateMinRequests < 1 {
		monitorSetting.ChannelFailureRateMinRequests = defaultChannelFailureRateMinRequests
	}
	return &monitorSetting
}
