package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-b", "main"},
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

func TestResolveWorkspaceRoot(t *testing.T) {
	timeout := 5 * time.Second

	tests := []struct {
		name            string
		primaryBranches []string
		setup           func(t *testing.T, root string)
		wantErr         string
		wantSuffix      string // expected suffix of the returned path
	}{
		{
			name:            "main with .git directory",
			primaryBranches: []string{"main"},
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "main")
				require.NoError(t, os.Mkdir(dir, 0755))
				initGitRepo(t, dir)
			},
			wantSuffix: "/main",
		},
		{
			name:            "develop with .git file (linked worktree)",
			primaryBranches: []string{"develop"},
			setup: func(t *testing.T, root string) {
				primary := filepath.Join(root, "primary")
				require.NoError(t, os.Mkdir(primary, 0755))
				initGitRepo(t, primary)

				worktreePath := filepath.Join(root, "develop")
				cmd := exec.Command("git", "worktree", "add", "-b", "develop", worktreePath)
				cmd.Dir = primary
				out, err := cmd.CombinedOutput()
				require.NoError(t, err, "git worktree add failed: %s", out)
			},
			wantSuffix: "/develop",
		},
		{
			name:            "main without .git marker",
			primaryBranches: []string{"main"},
			setup: func(t *testing.T, root string) {
				require.NoError(t, os.Mkdir(filepath.Join(root, "main"), 0755))
			},
			wantErr: "no valid worktree found",
		},
		{
			name:            "no matching children",
			primaryBranches: []string{"main", "develop"},
			setup: func(t *testing.T, root string) {
				require.NoError(t, os.Mkdir(filepath.Join(root, "other"), 0755))
			},
			wantErr: "no valid worktree found",
		},
		{
			name:            "first match wins based on list order",
			primaryBranches: []string{"develop", "main"},
			setup: func(t *testing.T, root string) {
				for _, name := range []string{"develop", "main"} {
					dir := filepath.Join(root, name)
					require.NoError(t, os.Mkdir(dir, 0755))
					initGitRepo(t, dir)
				}
			},
			wantSuffix: "/develop",
		},
		{
			name:            "empty primaryBranches",
			primaryBranches: []string{},
			setup:           func(t *testing.T, root string) {},
			wantErr:         "no primary branches configured",
		},
		{
			name:            "stale .git marker skipped, second candidate valid",
			primaryBranches: []string{"stale", "main"},
			setup: func(t *testing.T, root string) {
				staleDir := filepath.Join(root, "stale")
				require.NoError(t, os.Mkdir(staleDir, 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(staleDir, ".git"),
					[]byte("gitdir: /nonexistent/path"),
					0644,
				))

				mainDir := filepath.Join(root, "main")
				require.NoError(t, os.Mkdir(mainDir, 0755))
				initGitRepo(t, mainDir)
			},
			wantSuffix: "/main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			result, err := resolveWorkspaceRoot(root, tt.primaryBranches, timeout)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Truef(t, filepath.IsAbs(result), "expected absolute path, got %s", result)
			assert.Truef(t, strings.HasSuffix(result, tt.wantSuffix),
				"expected path ending with %q, got %q", tt.wantSuffix, result)
		})
	}
}

func TestHasGitMarker(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string // returns the directory to test
		want  bool
	}{
		{
			name: ".git directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0755))
				return dir
			},
			want: true,
		},
		{
			name: ".git file",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, ".git"),
					[]byte("gitdir: ../primary/.git/worktrees/linked"),
					0644,
				))
				return dir
			},
			want: true,
		},
		{
			name: "no .git",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: false,
		},
		{
			name: "nonexistent directory",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "does-not-exist")
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			assert.Equal(t, tt.want, hasGitMarker(dir))
		})
	}
}
