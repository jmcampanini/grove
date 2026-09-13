package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// workspaceSnapshot captures the state a mutating grove command could change:
// git refs, registered worktrees, and every file under the fixture including
// the fake gh call log, the cache, the config, and the state directory.
type workspaceSnapshot struct {
	files     map[string]string
	refs      string
	worktrees string
}

func TestRejectedOperandsLeaveWorkspaceUnchanged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake gh process uses a POSIX shell")
	}

	testDir := t.TempDir()
	homeDir := filepath.Join(testDir, "home")
	mainDir := filepath.Join(testDir, "main")
	worktreeDir := filepath.Join(testDir, "wt-test")
	fakeBinDir := filepath.Join(testDir, "bin")
	require.NoError(t, os.MkdirAll(homeDir, 0o755))
	require.NoError(t, os.MkdirAll(mainDir, 0o755))
	require.NoError(t, os.MkdirAll(fakeBinDir, 0o755))

	t.Setenv("HOME", homeDir)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("PATH", fakeBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TERM", "dumb")
	t.Setenv("XDG_CACHE_HOME", filepath.Join(testDir, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(testDir, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(testDir, "state"))

	runGrammarTestGit(t, mainDir, "init", "-q", "-b", "main")
	runGrammarTestGit(t, mainDir, "config", "user.email", "test@example.com")
	runGrammarTestGit(t, mainDir, "config", "user.name", "Test")
	runGrammarTestGit(t, mainDir, "commit", "--allow-empty", "-qm", "init")
	runGrammarTestGit(t, mainDir, "worktree", "add", "-qb", "feature/test", worktreeDir)
	runGrammarTestGit(t, mainDir, "remote", "add", "origin", "https://example.invalid/test.git")
	head := strings.TrimSpace(runGrammarTestGit(t, mainDir, "rev-parse", "feature/test"))
	runGrammarTestGit(t, mainDir, "update-ref", "refs/remotes/origin/feature/test", head)
	runGrammarTestGit(t, mainDir, "update-ref", "refs/remotes/origin/main", head)
	runGrammarTestGit(t, mainDir, "branch", "--set-upstream-to=origin/feature/test", "feature/test")

	// The fake gh records every call and answers with an empty list, so any
	// gh invocation shows up as a change to gh.log in the snapshot.
	t.Setenv("FAKE_GH_LOG", filepath.Join(testDir, "gh.log"))
	require.NoError(t, os.WriteFile(filepath.Join(fakeBinDir, "gh"), []byte(`#!/bin/sh
printf '%s\n' "$*" >>"$FAKE_GH_LOG"
printf '[]\n'
`), 0o755))

	// A valid command proves grove operates on this fixture and lets its own
	// side effects (log file, cache entries) land before the snapshot.
	stdout, stderr, exitCode := runGroveProcess(t, mainDir, "status")
	require.Equal(t, 0, exitCode, stderr)
	require.Contains(t, stdout, "feature/test")
	before := snapshotWorkspace(t, testDir, mainDir)
	require.Contains(t, before.files, "state/grove/grove.log", "the valid command must have created the log file")

	rejected := [][]string{
		{"cache", "clear", "extra"},
		{"catchup", "extra"},
		{"checkout", "feature/test", "extra"},
		{"create", "phrase", "extra"},
		{"issue", "start", "1", "extra"},
		{"pr", "checkout", "1", "extra"},
		{"prune", "extra"},
		{"remove", "wt-test", "extra"},
		{"sync", "extra"},
	}
	for _, args := range rejected {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, exitCode := runGroveProcess(t, mainDir, args...)

			assert.NotEqual(t, 0, exitCode)
			assert.Empty(t, stdout)
			assert.Regexp(t, "grove: error: "+strings.TrimPrefix(arityErrorPattern, "^"), stderr)
			assert.Equal(t, before, snapshotWorkspace(t, testDir, mainDir))
		})
	}
}

func snapshotWorkspace(t *testing.T, testDir, mainDir string) workspaceSnapshot {
	t.Helper()
	return workspaceSnapshot{
		files:     snapshotTree(t, testDir),
		refs:      runGrammarTestGit(t, mainDir, "for-each-ref"),
		worktrees: runGrammarTestGit(t, mainDir, "worktree", "list", "--porcelain"),
	}
}

// TestGroveCommandProcess is the re-exec entry point for runGroveProcess. It
// runs grove on the process streams with the arguments passed through the
// environment and exits the way main does.
func TestGroveCommandProcess(t *testing.T) {
	if os.Getenv("GROVE_GRAMMAR_COMMAND_PROCESS") != "1" {
		return
	}

	var args []string
	if err := json.Unmarshal([]byte(os.Getenv("GROVE_GRAMMAR_ARGS")), &args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := executeWithFileLogging(os.Stdin, os.Stdout, os.Stderr, args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "grove: error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runGroveProcess(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	encodedArgs, err := json.Marshal(args)
	require.NoError(t, err)
	process := exec.Command(os.Args[0], "-test.run=^TestGroveCommandProcess$")
	process.Dir = dir
	process.Env = append(os.Environ(),
		"GROVE_GRAMMAR_COMMAND_PROCESS=1",
		"GROVE_GRAMMAR_ARGS="+string(encodedArgs),
	)
	var out, errOut bytes.Buffer
	process.Stdout = &out
	process.Stderr = &errOut

	err = process.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		exitCode = exitErr.ExitCode()
	default:
		require.NoError(t, err)
	}
	return out.String(), errOut.String(), exitCode
}

func runGrammarTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s: %s", strings.Join(command.Args, " "), output)
	return string(output)
}
