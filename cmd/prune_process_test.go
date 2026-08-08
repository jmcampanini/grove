package cmd

import (
	"bytes"
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

func TestPruneUsesUncachedGitHubState(t *testing.T) {
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

	runPruneTestGit(t, mainDir, "init", "-q", "-b", "main")
	runPruneTestGit(t, mainDir, "config", "user.email", "test@example.com")
	runPruneTestGit(t, mainDir, "config", "user.name", "Test")
	runPruneTestGit(t, mainDir, "commit", "--allow-empty", "-qm", "init")
	runPruneTestGit(t, mainDir, "worktree", "add", "-qb", "feature/test", worktreeDir)
	runPruneTestGit(t, mainDir, "remote", "add", "origin", "https://example.invalid/test.git")
	head := strings.TrimSpace(runPruneTestGit(t, mainDir, "rev-parse", "feature/test"))
	runPruneTestGit(t, mainDir, "update-ref", "refs/remotes/origin/feature/test", head)
	runPruneTestGit(t, mainDir, "branch", "--set-upstream-to=origin/feature/test", "feature/test")

	statePath := filepath.Join(testDir, "gh-state")
	logPath := filepath.Join(testDir, "gh.log")
	t.Setenv("FAKE_GH_LOG", logPath)
	t.Setenv("FAKE_GH_STATE", statePath)
	require.NoError(t, os.WriteFile(filepath.Join(fakeBinDir, "gh"), []byte(`#!/bin/sh
set -eu
branch=""
previous=""
for argument in "$@"; do
  if [ "$previous" = "--head" ]; then
    branch=$argument
  fi
  previous=$argument
done
if [ "$branch" != "feature/test" ]; then
  printf '[]\n'
  exit 0
fi
printf '%s\n' "$branch" >>"$FAKE_GH_LOG"
state=$(cat "$FAKE_GH_STATE")
printf '[{"headRefName":"feature/test","isDraft":false,"number":42,"state":"%s","title":"Test PR"}]\n' "$state"
`), 0o755))

	require.NoError(t, os.WriteFile(statePath, []byte("OPEN\n"), 0o600))
	statusOutput := runPruneCommandProcess(t, mainDir, "status", "")
	require.Contains(t, statusOutput, "#42 open")

	require.NoError(t, os.WriteFile(statePath, []byte("MERGED\n"), 0o600))
	pruneOutput := runPruneCommandProcess(t, mainDir, "prune", "0\n")
	assert.Contains(t, pruneOutput, "PR #42 merged")
	assert.NotContains(t, pruneOutput, "Nothing to prune.")

	logData, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(string(logData), "feature/test"))
}

func TestPruneCommandProcess(t *testing.T) {
	if os.Getenv("GROVE_PRUNE_COMMAND_PROCESS") != "1" {
		return
	}

	err := executeWithFileLogging(os.Stdin, os.Stdout, os.Stderr, []string{os.Getenv("GROVE_PRUNE_COMMAND")})
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runPruneCommandProcess(t *testing.T, dir, command, input string) string {
	t.Helper()

	process := exec.Command(os.Args[0], "-test.run=^TestPruneCommandProcess$")
	process.Dir = dir
	process.Env = append(os.Environ(),
		"GROVE_PRUNE_COMMAND_PROCESS=1",
		"GROVE_PRUNE_COMMAND="+command,
	)
	process.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	process.Stdout = &stdout
	process.Stderr = &stderr
	require.NoError(t, process.Run(), stderr.String())
	return stdout.String() + stderr.String()
}

func runPruneTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s: %s", strings.Join(command.Args, " "), output)
	return string(output)
}
