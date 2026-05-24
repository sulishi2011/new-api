package service

import (
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoTakesTracePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	now := time.Now()
	payload := &relaycommon.TracePayload{
		Version:           1,
		UpstreamRequestId: "upstream-req",
	}
	info := &relaycommon.RelayInfo{
		StartTime:         now,
		FirstResponseTime: now,
		TracePayload:      payload,
		ChannelMeta:       &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(ctx, info, 1, 1, 1, 0, 0, 0, 1)

	trace, ok := other["trace"].(*relaycommon.TracePayload)
	require.True(t, ok)
	require.Same(t, payload, trace)
	require.Nil(t, info.TracePayload)

	next := make(map[string]interface{})
	AppendTraceOtherInfo(info, next)
	require.NotContains(t, next, "trace")
}
