package channel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestProcessHeaderOverride_ChannelTestSkipsPassthroughRules(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Empty(t, headers)
}

func TestProcessHeaderOverride_ChannelTestSkipsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	_, ok := headers["x-upstream-trace"]
	require.False(t, ok)
}

func TestProcessHeaderOverride_NonTestKeepsClientHeaderPlaceholder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Upstream-Trace": "{client_header:X-Trace-Id}",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-upstream-trace"])
}

func TestProcessHeaderOverride_RuntimeOverrideIsFinalHeaderMap(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	info := &relaycommon.RelayInfo{
		IsChannelTest:             false,
		UseRuntimeHeadersOverride: true,
		RuntimeHeadersOverride: map[string]any{
			"x-static":  "runtime-value",
			"x-runtime": "runtime-only",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
				"X-Legacy": "legacy-only",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "runtime-value", headers["x-static"])
	require.Equal(t, "runtime-only", headers["x-runtime"])
	_, exists := headers["x-legacy"]
	require.False(t, exists)
}

func TestProcessHeaderOverride_PassthroughSkipsAcceptEncoding(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("X-Trace-Id", "trace-123")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		ChannelMeta: &relaycommon.ChannelMeta{
			HeadersOverride: map[string]any{
				"*": "",
			},
		},
	}

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "trace-123", headers["x-trace-id"])

	_, hasAcceptEncoding := headers["accept-encoding"]
	require.False(t, hasAcceptEncoding)
}

func TestProcessHeaderOverride_PassHeadersTemplateSetsRuntimeHeaders(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	ctx.Request.Header.Set("Originator", "Codex CLI")
	ctx.Request.Header.Set("Session_id", "sess-123")

	info := &relaycommon.RelayInfo{
		IsChannelTest: false,
		RequestHeaders: map[string]string{
			"Originator": "Codex CLI",
			"Session_id": "sess-123",
		},
		ChannelMeta: &relaycommon.ChannelMeta{
			ParamOverride: map[string]any{
				"operations": []any{
					map[string]any{
						"mode":  "pass_headers",
						"value": []any{"Originator", "Session_id", "X-Codex-Beta-Features"},
					},
				},
			},
			HeadersOverride: map[string]any{
				"X-Static": "legacy-value",
			},
		},
	}

	_, err := relaycommon.ApplyParamOverrideWithRelayInfo([]byte(`{"model":"gpt-4.1"}`), info)
	require.NoError(t, err)
	require.True(t, info.UseRuntimeHeadersOverride)
	require.Equal(t, "Codex CLI", info.RuntimeHeadersOverride["originator"])
	require.Equal(t, "sess-123", info.RuntimeHeadersOverride["session_id"])
	_, exists := info.RuntimeHeadersOverride["x-codex-beta-features"]
	require.False(t, exists)
	require.Equal(t, "legacy-value", info.RuntimeHeadersOverride["x-static"])

	headers, err := processHeaderOverride(info, ctx)
	require.NoError(t, err)
	require.Equal(t, "Codex CLI", headers["originator"])
	require.Equal(t, "sess-123", headers["session_id"])
	_, exists = headers["x-codex-beta-features"]
	require.False(t, exists)

	upstreamReq := httptest.NewRequest(http.MethodPost, "https://example.com/v1/responses", nil)
	applyHeaderOverrideToRequest(upstreamReq, headers)
	require.Equal(t, "Codex CLI", upstreamReq.Header.Get("Originator"))
	require.Equal(t, "sess-123", upstreamReq.Header.Get("Session_id"))
	require.Empty(t, upstreamReq.Header.Get("X-Codex-Beta-Features"))
}

func TestNewDoRequestError_StreamResponseHeaderTimeout(t *testing.T) {
	t.Parallel()

	err := newDoRequestError(
		errors.New("Get \"https://upstream.example/v1/chat/completions\": net/http: timeout awaiting response headers"),
		&relaycommon.RelayInfo{IsStream: true},
		service.RelayHTTPClientPolicy{ResponseHeaderTimeout: 6 * time.Second},
	)

	var apiErr *types.NewAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, types.ErrorCodeStreamResponseHeaderTimeout, apiErr.GetErrorCode())
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "stream response header timeout after 6s")
	require.False(t, types.IsChannelError(apiErr))
}

func TestNewDoRequestError_NonStreamResponseHeaderTimeoutFallsBack(t *testing.T) {
	t.Parallel()

	err := newDoRequestError(
		errors.New("Get \"https://upstream.example/v1/chat/completions\": net/http: timeout awaiting response headers"),
		&relaycommon.RelayInfo{IsStream: false},
		service.RelayHTTPClientPolicy{ResponseHeaderTimeout: 6 * time.Second},
	)

	var apiErr *types.NewAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, types.ErrorCodeDoRequestFailed, apiErr.GetErrorCode())
	require.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
}

func TestNewDoRequestError_ContextCanceledSkipsRetry(t *testing.T) {
	t.Parallel()

	err := newDoRequestError(
		context.Canceled,
		&relaycommon.RelayInfo{IsStream: true},
		service.RelayHTTPClientPolicy{ResponseHeaderTimeout: 6 * time.Second},
	)

	var apiErr *types.NewAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, types.ErrorCodeDoRequestFailed, apiErr.GetErrorCode())
	require.Equal(t, 499, apiErr.StatusCode)
	require.True(t, types.IsSkipRetryError(apiErr))
}

func TestDoRequest_StreamResponseHeaderTimeoutBudgetIsPerAttempt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.ResetProxyClientCache()
	t.Cleanup(service.ResetProxyClientCache)

	newDelayedHeaderServer := func(delay time.Duration) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(delay)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}))
	}

	firstUpstream := newDelayedHeaderServer(1200 * time.Millisecond)
	defer firstUpstream.Close()
	secondUpstream := newDelayedHeaderServer(1200 * time.Millisecond)
	defer secondUpstream.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	firstReq, err := http.NewRequest(http.MethodPost, firstUpstream.URL, strings.NewReader(""))
	require.NoError(t, err)
	firstTimeout := 1
	firstEnabled := true
	firstInfo := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				StreamResponseHeaderTimeoutEnabled: &firstEnabled,
				StreamResponseHeaderTimeoutSeconds: &firstTimeout,
			},
		},
	}
	firstStart := time.Now()
	firstResp, err := doRequest(c, firstReq, firstInfo)
	require.Nil(t, firstResp)
	require.Error(t, err)
	require.GreaterOrEqual(t, time.Since(firstStart), 900*time.Millisecond)

	var firstAPIErr *types.NewAPIError
	require.ErrorAs(t, err, &firstAPIErr)
	require.Equal(t, types.ErrorCodeStreamResponseHeaderTimeout, firstAPIErr.GetErrorCode())

	secondReq, err := http.NewRequest(http.MethodPost, secondUpstream.URL, strings.NewReader(""))
	require.NoError(t, err)
	secondTimeout := 2
	secondEnabled := true
	secondInfo := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelSetting: dto.ChannelSettings{
				StreamResponseHeaderTimeoutEnabled: &secondEnabled,
				StreamResponseHeaderTimeoutSeconds: &secondTimeout,
			},
		},
	}
	secondStart := time.Now()
	secondResp, err := doRequest(c, secondReq, secondInfo)
	require.NoError(t, err)
	require.NotNil(t, secondResp)
	defer secondResp.Body.Close()
	require.Equal(t, http.StatusOK, secondResp.StatusCode)
	require.GreaterOrEqual(t, time.Since(secondStart), time.Second)
}
