package github

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	clog "charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/process"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunGhProcessContract(t *testing.T) {
	dir := installFakeGh(t)

	tests := []struct {
		assertError func(*testing.T, error)
		ctx         func() (context.Context, context.CancelFunc)
		mode        string
		name        string
		timeout     time.Duration
		wantOutput  string
	}{
		{
			mode:       "delayed",
			name:       "zero timeout and prompt disabled",
			timeout:    0,
			wantOutput: "1",
		},
		{
			assertError: func(t *testing.T, err error) {
				require.ErrorIs(t, err, process.ErrTimedOut)
				assert.Contains(t, err.Error(), "timed out after 50ms")
			},
			mode:    "sleep",
			name:    "configured timeout",
			timeout: 50 * time.Millisecond,
		},
		{
			assertError: func(t *testing.T, err error) {
				require.ErrorIs(t, err, process.ErrCanceled)
				assert.Contains(t, err.Error(), "canceled")
			},
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				time.AfterFunc(50*time.Millisecond, cancel)
				return ctx, cancel
			},
			mode:    "sleep",
			name:    "caller cancellation",
			timeout: 0,
		},
		{
			assertError: func(t *testing.T, err error) {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				assert.Equal(t, 23, exitErr.ExitCode())
				assert.Contains(t, err.Error(), "failure detail")
			},
			mode:    "fail",
			name:    "nonzero exit",
			timeout: time.Second,
		},
		{
			assertError: func(t *testing.T, err error) {
				require.ErrorIs(t, err, process.ErrOutputLimitExceeded)
				assert.Contains(t, err.Error(), "stdout exceeded 8 MiB")
				assert.NotContains(t, err.Error(), "sensitive-value")
			},
			mode:    "oversize-stdout",
			name:    "oversized stdout",
			timeout: 5 * time.Second,
		},
		{
			assertError: func(t *testing.T, err error) {
				require.ErrorIs(t, err, process.ErrOutputLimitExceeded)
				assert.Contains(t, err.Error(), "stderr exceeded 8 MiB")
				assert.NotContains(t, err.Error(), "sensitive-value")
			},
			mode:    "oversize-stderr",
			name:    "oversized stderr",
			timeout: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cancel := func() {}
			if tt.ctx != nil {
				ctx, cancel = tt.ctx()
			}
			defer cancel()

			var logs bytes.Buffer
			client := New(ctx, dir, tt.timeout, nil, nil).(*GitHubCli)
			client.log = clog.New(&logs)
			output, err := client.runGhProcess(tt.mode)
			if tt.assertError != nil {
				require.Error(t, err)
				tt.assertError(t, err)
				assert.Empty(t, output)
				assert.NotContains(t, logs.String(), "failure detail")
				assert.NotContains(t, logs.String(), "partial output")
				assert.NotContains(t, logs.String(), "sensitive-value")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, output)
		})
	}
}

func TestRunGhProcessExecutableNotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	client := New(context.Background(), dir, time.Second, nil, nil).(*GitHubCli)
	client.log = clog.New(io.Discard)

	_, err := client.runGhProcess("version")

	var execErr *exec.Error
	require.ErrorAs(t, err, &execErr)
	assert.True(t, errors.Is(execErr.Err, exec.ErrNotFound))
}

func installFakeGh(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	script := `#!/bin/sh
case "$1" in
  delayed)
    sleep 0.1
    printf '%s' "$GH_PROMPT_DISABLED"
    ;;
  fail)
    printf 'partial output\n'
    printf 'failure detail\n' >&2
    exit 23
    ;;
  oversize-stdout)
    yes sensitive-value
    ;;
  oversize-stderr)
    yes sensitive-value >&2
    ;;
  sleep)
    sleep 10
    ;;
esac
`
	require.NoError(t, os.WriteFile(path, []byte(script), 0755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}
