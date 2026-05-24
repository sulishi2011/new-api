package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareFullBodyFileTransfersOwnership(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "trace.tmp")
	require.NoError(t, os.WriteFile(filePath, []byte("body"), 0600))

	part := &TracePayloadPart{
		ContentType:    "text/plain",
		BodyObjectSize: 4,
		bodyFilePath:   filePath,
	}

	file, err := part.prepareFullBodyFile("request", "trace/request.body")
	require.NoError(t, err)
	require.NotNil(t, file)
	require.Equal(t, filePath, file.Path)
	require.Empty(t, part.bodyFilePath)

	payload := &TracePayload{Request: part}
	payload.CleanupFullBodyFiles()
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	file, err = part.prepareFullBodyFile("request", "trace/request-2.body")
	require.NoError(t, err)
	require.Nil(t, file)
}
