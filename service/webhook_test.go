package service

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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
	require.Equal(t, "text", payload.MsgType)
	require.Equal(t, "标题\n内容", payload.Content.Text)
	require.Empty(t, payload.Timestamp)
	require.Empty(t, payload.Sign)
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
