package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExitStatusClassifiesInvocationFailures covers the project's own
// mapping from the error an execution returns to the exit status: every
// failure Cobra reports while validating the invocation exits 2, every other
// failure exits 1.
func TestExitStatusClassifiesInvocationFailures(t *testing.T) {
	tests := []struct {
		args    []string
		name    string
		wantErr string
	}{
		{name: "unknown root command", args: []string{"nope"}, wantErr: `unknown command "nope" for "grove"`},
		{name: "unknown root command with a quote", args: []string{`a"b`}, wantErr: `unknown command "a\"b" for "grove"`},
		{name: "unknown group command", args: []string{"pr", "nope"}, wantErr: `unknown command "nope" for "grove pr"`},
		{name: "operand on a help topic", args: []string{"exit-codes", "x"}, wantErr: `unknown command "x" for "grove exit-codes"`},
		{name: "unknown flag", args: []string{"--bogus"}, wantErr: "unknown flag: --bogus"},
		{name: "missing operand", args: []string{"checkout"}, wantErr: "accepts 1 arg(s), received 0"},
		{name: "blank phrase", args: []string{"create", " "}, wantErr: "phrase cannot be empty"},
		{name: "blank branch", args: []string{"checkout", ""}, wantErr: "branch cannot be empty"},
		{name: "blank namer phrase", args: []string{"namer", "slug", ""}, wantErr: "phrase cannot be empty"},
		{name: "extra operand", args: []string{"checkout", "a", "b"}, wantErr: "accepts 1 arg(s), received 2"},
		{name: "pull request number not numeric", args: []string{"pr", "checkout", "abc"}, wantErr: `invalid pull request number "abc"`},
		{name: "pull request number not positive", args: []string{"pr", "preview", "--fzf", "0"}, wantErr: `invalid pull request number "0"`},
		{name: "issue number not numeric", args: []string{"issue", "start", "x"}, wantErr: `invalid issue number "x"`},
		{name: "invalid color mode", args: []string{"--color", "sometimes", "list"}, wantErr: `invalid argument "sometimes" for "--color" flag: valid values are auto, always, never`},
		{name: "conflicting diagnostic flags", args: []string{"--debug", "--quiet", "exit-codes"}, wantErr: "none of the others can be"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := executeWithFileLogging(strings.NewReader(""), &stdout, &stderr, tt.args)

			require.ErrorContains(t, err, tt.wantErr)
			assert.Equal(t, exitUsage, exitStatus(err))
			assert.Empty(t, stdout.String(), "a usage error prints no usage text on stdout")
		})
	}
}

func TestExitStatusForOtherOutcomes(t *testing.T) {
	assert.Equal(t, exitSuccess, exitStatus(nil))
	assert.Equal(t, exitFailure, exitStatus(errors.New("git fetch failed")))
	assert.Equal(t, exitFailure, exitStatus(errNotGitRepo))
	assert.Equal(t, exitUsage, exitStatus(usageError{err: errors.New("accepts 1 arg(s), received 0")}))
}

func TestExecuteSucceedsWithoutARepository(t *testing.T) {
	var stdout bytes.Buffer
	err := executeWithFileLogging(strings.NewReader(""), &stdout, io.Discard, []string{"help", "nope"})
	require.NoError(t, err)
	assert.Equal(t, exitSuccess, exitStatus(err))
	assert.Contains(t, stdout.String(), "Unknown help topic")
}

func TestReportFailureWritesOneLine(t *testing.T) {
	tests := []struct {
		err  error
		name string
		want string
	}{
		{name: "plain message", err: errors.New("branch \"x\" already exists"), want: "grove: branch \"x\" already exists\n"},
		{name: "usage error keeps its text", err: usageError{err: errors.New("unknown flag: --bogus")}, want: "grove: unknown flag: --bogus\n"},
		{name: "line breaks are folded", err: fmt.Errorf("git merge failed: %s", "exit status 1: CONFLICT\n  in a.go\n"), want: "grove: git merge failed: exit status 1: CONFLICT in a.go\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			reportFailure(&stderr, tt.err)
			assert.Equal(t, tt.want, stderr.String())
			assert.NotContains(t, stderr.String(), "rror:")
		})
	}
}
