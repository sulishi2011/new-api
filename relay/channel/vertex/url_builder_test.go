package vertex

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildGoogleModelURL(t *testing.T) {
	require.Equal(
		t,
		"https://aiplatform.googleapis.com/v1/projects/project-1/locations/global/publishers/google/models/gemini-2.5-pro:generateContent",
		BuildGoogleModelURL("", DefaultAPIVersion, "project-1", "global", "gemini-2.5-pro", "generateContent"),
	)

	require.Equal(
		t,
		"https://us-central1-aiplatform.googleapis.com/v1/projects/project-1/locations/us-central1/publishers/google/models/gemini-2.5-pro:generateContent",
		BuildGoogleModelURL("", DefaultAPIVersion, "project-1", "us-central1", "gemini-2.5-pro", "generateContent"),
	)

	require.Equal(
		t,
		"http://gateway.local/vertex/v1/projects/project-1/locations/us-central1/publishers/google/models/gemini-2.5-pro:generateContent",
		BuildGoogleModelURL("http://gateway.local/vertex", DefaultAPIVersion, "project-1", "us-central1", "gemini-2.5-pro", "generateContent"),
	)

	require.Equal(
		t,
		"http://gateway.local/vertex/v1/projects/project-1/locations/us-central1/publishers/google/models/gemini-2.5-pro:generateContent",
		BuildGoogleModelURL("http://gateway.local/vertex/v1", DefaultAPIVersion, "project-1", "us-central1", "gemini-2.5-pro", "generateContent"),
	)
}

func TestBuildModelURLWithoutProject(t *testing.T) {
	require.Equal(
		t,
		"http://gateway.local/v1/publishers/google/models/gemini-2.5-pro:streamGenerateContent?alt=sse",
		BuildGoogleModelURL("http://gateway.local", DefaultAPIVersion, "", "global", "gemini-2.5-pro", "streamGenerateContent?alt=sse"),
	)

	require.Equal(
		t,
		"http://gateway.local/v1/publishers/anthropic/models/claude-sonnet-4@20250514:rawPredict",
		BuildAnthropicModelURL("http://gateway.local", DefaultAPIVersion, "", "us-central1", "claude-sonnet-4@20250514", "rawPredict"),
	)
}

func TestBuildOpenSourceChatCompletionsURL(t *testing.T) {
	require.Equal(
		t,
		"http://gateway.local/v1beta1/projects/project-1/locations/us-central1/endpoints/openapi/chat/completions",
		BuildOpenSourceChatCompletionsURL("http://gateway.local", "project-1", "us-central1"),
	)
}
