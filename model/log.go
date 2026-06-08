package model

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

type requestAttribution struct {
	ExternalRequestId string
	BizLine           string
	BizScene          string
	UserTier          string
	Feature           string
	InternalUserId    string
	ConversationId    string
	ClientVersion     string
}

func limitHeaderValue(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit > 0 && len(value) > limit {
		return value[:limit]
	}
	return value
}

func getRequestAttribution(c *gin.Context) requestAttribution {
	if c == nil {
		return requestAttribution{}
	}
	return requestAttribution{
		ExternalRequestId: limitHeaderValue(c.GetHeader("X-Request-Id"), 128),
		BizLine:           limitHeaderValue(c.GetHeader("X-Biz-Line"), 32),
		BizScene:          limitHeaderValue(c.GetHeader("X-Biz-Scene"), 64),
		UserTier:          limitHeaderValue(c.GetHeader("X-User-Tier"), 32),
		Feature:           limitHeaderValue(c.GetHeader("X-Feature"), 64),
		InternalUserId:    limitHeaderValue(c.GetHeader("X-Internal-User-Id"), 128),
		ConversationId:    limitHeaderValue(c.GetHeader("X-Conversation-Id"), 128),
		ClientVersion:     limitHeaderValue(c.GetHeader("X-Client-Version"), 64),
	}
}

func applyExplicitLogTextFilter(tx *gorm.DB, column string, value string) (*gorm.DB, error) {
	if value == "" {
		return tx, nil
	}
	if strings.Contains(value, "%") {
		pattern, err := sanitizeLikePattern(value)
		if err != nil {
			return nil, err
		}
		return tx.Where(column+" LIKE ? ESCAPE '!'", pattern), nil
	}
	return tx.Where(column+" = ?", value), nil
}

type Log struct {
	Id                int64  `json:"id" gorm:"primaryKey;index:idx_created_at_id,priority:2;index:idx_logs_type_created_id,priority:3;index:idx_logs_channel_created_id,priority:3"`
	UserId            int    `json:"user_id" gorm:"index"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:1;index:idx_logs_type_created_id,priority:2;index:idx_logs_channel_created_id,priority:2;index:idx_logs_vendor_profile_created,priority:2;index:idx_logs_biz_line_scene_created,priority:3"`
	Type              int    `json:"type" gorm:"index:idx_logs_type_created_id,priority:1"`
	Content           string `json:"content"`
	Username          string `json:"username" gorm:"index;default:''"`
	TokenName         string `json:"token_name" gorm:"index;default:''"`
	ModelName         string `json:"model_name" gorm:"index;default:''"`
	Quota             int    `json:"quota" gorm:"default:0"`
	PromptTokens      int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int    `json:"completion_tokens" gorm:"default:0"`
	UseTime           int    `json:"use_time" gorm:"default:0"`
	IsStream          bool   `json:"is_stream"`
	ChannelId         int    `json:"channel" gorm:"index;index:idx_logs_channel_created_id,priority:1"`
	ChannelName       string `json:"channel_name" gorm:"->"`
	TokenId           int    `json:"token_id" gorm:"default:0;index"`
	Group             string `json:"group" gorm:"index"`
	Ip                string `json:"ip" gorm:"index;default:''"`
	RequestId         string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	ExternalRequestId string `json:"external_request_id,omitempty" gorm:"type:varchar(128);index;default:''"`
	UpstreamRequestId string `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	BizLine           string `json:"biz_line,omitempty" gorm:"type:varchar(32);index:idx_logs_biz_line_scene_created,priority:1;default:''"`
	BizScene          string `json:"biz_scene,omitempty" gorm:"type:varchar(64);index:idx_logs_biz_line_scene_created,priority:2;default:''"`
	UserTier          string `json:"user_tier,omitempty" gorm:"type:varchar(32);default:''"`
	Feature           string `json:"feature,omitempty" gorm:"type:varchar(64);default:''"`
	InternalUserId    string `json:"internal_user_id,omitempty" gorm:"type:varchar(128);default:''"`
	ConversationId    string `json:"conversation_id,omitempty" gorm:"type:varchar(128);default:''"`
	ClientVersion     string `json:"client_version,omitempty" gorm:"type:varchar(64);default:''"`
	VendorProfileId   int    `json:"vendor_profile_id,omitempty" gorm:"index:idx_logs_vendor_profile_created,priority:1;default:0"`
	VendorProfileCode string `json:"vendor_profile_code,omitempty" gorm:"type:varchar(64);index;default:''"`
	ProviderKeyId     int    `json:"provider_key_id,omitempty" gorm:"index;default:0"`
	CostQuota         *int   `json:"cost_quota,omitempty"`
	Other             string `json:"other"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			// delete(otherMap, "reject_reason")
			delete(otherMap, "stream_status")
			delete(otherMap, "trace")
			delete(otherMap, "trace_ref")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].ProviderKeyId = 0
		logs[i].Id = int64(startIdx + i + 1)
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	tableName := currentLogReadTable()
	err = logReadDB().Table(logReadTableExpr(tableName)).Where("logs.token_id = ?", tokenId).Order("logs.created_at desc, logs.id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := createLog(log)
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// RecordLogWithAdminInfo 记录操作日志，并将管理员相关信息存入 Other.admin_info，
func RecordLogWithAdminInfo(userId int, logType int, content string, adminInfo map[string]interface{}) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	if len(adminInfo) > 0 {
		other := map[string]interface{}{
			"admin_info": adminInfo,
		}
		log.Other = common.MapToJsonStr(other)
	}
	if err := createLog(log); err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

func RecordTopupLog(userId int, content string, callerIp string, paymentMethod string, callbackPaymentMethod string) {
	username, _ := GetUsernameById(userId, false)
	adminInfo := map[string]interface{}{
		"server_ip":               common.GetIp(),
		"node_name":               common.NodeName,
		"caller_ip":               callerIp,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 common.Version,
	}
	other := map[string]interface{}{
		"admin_info": adminInfo,
	}
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeTopup,
		Content:   content,
		Ip:        callerIp,
		Other:     common.MapToJsonStr(other),
	}
	err := createLog(log)
	if err != nil {
		common.SysLog("failed to record topup log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, common.LocalLogPreview(content)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	attribution := getRequestAttribution(c)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	vendorProfileId, vendorProfileCode := resolveVendorProfileByChannelID(channelId)
	other, providerKeyId := appendProviderKeyInfo(c, other)
	trace := prepareLogTrace(LogTypeError, other)
	otherStr := common.MapToJsonStr(other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		ExternalRequestId: attribution.ExternalRequestId,
		UpstreamRequestId: upstreamRequestId,
		BizLine:           attribution.BizLine,
		BizScene:          attribution.BizScene,
		UserTier:          attribution.UserTier,
		Feature:           attribution.Feature,
		InternalUserId:    attribution.InternalUserId,
		ConversationId:    attribution.ConversationId,
		ClientVersion:     attribution.ClientVersion,
		VendorProfileId:   vendorProfileId,
		VendorProfileCode: vendorProfileCode,
		ProviderKeyId:     providerKeyId,
		Other:             otherStr,
	}
	err := createLog(log)
	if err != nil {
		cleanupLogTraceFullBodyFiles(trace)
		logger.LogError(c, "failed to record log: "+err.Error())
	} else {
		persistLogTrace(c.Request.Context(), log, trace)
	}
}

func sanitizeOtherForConsoleLog(other map[string]interface{}) map[string]interface{} {
	if other == nil {
		return nil
	}
	sanitized := make(map[string]interface{}, len(other))
	for key, value := range other {
		sanitized[key] = value
	}
	if _, ok := sanitized["trace"]; ok {
		sanitized["trace"] = map[string]interface{}{
			"stored_in_log": true,
			"omitted":       true,
		}
	}
	if rawAdminInfo, ok := sanitized["admin_info"].(map[string]interface{}); ok {
		adminInfo := make(map[string]interface{}, len(rawAdminInfo))
		for key, value := range rawAdminInfo {
			adminInfo[key] = value
		}
		if _, ok := adminInfo["provider_key"]; ok {
			adminInfo["provider_key"] = "***stored_in_log***"
		}
		sanitized["admin_info"] = adminInfo
	}
	return sanitized
}

func appendProviderKeyInfo(c *gin.Context, other map[string]interface{}) (map[string]interface{}, int) {
	if c == nil {
		return other, 0
	}
	rawKey := common.GetContextKeyString(c, constant.ContextKeyChannelKey)
	if rawKey == "" {
		return other, 0
	}
	providerKey, err := GetOrCreateProviderKey(rawKey)
	if err != nil {
		common.SysLog("failed to resolve provider key: " + err.Error())
		return other, 0
	}
	if other == nil {
		other = make(map[string]interface{})
	}
	adminInfo, _ := other["admin_info"].(map[string]interface{})
	if adminInfo == nil {
		adminInfo = make(map[string]interface{})
	}
	adminInfo["provider_key_id"] = providerKey.Id
	adminInfo["provider_key"] = rawKey
	if providerKey.KeyPreview != "" {
		adminInfo["provider_key_preview"] = providerKey.KeyPreview
	}
	other["admin_info"] = adminInfo
	return other, providerKey.Id
}

func normalizeCostRatio(costRatio float64) float64 {
	if math.IsNaN(costRatio) || math.IsInf(costRatio, 0) || costRatio < 0 {
		return 1
	}
	return costRatio
}

func getChannelCostRatioByID(channelID int) float64 {
	if channelID <= 0 {
		return 1
	}
	channel, err := CacheGetChannel(channelID)
	if err != nil {
		return 1
	}
	if costRatio, ok := getVendorProfileCostRatio(channel); ok {
		return costRatio
	}
	return normalizeCostRatio(channel.GetSetting().GetCostRatio())
}

func resolveChannelCostRatio(c *gin.Context, channelID int) float64 {
	var channel *Channel
	if channelID > 0 {
		if cachedChannel, err := CacheGetChannel(channelID); err == nil {
			channel = cachedChannel
			if costRatio, ok := getVendorProfileCostRatio(channel); ok {
				return costRatio
			}
		}
	}
	if c != nil {
		channelSetting, ok := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting)
		if ok {
			return normalizeCostRatio(channelSetting.GetCostRatio())
		}
	}
	if channel != nil {
		return normalizeCostRatio(channel.GetSetting().GetCostRatio())
	}
	return 1
}

func getVendorProfileCostRatio(channel *Channel) (float64, bool) {
	if channel == nil {
		return 0, false
	}
	vendorProfileId := ChannelVendorProfileIdValue(channel.VendorProfileId)
	if vendorProfileId <= 0 {
		return 0, false
	}
	profile := channel.VendorProfile
	if profile == nil || profile.Id == 0 {
		var err error
		profile, err = GetVendorProfileByID(vendorProfileId)
		if err != nil || profile == nil {
			return 0, false
		}
	}
	if profile.DiscountRate == nil {
		return 0, false
	}
	return normalizeCostRatio(*profile.DiscountRate), true
}

func resolveVendorProfileByChannelID(channelID int) (int, string) {
	if channelID <= 0 {
		return 0, ""
	}
	channel, err := CacheGetChannel(channelID)
	if err != nil || channel == nil {
		return 0, ""
	}
	vendorProfileId := ChannelVendorProfileIdValue(channel.VendorProfileId)
	if vendorProfileId <= 0 {
		return 0, ""
	}
	if channel.VendorProfile != nil && channel.VendorProfile.Code != "" {
		return vendorProfileId, channel.VendorProfile.Code
	}
	profile, err := GetVendorProfileByID(vendorProfileId)
	if err != nil || profile == nil {
		return vendorProfileId, ""
	}
	return profile.Id, profile.Code
}

func calculateCostQuota(quota int, costRatio float64) *int {
	costQuota := int(math.Round(float64(quota) * normalizeCostRatio(costRatio)))
	return &costQuota
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	costRatio := resolveChannelCostRatio(c, params.ChannelId)
	costQuota := calculateCostQuota(params.Quota, costRatio)
	var providerKeyId int
	params.Other, providerKeyId = appendProviderKeyInfo(c, params.Other)
	if params.Other == nil {
		params.Other = make(map[string]interface{})
	}
	params.Other["cost_ratio"] = costRatio
	loggedParams := params
	loggedParams.Other = sanitizeOtherForConsoleLog(params.Other)
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(loggedParams)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	attribution := getRequestAttribution(c)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	vendorProfileId, vendorProfileCode := resolveVendorProfileByChannelID(params.ChannelId)
	trace := prepareLogTrace(LogTypeConsume, params.Other)
	otherStr := common.MapToJsonStr(params.Other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		ExternalRequestId: attribution.ExternalRequestId,
		UpstreamRequestId: upstreamRequestId,
		BizLine:           attribution.BizLine,
		BizScene:          attribution.BizScene,
		UserTier:          attribution.UserTier,
		Feature:           attribution.Feature,
		InternalUserId:    attribution.InternalUserId,
		ConversationId:    attribution.ConversationId,
		ClientVersion:     attribution.ClientVersion,
		VendorProfileId:   vendorProfileId,
		VendorProfileCode: vendorProfileCode,
		ProviderKeyId:     providerKeyId,
		CostQuota:         costQuota,
		Other:             otherStr,
	}
	err := createLog(log)
	if err != nil {
		cleanupLogTraceFullBodyFiles(trace)
		logger.LogError(c, "failed to record log: "+err.Error())
	} else {
		persistLogTrace(c.Request.Context(), log, trace)
		CreateUsageLedgerFromLog(log, params.Other)
	}
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
}

type RecordTaskBillingLogParams struct {
	UserId    int
	LogType   int
	Content   string
	ChannelId int
	ModelName string
	Quota     int
	TokenId   int
	Group     string
	Other     map[string]interface{}
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	costRatio := getChannelCostRatioByID(params.ChannelId)
	costQuota := calculateCostQuota(params.Quota, costRatio)
	if params.Other == nil {
		params.Other = make(map[string]interface{})
	}
	params.Other["cost_ratio"] = costRatio
	trace := prepareLogTrace(params.LogType, params.Other)
	log := &Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		CostQuota: costQuota,
		Other:     common.MapToJsonStr(params.Other),
	}
	err := createLog(log)
	if err != nil {
		cleanupLogTraceFullBodyFiles(trace)
		common.SysLog("failed to record task billing log: " + err.Error())
	} else if params.LogType == LogTypeConsume {
		persistLogTrace(context.Background(), log, trace)
		CreateUsageLedgerFromLog(log, params.Other)
	} else {
		persistLogTrace(context.Background(), log, trace)
	}
}

type LogQueryOptions struct {
	ExternalRequestId string
	UpstreamRequestId string
	VendorProfileId   int
	BizLine           string
	BizScene          string
	UserTier          string
	Feature           string
}

func applyLogRangeFilter(tx *gorm.DB, tableRange logReadTableRange, columnPrefix string) *gorm.DB {
	column := columnPrefix + "created_at"
	if tableRange.StartTimestamp != 0 {
		tx = tx.Where(column+" >= ?", tableRange.StartTimestamp)
	}
	if tableRange.EndTimestamp != 0 {
		tx = tx.Where(column+" <= ?", tableRange.EndTimestamp)
	}
	return tx
}

func buildAllLogsReadQuery(db *gorm.DB, tableRange logReadTableRange, logType int, modelName string, username string, tokenName string, channel int, group string, requestId string, providerKeyId int, opts LogQueryOptions) (*gorm.DB, error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = db.Table(logReadTableExpr(tableRange.TableName))
	} else {
		tx = db.Table(logReadTableExpr(tableRange.TableName)).Where("logs.type = ?", logType)
	}

	var err error
	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.username", username); err != nil {
		return nil, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if opts.ExternalRequestId != "" {
		tx = tx.Where("logs.external_request_id = ?", opts.ExternalRequestId)
	}
	if providerKeyId != 0 {
		tx = tx.Where("logs.provider_key_id = ?", providerKeyId)
	}
	if opts.VendorProfileId != 0 {
		tx = tx.Where("logs.vendor_profile_id = ?", opts.VendorProfileId)
	}
	if opts.BizLine != "" {
		tx = tx.Where("logs.biz_line = ?", opts.BizLine)
	}
	if opts.BizScene != "" {
		tx = tx.Where("logs.biz_scene = ?", opts.BizScene)
	}
	if opts.UserTier != "" {
		tx = tx.Where("logs.user_tier = ?", opts.UserTier)
	}
	if opts.Feature != "" {
		tx = tx.Where("logs.feature = ?", opts.Feature)
	}
	if opts.UpstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", opts.UpstreamRequestId)
	}
	tx = applyLogRangeFilter(tx, tableRange, "logs.")
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	return tx, nil
}

func buildUserLogsReadQuery(db *gorm.DB, tableRange logReadTableRange, userId int, logType int, modelName string, tokenName string, group string, requestId string, providerKeyId int, opts LogQueryOptions) (*gorm.DB, error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = db.Table(logReadTableExpr(tableRange.TableName)).Where("logs.user_id = ?", userId)
	} else {
		tx = db.Table(logReadTableExpr(tableRange.TableName)).Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	var err error
	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if opts.ExternalRequestId != "" {
		tx = tx.Where("logs.external_request_id = ?", opts.ExternalRequestId)
	}
	if providerKeyId != 0 {
		tx = tx.Where("logs.provider_key_id = ?", providerKeyId)
	}
	if opts.VendorProfileId != 0 {
		tx = tx.Where("logs.vendor_profile_id = ?", opts.VendorProfileId)
	}
	if opts.BizLine != "" {
		tx = tx.Where("logs.biz_line = ?", opts.BizLine)
	}
	if opts.BizScene != "" {
		tx = tx.Where("logs.biz_scene = ?", opts.BizScene)
	}
	if opts.UserTier != "" {
		tx = tx.Where("logs.user_tier = ?", opts.UserTier)
	}
	if opts.Feature != "" {
		tx = tx.Where("logs.feature = ?", opts.Feature)
	}
	if opts.UpstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", opts.UpstreamRequestId)
	}
	tx = applyLogRangeFilter(tx, tableRange, "logs.")
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	return tx, nil
}

func countLogsAcrossRanges(ranges []logReadTableRange, buildQuery func(*gorm.DB, logReadTableRange) (*gorm.DB, error)) ([]int64, int64, error) {
	counts := make([]int64, len(ranges))
	var total int64
	for i, tableRange := range ranges {
		tx, err := buildQuery(logReadDB(), tableRange)
		if err != nil {
			return nil, 0, err
		}
		if err := tx.Count(&counts[i]).Error; err != nil {
			return nil, 0, err
		}
		total += counts[i]
	}
	return counts, total, nil
}

func findLogsAcrossRanges(ranges []logReadTableRange, counts []int64, startIdx int, num int, buildQuery func(*gorm.DB, logReadTableRange) (*gorm.DB, error)) ([]*Log, error) {
	if num <= 0 {
		return []*Log{}, nil
	}

	logs := make([]*Log, 0, num)
	remainingOffset := int64(startIdx)
	remainingLimit := num
	for i := len(ranges) - 1; i >= 0 && remainingLimit > 0; i-- {
		if remainingOffset >= counts[i] {
			remainingOffset -= counts[i]
			continue
		}

		var rangeLogs []*Log
		tx, err := buildQuery(logReadDB(), ranges[i])
		if err != nil {
			return nil, err
		}
		err = tx.
			Order("logs.created_at desc, logs.id desc").
			Limit(remainingLimit).
			Offset(int(remainingOffset)).
			Find(&rangeLogs).Error
		if err != nil {
			return nil, err
		}
		logs = append(logs, rangeLogs...)
		remainingLimit -= len(rangeLogs)
		remainingOffset = 0
	}
	return logs, nil
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, providerKeyId int) (logs []*Log, total int64, err error) {
	return GetAllLogsWithOptions(logType, startTimestamp, endTimestamp, modelName, username, tokenName, startIdx, num, channel, group, requestId, providerKeyId, LogQueryOptions{})
}

func GetAllLogsWithOptions(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, providerKeyId int, opts LogQueryOptions) (logs []*Log, total int64, err error) {
	ranges, err := resolveLogReadTableRanges(startTimestamp, endTimestamp)
	if err != nil {
		return nil, 0, err
	}

	buildQuery := func(db *gorm.DB, tableRange logReadTableRange) (*gorm.DB, error) {
		return buildAllLogsReadQuery(db, tableRange, logType, modelName, username, tokenName, channel, group, requestId, providerKeyId, opts)
	}
	counts, total, err := countLogsAcrossRanges(ranges, buildQuery)
	if err != nil {
		return nil, 0, err
	}
	logs, err = findLogsAcrossRanges(ranges, counts, startIdx, num, buildQuery)
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = readDB().Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, providerKeyId int) (logs []*Log, total int64, err error) {
	return GetUserLogsWithOptions(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, startIdx, num, group, requestId, providerKeyId, LogQueryOptions{})
}

func GetUserLogsWithOptions(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, providerKeyId int, opts LogQueryOptions) (logs []*Log, total int64, err error) {
	ranges, err := resolveLogReadTableRanges(startTimestamp, endTimestamp)
	if err != nil {
		return nil, 0, err
	}

	buildQuery := func(db *gorm.DB, tableRange logReadTableRange) (*gorm.DB, error) {
		return buildUserLogsReadQuery(db, tableRange, userId, logType, modelName, tokenName, group, requestId, providerKeyId, opts)
	}
	counts, total, err := countLogsAcrossRanges(ranges, buildQuery)
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	if total > logSearchCountLimit {
		total = logSearchCountLimit
	}
	logs, err = findLogsAcrossRanges(ranges, counts, startIdx, num, buildQuery)
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func isPostgresRecoveryConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "sqlstate 40001") && strings.Contains(msg, "conflict with recovery")
}

func isLogReadReplicaMissingTable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "doesn't exist") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "undefined table")
}

func scanLogReadWithPrimaryFallback(queryName string, buildQuery func(*gorm.DB) (*gorm.DB, error), dest interface{}) error {
	readDB := logReadDB()
	tx, err := buildQuery(readDB)
	if err != nil {
		return err
	}
	err = tx.Scan(dest).Error
	if err == nil {
		return nil
	}
	if LOG_DB == nil || readDB == LOG_DB || (!isPostgresRecoveryConflict(err) && !isLogReadReplicaMissingTable(err)) {
		return err
	}

	common.SysLog("log read replica query failed, retrying " + queryName + " on primary: " + err.Error())
	retryTx, buildErr := buildQuery(LOG_DB)
	if buildErr != nil {
		return buildErr
	}
	if retryErr := retryTx.Scan(dest).Error; retryErr != nil {
		return fmt.Errorf("%s read replica query failed: %v; primary retry failed: %w", queryName, err, retryErr)
	}
	return nil
}

func uniqueLogReadTableNames(ranges []logReadTableRange) []string {
	seen := make(map[string]struct{}, len(ranges))
	tableNames := make([]string, 0, len(ranges))
	for _, tableRange := range ranges {
		if _, ok := seen[tableRange.TableName]; ok {
			continue
		}
		seen[tableRange.TableName] = struct{}{}
		tableNames = append(tableNames, tableRange.TableName)
	}
	return tableNames
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string, externalRequestId string, providerKeyId int, vendorProfileId int, bizLine string, bizScene string, userTier string, feature string) (stat Stat, err error) {
	ranges, err := resolveLogReadTableRanges(startTimestamp, endTimestamp)
	if err != nil {
		return stat, err
	}

	buildQuotaQuery := func(db *gorm.DB, tableRange logReadTableRange) (*gorm.DB, error) {
		tx := db.Table(logRawTableExpr(tableRange.TableName)).Select("sum(quota) quota")
		var err error
		if tx, err = applyExplicitLogTextFilter(tx, "username", username); err != nil {
			return nil, err
		}
		if tokenName != "" {
			tx = tx.Where("token_name = ?", tokenName)
		}
		tx = applyLogRangeFilter(tx, tableRange, "")
		if tx, err = applyExplicitLogTextFilter(tx, "model_name", modelName); err != nil {
			return nil, err
		}
		if channel != 0 {
			tx = tx.Where("channel_id = ?", channel)
		}
		if externalRequestId != "" {
			tx = tx.Where("external_request_id = ?", externalRequestId)
		}
		if providerKeyId != 0 {
			tx = tx.Where("provider_key_id = ?", providerKeyId)
		}
		if vendorProfileId != 0 {
			tx = tx.Where("vendor_profile_id = ?", vendorProfileId)
		}
		if bizLine != "" {
			tx = tx.Where("biz_line = ?", bizLine)
		}
		if bizScene != "" {
			tx = tx.Where("biz_scene = ?", bizScene)
		}
		if userTier != "" {
			tx = tx.Where("user_tier = ?", userTier)
		}
		if feature != "" {
			tx = tx.Where("feature = ?", feature)
		}
		if group != "" {
			tx = tx.Where(logGroupCol+" = ?", group)
		}
		return tx.Where("type = ?", LogTypeConsume), nil
	}

	rpmTpmStart := time.Now().Add(-60 * time.Second).Unix()
	buildRpmTpmQuery := func(db *gorm.DB, tableName string) (*gorm.DB, error) {
		tx := db.Table(logRawTableExpr(tableName)).Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")
		var err error
		if tx, err = applyExplicitLogTextFilter(tx, "username", username); err != nil {
			return nil, err
		}
		if tokenName != "" {
			tx = tx.Where("token_name = ?", tokenName)
		}
		if tx, err = applyExplicitLogTextFilter(tx, "model_name", modelName); err != nil {
			return nil, err
		}
		if channel != 0 {
			tx = tx.Where("channel_id = ?", channel)
		}
		if externalRequestId != "" {
			tx = tx.Where("external_request_id = ?", externalRequestId)
		}
		if providerKeyId != 0 {
			tx = tx.Where("provider_key_id = ?", providerKeyId)
		}
		if vendorProfileId != 0 {
			tx = tx.Where("vendor_profile_id = ?", vendorProfileId)
		}
		if bizLine != "" {
			tx = tx.Where("biz_line = ?", bizLine)
		}
		if bizScene != "" {
			tx = tx.Where("biz_scene = ?", bizScene)
		}
		if userTier != "" {
			tx = tx.Where("user_tier = ?", userTier)
		}
		if feature != "" {
			tx = tx.Where("feature = ?", feature)
		}
		if group != "" {
			tx = tx.Where(logGroupCol+" = ?", group)
		}
		return tx.Where("type = ?", LogTypeConsume).Where("created_at >= ?", rpmTpmStart), nil
	}

	for _, tableRange := range ranges {
		var partial Stat
		if err := scanLogReadWithPrimaryFallback("log quota stat", func(db *gorm.DB) (*gorm.DB, error) {
			return buildQuotaQuery(db, tableRange)
		}, &partial); err != nil {
			common.SysError("failed to query log stat: " + err.Error())
			return stat, errors.New("查询统计数据失败")
		}
		stat.Quota += partial.Quota
	}
	for _, tableName := range uniqueLogReadTableNames(ranges) {
		var partial Stat
		if err := scanLogReadWithPrimaryFallback("log rpm/tpm stat", func(db *gorm.DB) (*gorm.DB, error) {
			return buildRpmTpmQuery(db, tableName)
		}, &partial); err != nil {
			common.SysError("failed to query rpm/tpm stat: " + err.Error())
			return stat, errors.New("查询统计数据失败")
		}
		stat.Rpm += partial.Rpm
		stat.Tpm += partial.Tpm
	}

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	ranges, err := resolveLogReadTableRanges(startTimestamp, endTimestamp)
	if err != nil {
		return 0
	}
	sumExpr := "ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)"
	if logDatabaseType() == common.DatabaseTypePostgreSQL {
		sumExpr = "COALESCE(sum(prompt_tokens),0) + COALESCE(sum(completion_tokens),0)"
	}
	for _, tableRange := range ranges {
		var partial int
		tx := logReadDB().Table(logRawTableExpr(tableRange.TableName)).Select(sumExpr)
		if username != "" {
			tx = tx.Where("username = ?", username)
		}
		if tokenName != "" {
			tx = tx.Where("token_name = ?", tokenName)
		}
		tx = applyLogRangeFilter(tx, tableRange, "")
		if modelName != "" {
			tx = tx.Where("model_name = ?", modelName)
		}
		tx.Where("type = ?", LogTypeConsume).Scan(&partial)
		token += partial
	}
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	return 0, errors.New("日志已启用按月分表，按时间批量清理暂未启用")
}

func DeleteLogsByTypeBefore(ctx context.Context, logType int, targetTimestamp int64, limit int) (int64, error) {
	return 0, errors.New("日志已启用按月分表，按类型按时间清理暂未启用")
}
