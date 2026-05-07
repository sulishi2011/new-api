package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

const (
	channelHealthBucketSeconds    = int64(60)
	channelHealthRedisKeyPrefix   = "channel_health:v1"
	channelHealthDisableCooldown  = 5 * time.Minute
	channelHealthMaxWindowMinutes = 1440
)

type channelHealthBucket struct {
	Total  int64
	Failed int64
}

type channelHealthSnapshot struct {
	Total         int64
	Failed        int64
	FailureRate   float64
	WindowMinutes int
}

var (
	channelHealthMemoryMu      sync.Mutex
	channelHealthMemoryBuckets = make(map[int]map[int64]channelHealthBucket)

	channelHealthDisableMu        sync.Mutex
	channelHealthDisableCooldowns = make(map[int]time.Time)
)

func RecordChannelSuccess(c *gin.Context, channelError types.ChannelError) {
	setting := operation_setting.GetMonitorSetting()
	if !setting.ChannelFailureRateDisableEnabled || channelError.ChannelId <= 0 {
		return
	}
	_, _ = recordChannelHealthEvent(channelError.ChannelId, false, normalizeChannelHealthWindowMinutes(setting.ChannelFailureRateWindowMinutes))
}

func RecordChannelFailure(c *gin.Context, relayInfo *relaycommon.RelayInfo, channelError types.ChannelError, err *types.NewAPIError) {
	if err == nil {
		return
	}

	setting := operation_setting.GetMonitorSetting()
	if setting.RequestFailureWebhookEnabled {
		sendRequestFailureWebhookAsync(c, relayInfo, channelError, err)
	}

	if !setting.ChannelFailureRateDisableEnabled || channelError.ChannelId <= 0 {
		return
	}

	windowMinutes := normalizeChannelHealthWindowMinutes(setting.ChannelFailureRateWindowMinutes)
	snapshot, recordErr := recordChannelHealthEvent(channelError.ChannelId, true, windowMinutes)
	if recordErr != nil {
		common.SysLog(fmt.Sprintf("failed to record channel health: channel_id=%d, error=%v", channelError.ChannelId, recordErr))
		return
	}
	if !shouldDisableByChannelHealth(setting, snapshot) {
		return
	}
	if !channelError.AutoBan {
		return
	}
	if !reserveChannelHealthDisable(channelError.ChannelId) {
		return
	}

	reason := fmt.Sprintf(
		"%d分钟内失败率 %.2f%% (%d/%d)，超过阈值 %.2f%%",
		snapshot.WindowMinutes,
		snapshot.FailureRate,
		snapshot.Failed,
		snapshot.Total,
		setting.ChannelFailureRateThreshold,
	)
	gopool.Go(func() {
		DisableChannel(channelError, reason)
	})
}

func NotifyChannelDisabledWebhook(channelError types.ChannelError, reason string) {
	setting := operation_setting.GetMonitorSetting()
	if !setting.ChannelDisabledWebhookEnabled {
		return
	}
	webhookURL := strings.TrimSpace(setting.ChannelDisabledWebhookUrl)
	if webhookURL == "" {
		common.SysLog("channel disabled webhook enabled but url is empty")
		return
	}

	title := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
	content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
	notify := dto.NewNotify(dto.NotifyTypeChannelDisabled, title, content, nil)
	sendMonitorWebhookAsync(webhookURL, setting.ChannelDisabledWebhookSecret, notify, "channel disabled")
}

func sendRequestFailureWebhookAsync(c *gin.Context, relayInfo *relaycommon.RelayInfo, channelError types.ChannelError, err *types.NewAPIError) {
	setting := operation_setting.GetMonitorSetting()
	webhookURL := strings.TrimSpace(setting.RequestFailureWebhookUrl)
	if webhookURL == "" {
		common.SysLog("request failure webhook enabled but url is empty")
		return
	}

	modelName := ""
	retryIndex := 0
	if relayInfo != nil {
		modelName = relayInfo.OriginModelName
		retryIndex = relayInfo.RetryIndex
	}
	if modelName == "" && c != nil {
		modelName = c.GetString("original_model")
	}

	requestPath := ""
	if c != nil && c.Request != nil && c.Request.URL != nil {
		requestPath = c.Request.URL.Path
	}

	errText := err.MaskSensitiveErrorWithStatusCode()
	title := fmt.Sprintf("通道「%s」（#%d）请求失败", channelError.ChannelName, channelError.ChannelId)
	content := fmt.Sprintf(
		"通道「%s」（#%d）请求失败，状态码：%d，模型：%s，路径：%s，重试序号：%d，错误：%s",
		channelError.ChannelName,
		channelError.ChannelId,
		err.StatusCode,
		modelName,
		requestPath,
		retryIndex,
		errText,
	)
	notify := dto.NewNotify(dto.NotifyTypeChannelRequestFailure, title, content, nil)
	sendMonitorWebhookAsync(webhookURL, setting.RequestFailureWebhookSecret, notify, "request failure")
}

func sendMonitorWebhookAsync(webhookURL string, secret string, notify dto.Notify, label string) {
	gopool.Go(func() {
		if err := SendWebhookNotify(webhookURL, secret, notify); err != nil {
			common.SysLog(fmt.Sprintf("failed to send %s webhook: %s", label, err.Error()))
		}
	})
}

func normalizeChannelHealthWindowMinutes(windowMinutes int) int {
	if windowMinutes < 1 {
		return 5
	}
	if windowMinutes > channelHealthMaxWindowMinutes {
		return channelHealthMaxWindowMinutes
	}
	return windowMinutes
}

func shouldDisableByChannelHealth(setting *operation_setting.MonitorSetting, snapshot channelHealthSnapshot) bool {
	if setting == nil {
		return false
	}
	minRequests := setting.ChannelFailureRateMinRequests
	if minRequests < 1 {
		minRequests = 20
	}
	if snapshot.Total < minRequests {
		return false
	}
	threshold := setting.ChannelFailureRateThreshold
	if threshold <= 0 || threshold > 100 {
		threshold = 50
	}
	return snapshot.FailureRate >= threshold
}

func reserveChannelHealthDisable(channelId int) bool {
	now := time.Now()
	channelHealthDisableMu.Lock()
	defer channelHealthDisableMu.Unlock()

	if nextAllowed, ok := channelHealthDisableCooldowns[channelId]; ok && now.Before(nextAllowed) {
		return false
	}
	channelHealthDisableCooldowns[channelId] = now.Add(channelHealthDisableCooldown)
	return true
}

func recordChannelHealthEvent(channelId int, failed bool, windowMinutes int) (channelHealthSnapshot, error) {
	if common.RedisEnabled && common.RDB != nil {
		return recordChannelHealthRedis(channelId, failed, windowMinutes)
	}
	return recordChannelHealthMemory(channelId, failed, windowMinutes), nil
}

func recordChannelHealthRedis(channelId int, failed bool, windowMinutes int) (channelHealthSnapshot, error) {
	now := time.Now()
	bucket := now.Unix() / channelHealthBucketSeconds
	key := channelHealthRedisKey(channelId, bucket)
	expiration := time.Duration(windowMinutes+2) * time.Minute

	ctx := context.Background()
	txn := common.RDB.TxPipeline()
	txn.HIncrBy(ctx, key, "total", 1)
	if failed {
		txn.HIncrBy(ctx, key, "failed", 1)
	}
	txn.Expire(ctx, key, expiration)
	if _, err := txn.Exec(ctx); err != nil {
		return channelHealthSnapshot{}, err
	}

	var total int64
	var failedTotal int64
	firstBucket := bucket - int64(windowMinutes) + 1
	for b := firstBucket; b <= bucket; b++ {
		values, err := common.RDB.HGetAll(ctx, channelHealthRedisKey(channelId, b)).Result()
		if err != nil {
			return channelHealthSnapshot{}, err
		}
		total += parseChannelHealthCount(values["total"])
		failedTotal += parseChannelHealthCount(values["failed"])
	}
	return buildChannelHealthSnapshot(total, failedTotal, windowMinutes), nil
}

func recordChannelHealthMemory(channelId int, failed bool, windowMinutes int) channelHealthSnapshot {
	now := time.Now()
	bucket := now.Unix() / channelHealthBucketSeconds
	firstBucket := bucket - int64(windowMinutes) + 1

	channelHealthMemoryMu.Lock()
	defer channelHealthMemoryMu.Unlock()

	buckets := channelHealthMemoryBuckets[channelId]
	if buckets == nil {
		buckets = make(map[int64]channelHealthBucket)
		channelHealthMemoryBuckets[channelId] = buckets
	}
	current := buckets[bucket]
	current.Total++
	if failed {
		current.Failed++
	}
	buckets[bucket] = current

	var total int64
	var failedTotal int64
	for b, value := range buckets {
		if b < firstBucket {
			delete(buckets, b)
			continue
		}
		total += value.Total
		failedTotal += value.Failed
	}
	return buildChannelHealthSnapshot(total, failedTotal, windowMinutes)
}

func buildChannelHealthSnapshot(total int64, failed int64, windowMinutes int) channelHealthSnapshot {
	var failureRate float64
	if total > 0 {
		failureRate = float64(failed) / float64(total) * 100
	}
	return channelHealthSnapshot{
		Total:         total,
		Failed:        failed,
		FailureRate:   failureRate,
		WindowMinutes: windowMinutes,
	}
}

func channelHealthRedisKey(channelId int, bucket int64) string {
	return fmt.Sprintf("%s:%d:%d", channelHealthRedisKeyPrefix, channelId, bucket)
}

func parseChannelHealthCount(raw string) int64 {
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return value
}
