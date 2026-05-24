package types

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewOpenAIErrorContextCanceledUses499AndSkipsRetry(t *testing.T) {
	err := NewOpenAIError(
		fmt.Errorf("InvokeModelWithResponseStream: %w", context.Canceled),
		ErrorCodeAwsInvokeError,
		http.StatusInternalServerError,
	)

	require.Equal(t, 499, err.StatusCode)
	require.Equal(t, ErrorCodeAwsInvokeError, err.GetErrorCode())
	require.True(t, IsSkipRetryError(err))
	require.ErrorIs(t, err, context.Canceled)
}

func TestNewOpenAIErrorContextCanceledMessageUses499AndSkipsRetry(t *testing.T) {
	err := NewOpenAIError(
		fmt.Errorf("InvokeModelWithResponseStream: operation error Bedrock Runtime: InvokeModelWithResponseStream, context canceled"),
		ErrorCodeAwsInvokeError,
		http.StatusInternalServerError,
	)

	require.Equal(t, 499, err.StatusCode)
	require.Equal(t, ErrorCodeAwsInvokeError, err.GetErrorCode())
	require.True(t, IsSkipRetryError(err))
}
