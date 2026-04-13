package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteLogsPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmpDir, "state"))

	var buf bytes.Buffer
	err := executeLogsPath(&buf)
	require.NoError(t, err)

	out := buf.String()
	assert.True(t, strings.HasSuffix(out, "grove/grove.log\n"))
	assert.True(t, filepath.IsAbs(strings.TrimSpace(out)))
}
