package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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
