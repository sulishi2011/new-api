package dto

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestMessageReasoningContentPreservesEmptyString(t *testing.T) {
	raw := []byte(`{
		"role": "assistant",
		"content": "Hello",
		"reasoning_content": "",
		"reasoning": ""
	}`)

	var msg Message
	err := common.Unmarshal(raw, &msg)
	require.NoError(t, err)
	require.NotNil(t, msg.ReasoningContent)
	require.NotNil(t, msg.Reasoning)
	require.Equal(t, "", *msg.ReasoningContent)
	require.Equal(t, "", *msg.Reasoning)

	encoded, err := common.Marshal(msg)
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	err = common.Unmarshal(encoded, &fields)
	require.NoError(t, err)
	require.Contains(t, fields, "reasoning_content")
	require.Contains(t, fields, "reasoning")
}

func TestMessageReasoningContentOmitsAbsentField(t *testing.T) {
	raw := []byte(`{
		"role": "assistant",
		"content": "Hello"
	}`)

	var msg Message
	err := common.Unmarshal(raw, &msg)
	require.NoError(t, err)
	require.Nil(t, msg.ReasoningContent)
	require.Nil(t, msg.Reasoning)

	encoded, err := common.Marshal(msg)
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	err = common.Unmarshal(encoded, &fields)
	require.NoError(t, err)
	require.NotContains(t, fields, "reasoning_content")
	require.NotContains(t, fields, "reasoning")
}

func TestMessageGetReasoningContent(t *testing.T) {
	msg := Message{Role: "assistant"}
	require.Equal(t, "", msg.GetReasoningContent())

	reasoningContent := "thinking"
	reasoning := "fallback"
	msg = Message{ReasoningContent: &reasoningContent, Reasoning: &reasoning}
	require.Equal(t, "thinking", msg.GetReasoningContent())

	msg = Message{Reasoning: &reasoning}
	require.Equal(t, "fallback", msg.GetReasoningContent())
}
