package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldSettleChatViaResponsesClientGoneAfterResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	cancelCtx, cancel := context.WithCancel(req.Context())
	cancel()
	ctx.Request = req.WithContext(cancelCtx)
	err := types.NewOpenAIError(fmt.Errorf("request context done: %w", context.Canceled), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	info := &relaycommon.RelayInfo{ReceivedResponseCount: 1}

	require.True(t, shouldSettleChatViaResponsesClientGone(ctx, info, err, false))
	require.True(t, shouldSettleChatViaResponsesClientGone(ctx, &relaycommon.RelayInfo{}, err, true))
}

func TestShouldSettleChatViaResponsesClientGoneRejectsUpstreamError(t *testing.T) {
	t.Parallel()

	err := types.NewOpenAIError(fmt.Errorf("upstream failed"), types.ErrorCodeBadResponse, http.StatusInternalServerError)

	require.False(t, shouldSettleChatViaResponsesClientGone(nil, &relaycommon.RelayInfo{ReceivedResponseCount: 1}, err, true))
}
