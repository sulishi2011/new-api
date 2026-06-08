package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	channelLabel := formatChannelLabel(channelError)
	common.SysLog(fmt.Sprintf("%s 发生错误，准备禁用，原因：%s", channelLabel, common.LocalLogPreview(reason)))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("%s 未启用自动禁用功能，跳过禁用操作", channelLabel))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("%s 已被禁用", channelLabel)
		content := fmt.Sprintf("%s 已被禁用，原因：%s", channelLabel, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
		NotifyChannelDisabledWebhook(channelError, reason)
	}
}

func EnableChannel(channelId int, usingKey string, channelName string, vendorProfileCode ...string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		code := ""
		if len(vendorProfileCode) > 0 {
			code = vendorProfileCode[0]
		}
		channelLabel := formatChannelLabel(types.ChannelError{
			ChannelId:         channelId,
			ChannelName:       channelName,
			VendorProfileCode: code,
		})
		subject := fmt.Sprintf("%s 已被启用", channelLabel)
		content := fmt.Sprintf("%s 已被启用", channelLabel)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func ShouldDisableChannel(err *types.NewAPIError, policyGroupIDs ...string) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	policyGroupID := ""
	if len(policyGroupIDs) > 0 {
		policyGroupID = policyGroupIDs[0]
	}
	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.GetAutomaticDisablePolicyKeywords(policyGroupID), true)
	return search
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int, channelSettings ...dto.ChannelSettings) bool {
	automaticEnableChannelEnabled := common.AutomaticEnableChannelEnabled
	if len(channelSettings) > 0 {
		automaticEnableChannelEnabled = channelSettings[0].ResolveAutoRecoveryEnabled(automaticEnableChannelEnabled)
	}
	if !automaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
