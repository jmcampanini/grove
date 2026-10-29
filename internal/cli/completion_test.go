package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataCommandsDoNotCreateLogs(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"--version"}, want: "grove version " + Version},
		{args: []string{"-v", "--worktree-template", "x"}, want: "grove version " + Version},
		{args: []string{"completion"}, want: "Print a shell completion script"},
		{args: []string{"completion", "bash"}, want: "__start_grove"},
		{args: []string{"completion", "zsh"}, want: "#compdef grove"},
		{args: []string{"completion", "fish"}, want: "complete -c grove"},
		{args: []string{"completion", "powershell"}, want: "Register-ArgumentCompleter"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			state := filepath.Join(t.TempDir(), "state")
			t.Setenv("XDG_STATE_HOME", state)
			var stdout bytes.Buffer

			err := executeWithFileLogging(strings.NewReader(""), &stdout, io.Discard, tt.args)

			require.NoError(t, err)
			assert.Contains(t, stdout.String(), tt.want)
			_, err = os.Stat(state)
			assert.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestMetadataCommandsReturnOutputErrors(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"completion", "bash"}, {"completion", "zsh"}, {"completion", "fish"}, {"completion", "powershell"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			err := executeWithFileLogging(strings.NewReader(""), failingWriter{}, io.Discard, args)
			require.ErrorIs(t, err, errWriteFailed)
		})
	}
}
