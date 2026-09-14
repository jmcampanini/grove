package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupResolveRepo creates a primary clone at root/main and a linked worktree
// at root/elsewhere/wt-feature, mirroring a worktree placed under a separate
// worktree root.
func setupResolveRepo(t *testing.T, root string) {
	t.Helper()

	primaryDir := filepath.Join(root, "main")
	require.NoError(t, os.Mkdir(primaryDir, 0755))
	initGitRepo(t, primaryDir)

	featureDir := filepath.Join(root, "elsewhere", "wt-feature")
	cmd := exec.Command("git", "worktree", "add", "-b", "feature", featureDir)
	cmd.Dir = primaryDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git worktree add failed: %s", out)
}

func TestExecuteResolve(t *testing.T) {
	timeout := 5 * time.Second

	tests := []struct {
		name       string
		setup      func(t *testing.T) string // returns the target path
		wantErr    string
		wantSuffix string
	}{
		{
			name: "primary worktree path returns itself",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				setupResolveRepo(t, root)
				return filepath.Join(root, "main")
			},
			wantSuffix: "/main",
		},
		{
			name: "linked worktree elsewhere on disk returns primary",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				setupResolveRepo(t, root)
				return filepath.Join(root, "elsewhere", "wt-feature")
			},
			wantSuffix: "/main",
		},
		{
			name: "parent directory of worktrees returns error",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				setupResolveRepo(t, root)
				return filepath.Join(root, "elsewhere")
			},
			wantErr: "is not inside a git worktree",
		},
		{
			name: "subdirectory within worktree resolves to primary",
			setup: func(t *testing.T) string {
				root := t.TempDir()
				setupResolveRepo(t, root)
				sub := filepath.Join(root, "main", "subdir")
				require.NoError(t, os.Mkdir(sub, 0755))
				return sub
			},
			wantSuffix: "/main",
		},
		{
			name: "non-existent path returns error",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nope")
			},
			wantErr: "path does not exist",
		},
		{
			name: "regular directory returns error",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: "is not inside a git worktree",
		},
		{
			name: "file path returns error",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				f := filepath.Join(dir, "somefile")
				require.NoError(t, os.WriteFile(f, []byte("hi"), 0644))
				return f
			},
			wantErr: "path is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targetPath := tt.setup(t)
			var buf bytes.Buffer

			ctx := &resolveContext{
				logger:  testLogger(),
				timeout: timeout,
			}

			err := executeResolve(&buf, targetPath, ctx)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			output := strings.TrimSpace(buf.String())
			assert.Truef(t, filepath.IsAbs(output), "expected absolute path, got %s", output)
			assert.Truef(t, strings.HasSuffix(output, tt.wantSuffix),
				"expected path ending with %q, got %q", tt.wantSuffix, output)
		})
	}
}

func TestResolveTargetPath(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "no args defaults to cwd",
			args: []string{},
		},
		{
			name: "absolute path returned as-is",
			args: []string{"/tmp"},
		},
		{
			name: "relative path resolved to absolute",
			args: []string{"relative/path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolveTargetPath(tt.args)
			require.NoError(t, err)
			assert.Truef(t, filepath.IsAbs(result), "expected absolute path, got %s", result)

			if len(tt.args) == 0 {
				cwd, err := os.Getwd()
				require.NoError(t, err)
				assert.Equal(t, cwd, result)
			}
		})
	}
}
