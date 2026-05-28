package aws

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDoAwsClientRequest_AppliesRuntimeHeaderOverrideToAnthropicBeta(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	info := &relaycommon.RelayInfo{
		OriginModelName:           "claude-3-5-sonnet-20240620",
		IsStream:                  false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"anthropic-beta": "computer-use-2025-01-24",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:            "access-key|secret-key|us-east-1",
			UpstreamModelName: "claude-3-5-sonnet-20240620",
		},
	}

	requestBody := bytes.NewBufferString(`{"messages":[{"role":"user","content":"hello"}],"max_tokens":128}`)
	adaptor := &Adaptor{}

	_, err := doAwsClientRequest(ctx, info, adaptor, requestBody)
	require.NoError(t, err)

	awsReq, ok := adaptor.AwsReq.(*bedrockruntime.InvokeModelInput)
	require.True(t, ok)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(awsReq.Body, &payload))

	anthropicBeta, exists := payload["anthropic_beta"]
	require.True(t, exists)

	values, ok := anthropicBeta.([]any)
	require.True(t, ok)
	require.Equal(t, []any{"computer-use-2025-01-24"}, values)
}

func TestNewAwsInvokeErrorStreamResponseHeaderTimeout(t *testing.T) {
	t.Parallel()

	seconds := 6
	err := newAwsInvokeError(
		errors.New("Post \"https://bedrock-runtime.example/model\": net/http: timeout awaiting response headers"),
		"InvokeModelWithResponseStream",
		&relaycommon.RelayInfo{
			IsStream: true,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelSetting: dto.ChannelSettings{
					StreamResponseHeaderTimeoutEnabled: boolPtr(true),
					StreamResponseHeaderTimeoutSeconds: &seconds,
				},
			},
		},
		http.StatusInternalServerError,
	)

	require.Equal(t, types.ErrorCodeStreamResponseHeaderTimeout, err.GetErrorCode())
	require.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
	require.Contains(t, err.Error(), "stream response header timeout after 6s")
}

func TestNewAwsInvokeErrorContextCanceledSkipsRetry(t *testing.T) {
	t.Parallel()

	err := newAwsInvokeError(
		errors.New("operation error Bedrock Runtime: InvokeModelWithResponseStream, context canceled"),
		"InvokeModelWithResponseStream",
		&relaycommon.RelayInfo{
			IsStream: true,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelSetting: dto.ChannelSettings{
					StreamResponseHeaderTimeoutEnabled: boolPtr(true),
					StreamResponseHeaderTimeoutSeconds: intPtr(20),
				},
			},
		},
		http.StatusInternalServerError,
	)

	require.Equal(t, types.ErrorCodeAwsInvokeError, err.GetErrorCode())
	require.Equal(t, 499, err.StatusCode)
	require.True(t, types.IsSkipRetryError(err))
}

func TestNewAwsInvokeContextStreamDoesNotUseRequestTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := newAwsInvokeContext(nil, &relaycommon.RelayInfo{
		IsStream: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				RequestTimeoutEnabled:              boolPtr(true),
				RequestTimeoutSeconds:              intPtr(5),
				StreamResponseHeaderTimeoutEnabled: boolPtr(true),
				StreamResponseHeaderTimeoutSeconds: intPtr(20),
			},
		},
	})
	defer cancel()

	_, ok := ctx.Deadline()
	require.False(t, ok)
}

func TestFinalizeAwsStreamClientGoneSettlesPartialUsageAfterResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	var responseText strings.Builder
	responseText.WriteString("partial")
	info := &relaycommon.RelayInfo{
		RelayFormat:           types.RelayFormatClaude,
		IsStream:              true,
		StartTime:             time.Now().Add(-time.Second),
		OriginModelName:       "claude-3-5-sonnet-20240620",
		ReceivedResponseCount: 1,
		StreamStatus:          relaycommon.NewStreamStatus(),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-3-5-sonnet-20240620",
		},
	}
	info.SetEstimatePromptTokens(77)
	info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, context.Canceled)
	claudeInfo := &claude.ClaudeResponseInfo{
		Usage: &dto.Usage{
			PromptTokens:     123,
			CompletionTokens: 5,
		},
		ResponseText: responseText,
	}

	apiErr, usage := finalizeAwsStreamClientGone(ctx, info, claudeInfo, context.Canceled)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 123, usage.PromptTokens)
	require.GreaterOrEqual(t, usage.CompletionTokens, 5)
	require.Equal(t, "anthropic", usage.UsageSemantic)
}

func TestFinalizeAwsStreamClientGoneKeepsErrorBeforeResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat:  types.RelayFormatClaude,
		IsStream:     true,
		StreamStatus: relaycommon.NewStreamStatus(),
	}
	claudeInfo := &claude.ClaudeResponseInfo{Usage: &dto.Usage{}}

	apiErr, usage := finalizeAwsStreamClientGone(ctx, info, claudeInfo, context.Canceled)

	require.NotNil(t, apiErr)
	require.Nil(t, usage)
	require.Equal(t, types.ErrorCodeChannelResponseTimeExceeded, apiErr.GetErrorCode())
}

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
