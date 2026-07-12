package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const helperEnv = "GROVE_PROCESS_HELPER"

func TestProcessHelper(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		return
	}

	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator == -1 || separator+1 >= len(os.Args) {
		os.Exit(2)
	}

	args := os.Args[separator+1:]
	switch args[0] {
	case "child":
		signal.Ignore(os.Interrupt, syscall.SIGTERM)
		require.NoError(t, os.WriteFile(args[1], []byte(strconv.Itoa(os.Getpid())), 0600))
		for {
			time.Sleep(time.Second)
		}
	case "delay":
		delay, err := time.ParseDuration(args[1])
		require.NoError(t, err)
		time.Sleep(delay)
		_, _ = fmt.Fprint(os.Stdout, "delayed success\n")
	case "fail":
		_, _ = fmt.Fprint(os.Stdout, "partial stdout\n")
		_, _ = fmt.Fprint(os.Stderr, "failure detail\n")
		os.Exit(23)
	case "oversize":
		stream := os.Stdout
		if args[1] == "stderr" {
			stream = os.Stderr
		}
		chunk := []byte(strings.Repeat("sensitive-value", 4096))
		for written := 0; written <= OutputLimit; written += len(chunk) {
			_, _ = stream.Write(chunk)
		}
	case "streams":
		_, _ = fmt.Fprint(os.Stdout, " stdout value \n")
		_, _ = fmt.Fprint(os.Stderr, "stderr value\n")
	case "tree":
		signal.Ignore(os.Interrupt, syscall.SIGTERM)
		parentPIDPath, childPIDPath, readyPath := args[1], args[2], args[3]
		require.NoError(t, os.WriteFile(parentPIDPath, []byte(strconv.Itoa(os.Getpid())), 0600))
		child := exec.Command(os.Args[0], "-test.run=TestProcessHelper", "--", "child", childPIDPath)
		child.Env = append(os.Environ(), helperEnv+"=1")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		require.NoError(t, child.Start())
		require.Eventually(t, func() bool {
			_, err := os.Stat(childPIDPath)
			return err == nil
		}, 2*time.Second, 10*time.Millisecond)
		require.NoError(t, os.WriteFile(readyPath, []byte("ready"), 0600))
		for {
			time.Sleep(time.Second)
		}
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

func helperSpec(mode string, args ...string) Spec {
	commandArgs := []string{"-test.run=TestProcessHelper", "--", mode}
	commandArgs = append(commandArgs, args...)
	return Spec{
		Args: commandArgs,
		Env:  append(os.Environ(), helperEnv+"=1"),
		Name: os.Args[0],
	}
}

func TestRunBasicOutcomes(t *testing.T) {
	tests := []struct {
		assertResult func(*testing.T, Result, error)
		name         string
		spec         Spec
		timeout      time.Duration
	}{
		{
			name:    "zero timeout permits delayed success",
			spec:    helperSpec("delay", "100ms"),
			timeout: 0,
			assertResult: func(t *testing.T, result Result, err error) {
				require.NoError(t, err)
				assert.Equal(t, "delayed success\n", string(result.Stdout))
			},
		},
		{
			name:    "stdout and stderr remain separate",
			spec:    helperSpec("streams"),
			timeout: 5 * time.Second,
			assertResult: func(t *testing.T, result Result, err error) {
				require.NoError(t, err)
				assert.Equal(t, " stdout value \n", string(result.Stdout))
				assert.Equal(t, "stderr value\n", string(result.Stderr))
			},
		},
		{
			name:    "nonzero exit preserves output and exit error",
			spec:    helperSpec("fail"),
			timeout: 5 * time.Second,
			assertResult: func(t *testing.T, result Result, err error) {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				assert.Equal(t, 23, exitErr.ExitCode())
				assert.Equal(t, "partial stdout\n", string(result.Stdout))
				assert.Equal(t, "failure detail\n", string(result.Stderr))
			},
		},
		{
			name: "executable not found remains discoverable",
			spec: Spec{
				Name: filepath.Join(t.TempDir(), "missing-executable"),
			},
			timeout: time.Second,
			assertResult: func(t *testing.T, _ Result, err error) {
				var pathErr *os.PathError
				require.ErrorAs(t, err, &pathErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Run(context.Background(), tt.spec, tt.timeout)
			tt.assertResult(t, result, err)
		})
	}
}

func TestRunTimeoutAndCancellation(t *testing.T) {
	tests := []struct {
		ctx         func() (context.Context, context.CancelFunc)
		name        string
		rejectError error
		timeout     time.Duration
		wantError   error
	}{
		{
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			name:      "configured timeout",
			timeout:   50 * time.Millisecond,
			wantError: ErrTimedOut,
		},
		{
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 50*time.Millisecond)
			},
			name:        "caller deadline is cancellation",
			rejectError: ErrTimedOut,
			timeout:     0,
			wantError:   ErrCanceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.ctx()
			defer cancel()
			_, err := Run(ctx, helperSpec("delay", "2s"), tt.timeout)
			require.ErrorIs(t, err, tt.wantError)
			if tt.rejectError != nil {
				assert.NotErrorIs(t, err, tt.rejectError)
			}
			var exitErr *exec.ExitError
			assert.ErrorAs(t, err, &exitErr)
		})
	}
}

func TestRunOutputLimit(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			result, err := Run(context.Background(), helperSpec("oversize", stream), 5*time.Second)
			require.ErrorIs(t, err, ErrOutputLimitExceeded)
			var limitErr *OutputLimitError
			require.ErrorAs(t, err, &limitErr)
			assert.Equal(t, stream, limitErr.Stream)
			assert.Equal(t, OutputLimit, limitErr.Limit)
			assert.NotContains(t, err.Error(), "sensitive-value")
			if stream == "stdout" {
				assert.Len(t, result.Stdout, OutputLimit)
				assert.Empty(t, result.Stderr)
			} else {
				assert.Len(t, result.Stderr, OutputLimit)
				assert.Empty(t, result.Stdout)
			}
		})
	}
}

func TestRunCancellationTerminatesProcessGroup(t *testing.T) {
	dir := t.TempDir()
	parentPIDPath := filepath.Join(dir, "parent.pid")
	childPIDPath := filepath.Join(dir, "child.pid")
	readyPath := filepath.Join(dir, "ready")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(readyPath); err == nil {
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}()

	_, err := Run(ctx, helperSpec("tree", parentPIDPath, childPIDPath, readyPath), 0)
	require.ErrorIs(t, err, ErrCanceled)
	require.FileExists(t, readyPath)
	assertProcessTreeGone(t, parentPIDPath, childPIDPath)
}

func TestRunTimeoutTerminatesProcessGroup(t *testing.T) {
	dir := t.TempDir()
	parentPIDPath := filepath.Join(dir, "parent.pid")
	childPIDPath := filepath.Join(dir, "child.pid")
	readyPath := filepath.Join(dir, "ready")

	_, err := Run(
		context.Background(),
		helperSpec("tree", parentPIDPath, childPIDPath, readyPath),
		2*time.Second,
	)
	require.ErrorIs(t, err, ErrTimedOut)
	require.FileExists(t, readyPath)

	assertProcessTreeGone(t, parentPIDPath, childPIDPath)
}

func assertProcessTreeGone(t *testing.T, parentPIDPath, childPIDPath string) {
	t.Helper()
	parentPID := readPID(t, parentPIDPath)
	childPID := readPID(t, childPIDPath)
	require.Eventually(t, func() bool {
		return !processExists(parentPID) && !processExists(childPID) && !processGroupExists(parentPID)
	}, 3*time.Second, 20*time.Millisecond)
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	pid, err := strconv.Atoi(string(data))
	require.NoError(t, err)
	return pid
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func processGroupExists(pgid int) bool {
	err := syscall.Kill(-pgid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
