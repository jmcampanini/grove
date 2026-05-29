package logging

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/log/v2"
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

type errWriter struct{ err error }

func (e *errWriter) Write([]byte) (int, error) { return 0, e.err }

func TestTeeWriter_TerminalError_StillWritesToFile(t *testing.T) {
	var file bytes.Buffer
	termErr := errors.New("terminal broken")

	tw := &teeWriter{
		terminal: &errWriter{err: termErr},
		file:     &file,
	}

	n, err := tw.Write([]byte("hello\n"))
	assert.Equal(t, 0, n)
	assert.ErrorIs(t, err, termErr)
	assert.Equal(t, "hello\n", file.String(), "file should get full content even when terminal fails")
}

func TestTeeWriter_FileError_LogsWarningOnce(t *testing.T) {
	var terminal bytes.Buffer
	fileErr := errors.New("disk full")

	tw := &teeWriter{
		terminal: &terminal,
		file:     &errWriter{err: fileErr},
	}

	_, _ = tw.Write([]byte("line 1\n"))
	_, _ = tw.Write([]byte("line 2\n"))

	output := terminal.String()
	assert.Contains(t, output, "WARN file logging failed: disk full")
	assert.Equal(t, 1, strings.Count(output, "WARN file logging failed"),
		"warning should be logged only once")
}

func TestSetup_DoubleCall_ClosesPreviousFile(t *testing.T) {
	saveAndRestoreLogger(t)

	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.log")
	pathB := filepath.Join(dir, "b.log")

	require.NoError(t, Setup(pathA))
	firstFile := logFile

	require.NoError(t, Setup(pathB))

	_, writeErr := firstFile.WriteString("should fail")
	assert.Error(t, writeErr, "first file should be closed after second Setup call")
	assert.FileExists(t, pathB)
}

func TestSetup_Integration_LogReachesFile(t *testing.T) {
	saveAndRestoreLogger(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "grove.log")

	require.NoError(t, Setup(path))
	log.Info("integration test message", "key", "value")

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	text := string(content)
	assert.Contains(t, text, "integration test message")
	assert.Contains(t, text, "key=value")
	assert.NotContains(t, text, "\x1b[", "file should not contain ANSI codes")
}
