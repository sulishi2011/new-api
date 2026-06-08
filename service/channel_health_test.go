package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testFeishuWebhookURL = "https://open.feishu.cn/open-apis/bot/v2/hook/xxxxx"

func TestShouldSkipRequestFailureFeishuWebhookForTestGroup(t *testing.T) {
	require.True(t, shouldSkipRequestFailureFeishuWebhook(nil, &relaycommon.RelayInfo{UsingGroup: "test"}, testFeishuWebhookURL))
	require.False(t, shouldSkipRequestFailureFeishuWebhook(nil, &relaycommon.RelayInfo{UsingGroup: "default"}, testFeishuWebhookURL))
	require.False(t, shouldSkipRequestFailureFeishuWebhook(nil, &relaycommon.RelayInfo{UsingGroup: "test"}, "https://example.com/webhook"))
}

func TestShouldSkipRequestFailureFeishuWebhookUsesEffectiveGroup(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyTokenGroup, "test")

	require.False(t, shouldSkipRequestFailureFeishuWebhook(c, nil, testFeishuWebhookURL))
	require.False(t, shouldSkipRequestFailureFeishuWebhook(c, &relaycommon.RelayInfo{UsingGroup: "default"}, testFeishuWebhookURL))
	require.False(t, shouldSkipRequestFailureFeishuWebhook(c, &relaycommon.RelayInfo{TokenGroup: " test "}, testFeishuWebhookURL))
}

func TestShouldSkipRequestFailureFeishuWebhookFallsBackToContextGroup(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyTokenGroup, " test ")

	require.True(t, shouldSkipRequestFailureFeishuWebhook(c, nil, testFeishuWebhookURL))
	require.True(t, shouldSkipRequestFailureFeishuWebhook(nil, &relaycommon.RelayInfo{TokenGroup: " test "}, testFeishuWebhookURL))
}
