package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func saveAndRestoreLogger(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		Close()
		log.SetOutput(os.Stderr)
	})
}

func TestTeeWriter_Write(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantTerminal string
		wantFile     string
	}{
		{
			name:         "plain text passes through unchanged",
			input:        "hello world\n",
			wantTerminal: "hello world\n",
			wantFile:     "hello world\n",
		},
		{
			name:         "ANSI codes kept for terminal, stripped for file",
			input:        "\x1b[32mINFO\x1b[0m message\n",
			wantTerminal: "\x1b[32mINFO\x1b[0m message\n",
			wantFile:     "INFO message\n",
		},
		{
			name:         "multiple ANSI sequences stripped",
			input:        "\x1b[1m\x1b[36mDEBU\x1b[0m \x1b[90mgit:\x1b[0m executing\n",
			wantTerminal: "\x1b[1m\x1b[36mDEBU\x1b[0m \x1b[90mgit:\x1b[0m executing\n",
			wantFile:     "DEBU git: executing\n",
		},
		{
			name:         "empty input",
			input:        "",
			wantTerminal: "",
			wantFile:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var terminal, file bytes.Buffer

			tw := &teeWriter{
				terminal: &terminal,
				file:     &file,
			}

			n, err := tw.Write([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, len(tt.input), n)
			assert.Equal(t, tt.wantTerminal, terminal.String())
			assert.Equal(t, tt.wantFile, file.String())
		})
	}
}

func TestSetup_EmptyPath(t *testing.T) {
	err := Setup("")
	require.NoError(t, err)
	assert.Nil(t, logFile)
}

func TestSetup_CreatesFileAndDirs(t *testing.T) {
	saveAndRestoreLogger(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "grove.log")

	err := Setup(path)
	require.NoError(t, err)

	assert.NotNil(t, logFile)
	assert.FileExists(t, path)
}

func TestSetup_AppendsToExistingFile(t *testing.T) {
	saveAndRestoreLogger(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "grove.log")

	require.NoError(t, os.WriteFile(path, []byte("existing\n"), 0644))

	err := Setup(path)
	require.NoError(t, err)

	_, err = logFile.WriteString("new\n")
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "existing\nnew\n", string(content))
}

func TestClose_NilSafe(t *testing.T) {
	logFile = nil
	Close()
}
