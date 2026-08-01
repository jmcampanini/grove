package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteLogsPath(t *testing.T) {
	stateDir := filepath.Join(t.TempDir(), "state")
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", t.TempDir())

	var buf bytes.Buffer
	err := executeLogsPath(&buf)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(stateDir, "grove", "grove.log")+"\n", buf.String())
}
