package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

type UsageLedger struct {
	Id                   int      `json:"id"`
	NewapiLogId          int      `json:"newapi_log_id" gorm:"uniqueIndex"`
	RequestId            string   `json:"request_id" gorm:"type:varchar(64);index;default:''"`
	ExternalRequestId    string   `json:"external_request_id" gorm:"type:varchar(128);index;default:''"`
	Timestamp            int64    `json:"timestamp" gorm:"bigint;index;index:idx_usage_ledger_vendor_profile_time,priority:2;index:idx_usage_ledger_channel_time,priority:2;index:idx_usage_ledger_model_time,priority:2;index:idx_usage_ledger_token_time,priority:2;index:idx_usage_ledger_provider_key_time,priority:2;index:idx_usage_ledger_group_time,priority:2;index:idx_usage_ledger_biz_scene_time,priority:3"`
	UserId               int      `json:"user_id" gorm:"index;default:0"`
	Username             string   `json:"username" gorm:"type:varchar(64);index;default:''"`
	TokenId              int      `json:"token_id" gorm:"default:0;index:idx_usage_ledger_token_time,priority:1"`
	TokenName            string   `json:"token_name" gorm:"type:varchar(191);default:''"`
	ProviderKeyId        int      `json:"provider_key_id" gorm:"default:0;index:idx_usage_ledger_provider_key_time,priority:1"`
	ProviderKeyPreview   string   `json:"provider_key_preview" gorm:"type:varchar(255);default:''"`
	BizLine              string   `json:"biz_line" gorm:"type:varchar(32);index:idx_usage_ledger_biz_scene_time,priority:1;default:''"`
	BizScene             string   `json:"biz_scene" gorm:"type:varchar(64);index:idx_usage_ledger_biz_scene_time,priority:2;default:''"`
	UserTier             string   `json:"user_tier" gorm:"type:varchar(32);default:''"`
	Feature              string   `json:"feature" gorm:"type:varchar(64);default:''"`
	InternalUserId       string   `json:"internal_user_id" gorm:"type:varchar(128);default:''"`
	ConversationId       string   `json:"conversation_id" gorm:"type:varchar(128);default:''"`
	ClientVersion        string   `json:"client_version" gorm:"type:varchar(64);default:''"`
	GroupName            string   `json:"group_name" gorm:"type:varchar(64);index:idx_usage_ledger_group_time,priority:1;default:''"`
	RequestedModel       string   `json:"requested_model" gorm:"type:varchar(128);default:''"`
	ActualModel          string   `json:"actual_model" gorm:"type:varchar(128);index:idx_usage_ledger_model_time,priority:1;default:''"`
	NewapiChannelId      int      `json:"newapi_channel_id" gorm:"index:idx_usage_ledger_channel_time,priority:1;default:0"`
	ChannelName          string   `json:"channel_name" gorm:"type:varchar(191);default:''"`
	VendorProfileId      int      `json:"vendor_profile_id" gorm:"index:idx_usage_ledger_vendor_profile_time,priority:1;default:0"`
	VendorProfileCode    string   `json:"vendor_profile_code" gorm:"type:varchar(64);default:''"`
	VendorCode           string   `json:"vendor_code" gorm:"type:varchar(32);default:''"`
	VendorName           string   `json:"vendor_name" gorm:"type:varchar(128);default:''"`
	PlatformType         string   `json:"platform_type" gorm:"type:varchar(32);default:''"`
	DiscountCode         string   `json:"discount_code" gorm:"type:varchar(32);default:''"`
	DiscountRateSnapshot *float64 `json:"discount_rate_snapshot,omitempty" gorm:"type:decimal(18,8)"`
	InputTokens          int      `json:"input_tokens" gorm:"default:0"`
	OutputTokens         int      `json:"output_tokens" gorm:"default:0"`
	CacheWriteTokens     int      `json:"cache_write_tokens" gorm:"default:0"`
	CacheReadTokens      int      `json:"cache_read_tokens" gorm:"default:0"`
	ReasoningTokens      int      `json:"reasoning_tokens" gorm:"default:0"`
	TotalTokens          int      `json:"total_tokens" gorm:"default:0"`
	Quota                int      `json:"quota" gorm:"default:0"`
	CostQuota            int      `json:"cost_quota" gorm:"default:0"`
	Status               string   `json:"status" gorm:"type:varchar(32);default:'success'"`
	ErrorCode            string   `json:"error_code" gorm:"type:varchar(64);default:''"`
	LatencyMs            int      `json:"latency_ms" gorm:"default:0"`
	RetryCount           int      `json:"retry_count" gorm:"default:0"`
	CreatedAt            int64    `json:"created_at" gorm:"bigint"`
}

func intFromInterface(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case int32:
		return int(v)
	case uint:
		return int(v)
	case uint64:
		return int(v)
	case uint32:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}

func retryCountFromOther(other map[string]interface{}) int {
	if other == nil {
		return 0
	}
	adminInfo, _ := other["admin_info"].(map[string]interface{})
	length := 0
	switch channels := adminInfo["use_channel"].(type) {
	case []interface{}:
		length = len(channels)
	case []int:
		length = len(channels)
	case []string:
		length = len(channels)
	}
	if length <= 1 {
		return 0
	}
	return length - 1
}

func providerKeyPreviewFromOther(other map[string]interface{}) string {
	if other == nil {
		return ""
	}
	adminInfo, _ := other["admin_info"].(map[string]interface{})
	if adminInfo == nil {
		return ""
	}
	preview, _ := adminInfo["provider_key_preview"].(string)
	return preview
}

func CreateUsageLedgerFromLog(log *Log, other map[string]interface{}) {
	if log == nil || log.Type != LogTypeConsume || log.Id == 0 {
		return
	}
	if other == nil {
		other = make(map[string]interface{})
	}

	channelName := log.ChannelName
	var profile *VendorProfile
	if log.ChannelId > 0 {
		if channel, err := CacheGetChannel(log.ChannelId); err == nil && channel != nil {
			channelName = channel.Name
			if vendorProfileId := ChannelVendorProfileIdValue(channel.VendorProfileId); vendorProfileId > 0 {
				profile, _ = GetVendorProfileByID(vendorProfileId)
			}
		}
	}

	costQuota := 0
	if log.CostQuota != nil {
		costQuota = *log.CostQuota
	}

	ledger := UsageLedger{
		NewapiLogId:        log.Id,
		RequestId:          log.RequestId,
		ExternalRequestId:  log.ExternalRequestId,
		Timestamp:          log.CreatedAt,
		UserId:             log.UserId,
		Username:           log.Username,
		TokenId:            log.TokenId,
		TokenName:          log.TokenName,
		ProviderKeyId:      log.ProviderKeyId,
		ProviderKeyPreview: providerKeyPreviewFromOther(other),
		BizLine:            log.BizLine,
		BizScene:           log.BizScene,
		UserTier:           log.UserTier,
		Feature:            log.Feature,
		InternalUserId:     log.InternalUserId,
		ConversationId:     log.ConversationId,
		ClientVersion:      log.ClientVersion,
		GroupName:          log.Group,
		RequestedModel:     log.ModelName,
		ActualModel:        log.ModelName,
		NewapiChannelId:    log.ChannelId,
		ChannelName:        channelName,
		VendorProfileId:    log.VendorProfileId,
		VendorProfileCode:  log.VendorProfileCode,
		InputTokens:        log.PromptTokens,
		OutputTokens:       log.CompletionTokens,
		CacheWriteTokens:   intFromInterface(other["cache_write_tokens"]),
		CacheReadTokens:    intFromInterface(other["cache_tokens"]),
		ReasoningTokens:    intFromInterface(other["reasoning_tokens"]),
		TotalTokens:        log.PromptTokens + log.CompletionTokens,
		Quota:              log.Quota,
		CostQuota:          costQuota,
		Status:             "success",
		LatencyMs:          log.UseTime * 1000,
		RetryCount:         retryCountFromOther(other),
		CreatedAt:          common.GetTimestamp(),
	}

	if upstreamModel, ok := other["upstream_model_name"].(string); ok && upstreamModel != "" {
		ledger.ActualModel = upstreamModel
	}
	if totalInput := intFromInterface(other["input_tokens_total"]); totalInput > 0 {
		ledger.InputTokens = totalInput
	}
	if profile != nil {
		ledger.VendorProfileId = profile.Id
		ledger.VendorProfileCode = profile.Code
		ledger.VendorCode = profile.VendorCode
		ledger.VendorName = profile.VendorName
		ledger.PlatformType = profile.PlatformType
		ledger.DiscountCode = profile.DiscountCode
		ledger.DiscountRateSnapshot = profile.DiscountRate
	}

	if err := DB.Create(&ledger).Error; err != nil {
		common.SysLog(fmt.Sprintf("failed to create usage ledger: log_id=%d, error=%v", log.Id, err))
	}
}
