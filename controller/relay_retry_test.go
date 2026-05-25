package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRetrySkipsCanceledRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)

	err := types.NewErrorWithStatusCode(context.Canceled, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)

	require.False(t, shouldRetry(c, err, 1))
	require.True(t, isRequestContextCanceled(c, err))
}

func TestShouldRetrySkipsWrappedContextCanceled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	err := types.NewOpenAIError(
		errors.New("InvokeModelWithResponseStream: operation error Bedrock Runtime: InvokeModelWithResponseStream, context canceled"),
		types.ErrorCodeAwsInvokeError,
		http.StatusInternalServerError,
	)

	require.False(t, shouldRetry(c, err, 1))
	require.True(t, isRequestContextCanceled(c, err))
}

func TestShouldRetryAllowsRetryableStatusWhenRequestActive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	err := types.NewErrorWithStatusCode(errors.New("upstream unavailable"), types.ErrorCodeDoRequestFailed, http.StatusServiceUnavailable)

	require.True(t, shouldRetry(c, err, 1))
	require.False(t, isRequestContextCanceled(c, err))
}

func TestShouldRetryAllowsStreamResponseHeaderTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	err := types.NewErrorWithStatusCode(
		errors.New("stream response header timeout after 5s"),
		types.ErrorCodeStreamResponseHeaderTimeout,
		http.StatusServiceUnavailable,
	)

	retry, reason := shouldRetryWithReason(c, err, 1)
	require.True(t, retry)
	require.Equal(t, "status_code_retry", reason)
}

func TestShouldRetryRejectsStreamResponseHeaderTimeoutWithoutRemainingRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	err := types.NewErrorWithStatusCode(
		errors.New("stream response header timeout after 5s"),
		types.ErrorCodeStreamResponseHeaderTimeout,
		http.StatusServiceUnavailable,
	)

	retry, reason := shouldRetryWithReason(c, err, 0)
	require.False(t, retry)
	require.Equal(t, "no_remaining_retry", reason)
}

func TestSelectedChannelErrorUsesPolicyGroupFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	autoBan := 1
	channel := &model.Channel{
		Id:      88,
		Type:    14,
		Name:    "Prince-53",
		AutoBan: &autoBan,
	}

	common.SetContextKey(c, constant.ContextKeyChannelId, 88)
	common.SetContextKey(c, constant.ContextKeyChannelName, "Prince-53")
	common.SetContextKey(c, constant.ContextKeyChannelType, 14)
	common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
	common.SetContextKey(c, constant.ContextKeyChannelAutoBan, true)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "sk-test")
	common.SetContextKey(c, constant.ContextKeyChannelSetting, dto.ChannelSettings{
		AutoDisablePolicyGroup: "claude-aws-policy",
	})

	channelError := selectedChannelError(c, channel)

	require.Equal(t, 88, channelError.ChannelId)
	require.Equal(t, "Prince-53", channelError.ChannelName)
	require.Equal(t, 14, channelError.ChannelType)
	require.True(t, channelError.IsMultiKey)
	require.True(t, channelError.AutoBan)
	require.Equal(t, "sk-test", channelError.UsingKey)
	require.Equal(t, "claude-aws-policy", channelError.AutoDisablePolicyGroup)
}
