package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestIsFeishuWebhookURL(t *testing.T) {
	require.True(t, isFeishuWebhookURL("https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx"))
	require.True(t, isFeishuWebhookURL("https://open.larksuite.com/open-apis/bot/v2/hook/xxxxx"))
	require.False(t, isFeishuWebhookURL("https://example.com/open-apis/bot/v2/hook/xxxxx"))
}

func TestBuildFeishuWebhookPayloadWithoutSecret(t *testing.T) {
	payloadBytes, err := buildFeishuWebhookPayload("", dto.NewNotify("test", "标题", "内容", nil), "内容")
	require.NoError(t, err)

	var payload feishuWebhookPayload
	require.NoError(t, common.Unmarshal(payloadBytes, &payload))
	require.Equal(t, "interactive", payload.MsgType)
	require.Nil(t, payload.Content)
	require.NotNil(t, payload.Card)
	require.Equal(t, "标题", payload.Card.Header.Title.Content)
	require.Contains(t, payload.Card.Elements[0].Text.Content, "时间")
	require.Contains(t, payload.Card.Elements[2].Text.Content, "内容")
	require.Empty(t, payload.Timestamp)
	require.Empty(t, payload.Sign)
}

func TestBuildFeishuWebhookPayloadWithFields(t *testing.T) {
	notify := dto.NewNotifyWithFields(
		dto.NotifyTypeChannelRequestFailure,
		"通道「test」（#1）请求失败",
		"旧文本详情",
		nil,
		[]dto.NotifyField{
			{Label: "通道", Value: "JOHN-AWS-55「test」（#1）"},
			{Label: "状态码", Value: "500"},
			{Label: "模型", Value: "gpt-4o"},
			{Label: "请求路径", Value: "/v1/chat/completions"},
			{Label: "重试序号", Value: "0"},
			{Label: "错误", Value: "upstream error"},
		},
	)
	payloadBytes, err := buildFeishuWebhookPayload("", notify, notify.Content)
	require.NoError(t, err)

	var payload feishuWebhookPayload
	require.NoError(t, common.Unmarshal(payloadBytes, &payload))
	require.Equal(t, "interactive", payload.MsgType)
	require.Equal(t, "red", payload.Card.Header.Template)
	require.Contains(t, payload.Card.Elements[0].Text.Content, "时间")
	require.Contains(t, payload.Card.Elements[0].Text.Content, "**状态码** 500")
	require.Contains(t, payload.Card.Elements[0].Text.Content, "**路径** /v1/chat/completions")
	require.Contains(t, payload.Card.Elements[0].Text.Content, "\n")
	require.NotContains(t, payload.Card.Elements[0].Text.Content, "JOHN-AWS-55")
	require.Contains(t, payload.Card.Elements[2].Text.Content, "upstream error")
	require.NotContains(t, payload.Card.Elements[2].Text.Content, "旧文本详情")
}

func TestFormatChannelLabelIncludesVendorProfileCode(t *testing.T) {
	require.Equal(t, "JOHN-AWS-55「test」（#1）", formatChannelLabel(types.ChannelError{ChannelId: 1, ChannelName: "test", VendorProfileCode: " JOHN-AWS-55 "}))
	require.Equal(t, "通道「test」（#1）", formatChannelLabel(types.ChannelError{ChannelId: 1, ChannelName: "test"}))
}

func TestCompactFeishuValue(t *testing.T) {
	require.Equal(t, "a b c", compactFeishuValue(" a\nb\tc ", 20))
	require.Equal(t, "abcdef...", compactFeishuValue("abcdefgh", 6))
}

func TestCompactFeishuLabel(t *testing.T) {
	require.Equal(t, "路径", compactFeishuLabel("请求路径"))
	require.Equal(t, "重试", compactFeishuLabel("重试序号"))
	require.Equal(t, "模型", compactFeishuLabel("模型"))
}

func TestCheckWebhookResponseDetectsFeishuBusinessError(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"code":19024,"msg":"keywords not match"}`)),
	}

	err := checkWebhookResponse(resp, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "19024")
	require.Contains(t, err.Error(), "keywords not match")
}
