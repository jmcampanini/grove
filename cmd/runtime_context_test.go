package cmd

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	initGitRepoWithBranch(t, dir, "main")
}

func initGitRepoWithBranch(t *testing.T, dir, branch string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-b", branch},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--no-gpg-sign", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, out)
	}
}
