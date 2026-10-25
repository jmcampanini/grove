package cli

import (
	"bytes"
	"io"
	"strings"
	"time"

	"github.com/jmcampanini/grove/internal/config"

	"charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/spf13/cobra"
)

func testLogger() *log.Logger {
	return log.New(io.Discard)
}

func newTestRootCommand() *cobra.Command {
	return NewRootCommand(strings.NewReader(""), io.Discard, io.Discard)
}

// executeForTest runs a fresh command tree on buffered streams, the same way
// Execute wires the process streams.
func executeForTest(args ...string) (stdout, stderr string, err error) {
	var out, errOut bytes.Buffer
	root := NewRootCommand(strings.NewReader(""), &out, &errOut)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errOut.String(), err
}

func testWorktreeDetached(path string) git.Worktree {
	commit := git.NewCommit("abc123", "test", time.Now(), "tester")
	return git.Worktree{AbsolutePath: path, Ref: commit}
}

// defaultTestConfig returns the defaults with worktrees placed directly under
// /workspace, so mock-based tests can assert on <root>/<name> paths.
func defaultTestConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.Worktree.Root = "/workspace"
	cfg.Worktree.Layout = "{{.Name}}"
	return cfg
}

// testConfigRootedAt returns defaultTestConfig with worktrees placed directly
// under dir.
func testConfigRootedAt(dir string) config.Config {
	cfg := defaultTestConfig()
	cfg.Worktree.Root = dir
	return cfg
}

func createTestWorktree(path string, branchName string) git.Worktree {
	commit := git.NewCommit("abc123", "Test commit", time.Now(), "tester")
	branch := git.NewLocalBranch(branchName, "", path, true, 0, 0, commit)
	return git.Worktree{
		AbsolutePath: path,
		Ref:          branch,
	}
}
