package cmd

import (
	"bytes"
	"os"
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

func TestExecuteLogsPath_DisabledLogging(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "config"))

	configDir := filepath.Join(tmpDir, "config", "grove")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "grove.toml"),
		[]byte("[log]\nfile = \"\"\n"),
		0644,
	))

	var buf bytes.Buffer
	err := executeLogsPath(&buf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file logging is disabled")
}
