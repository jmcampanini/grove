package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmcampanini/grove/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testWorktreeWithBranch(path, branchName string) git.Worktree {
	commit := git.NewCommit("abc123", "test", time.Now(), "tester")
	branch := git.NewLocalBranch(branchName, "", path, true, 0, 0, commit)
	return git.Worktree{AbsolutePath: path, Ref: &branch}
}

func testWorktreeDetached(path string) git.Worktree {
	commit := git.NewCommit("abc123", "test", time.Now(), "tester")
	return git.Worktree{AbsolutePath: path, Ref: commit}
}

func TestExecuteRemove(t *testing.T) {
	tests := []struct {
		force          bool
		gitMock        *mockGit
		keepBranch     bool
		mainWorktree   string
		name           string
		target         string
		wantErr        bool
		wantErrContain string
		wantOutput     string
	}{
		{
			name:         "clean worktree removed with branch",
			target:       "wt-feature",
			force:        false,
			keepBranch:   false,
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						testWorktreeWithBranch("/workspace/main", "main"),
						testWorktreeWithBranch("/workspace/wt-feature", "feature/add-auth"),
					}, nil
				},
				isWorktreeDirtyFn: func(_ string) (bool, error) { return false, nil },
				removeWorktreeFn:  func(_ string, _ bool) error { return nil },
				deleteBranchFn:    func(_ string, _ bool) error { return nil },
				pruneWorktreesFn:  func() error { return nil },
			},
			wantOutput: "Removed worktree wt-feature and branch feature/add-auth\n",
		},
		{
			name:         "dirty worktree without force returns error",
			target:       "wt-feature",
			force:        false,
			keepBranch:   false,
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						testWorktreeWithBranch("/workspace/main", "main"),
						testWorktreeWithBranch("/workspace/wt-feature", "feature/add-auth"),
					}, nil
				},
				isWorktreeDirtyFn: func(_ string) (bool, error) { return true, nil },
			},
			wantErr:        true,
			wantErrContain: "uncommitted changes",
		},
		{
			name:         "dirty worktree with force succeeds",
			target:       "wt-feature",
			force:        true,
			keepBranch:   false,
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						testWorktreeWithBranch("/workspace/main", "main"),
						testWorktreeWithBranch("/workspace/wt-feature", "feature/add-auth"),
					}, nil
				},
				removeWorktreeFn: func(_ string, _ bool) error { return nil },
				deleteBranchFn:   func(_ string, _ bool) error { return nil },
				pruneWorktreesFn: func() error { return nil },
			},
			wantOutput: "Removed worktree wt-feature and branch feature/add-auth\n",
		},
		{
			name:         "main worktree returns error",
			target:       "main",
			force:        false,
			keepBranch:   false,
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						testWorktreeWithBranch("/workspace/main", "main"),
					}, nil
				},
			},
			wantErr:        true,
			wantErrContain: "cannot remove the main worktree",
		},
		{
			name:         "keep-branch preserves branch",
			target:       "wt-feature",
			force:        false,
			keepBranch:   true,
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						testWorktreeWithBranch("/workspace/main", "main"),
						testWorktreeWithBranch("/workspace/wt-feature", "feature/add-auth"),
					}, nil
				},
				isWorktreeDirtyFn: func(_ string) (bool, error) { return false, nil },
				removeWorktreeFn:  func(_ string, _ bool) error { return nil },
				pruneWorktreesFn:  func() error { return nil },
			},
			wantOutput: "Removed worktree wt-feature\n",
		},
		{
			name:         "target not found returns error",
			target:       "nonexistent",
			force:        false,
			keepBranch:   false,
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						testWorktreeWithBranch("/workspace/main", "main"),
					}, nil
				},
			},
			wantErr:        true,
			wantErrContain: "no worktree found",
		},
		{
			name:         "detached HEAD worktree removed without branch deletion",
			target:       "wt-detached",
			force:        false,
			keepBranch:   false,
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						testWorktreeWithBranch("/workspace/main", "main"),
						testWorktreeDetached("/workspace/wt-detached"),
					}, nil
				},
				isWorktreeDirtyFn: func(_ string) (bool, error) { return false, nil },
				removeWorktreeFn:  func(_ string, _ bool) error { return nil },
				pruneWorktreesFn:  func() error { return nil },
			},
			wantOutput: "Removed worktree wt-detached\n",
		},
		{
			name:         "resolved by branch name",
			target:       "feature/add-auth",
			force:        false,
			keepBranch:   false,
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						testWorktreeWithBranch("/workspace/main", "main"),
						testWorktreeWithBranch("/workspace/wt-feature", "feature/add-auth"),
					}, nil
				},
				isWorktreeDirtyFn: func(_ string) (bool, error) { return false, nil },
				removeWorktreeFn:  func(_ string, _ bool) error { return nil },
				deleteBranchFn:    func(_ string, _ bool) error { return nil },
				pruneWorktreesFn:  func() error { return nil },
			},
			wantOutput: "Removed worktree wt-feature and branch feature/add-auth\n",
		},
		{
			name:         "resolved by absolute path",
			target:       "/workspace/wt-feature",
			force:        false,
			keepBranch:   false,
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						testWorktreeWithBranch("/workspace/main", "main"),
						testWorktreeWithBranch("/workspace/wt-feature", "feature/add-auth"),
					}, nil
				},
				isWorktreeDirtyFn: func(_ string) (bool, error) { return false, nil },
				removeWorktreeFn:  func(_ string, _ bool) error { return nil },
				deleteBranchFn:    func(_ string, _ bool) error { return nil },
				pruneWorktreesFn:  func() error { return nil },
			},
			wantOutput: "Removed worktree wt-feature and branch feature/add-auth\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			ctx := &removeContext{
				cfg:              defaultTestConfig(),
				gitClient:        tt.gitMock,
				logger:           testLogger(),
				mainWorktreePath: tt.mainWorktree,
			}

			err := executeRemove(&buf, ctx, tt.target, tt.force, tt.keepBranch)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				require.NoError(t, err)
			}

			if tt.wantOutput != "" {
				assert.Equal(t, tt.wantOutput, buf.String())
			}
		})
	}
}

func TestExecuteRemove_BareNameUsesActiveSpace(t *testing.T) {
	var removed string
	gitMock := &mockGit{
		listWorktreesFn: func() ([]git.Worktree, error) {
			return []git.Worktree{
				testWorktreeWithBranch("/repo", "main"),
				testWorktreeWithBranch("/root/grove/github.com/acme/app/wt-shared", "feature/shared"),
				testWorktreeWithBranch("/root/claude/github.com/acme/app/wt-shared", "feature/shared-agent"),
			}, nil
		},
		isWorktreeDirtyFn: func(_ string) (bool, error) { return false, nil },
		removeWorktreeFn: func(path string, _ bool) error {
			removed = path
			return nil
		},
		deleteBranchFn:   func(_ string, _ bool) error { return nil },
		pruneWorktreesFn: func() error { return nil },
	}

	cfg := defaultTestConfig()
	cfg.Worktree.Root = "/root"
	cfg.Worktree.Layout = "{{.Space}}/{{.Host}}/{{.Owner}}/{{.Repo}}/{{.Name}}"
	cfg.Worktree.Space = "claude"

	var buf bytes.Buffer
	ctx := &removeContext{cfg: cfg, gitClient: gitMock, logger: testLogger(), mainWorktreePath: "/repo"}
	require.NoError(t, executeRemove(&buf, ctx, "wt-shared", false, false))

	assert.Equal(t, "/root/claude/github.com/acme/app/wt-shared", removed)
	assert.Equal(t, "Removed worktree wt-shared and branch feature/shared-agent\n", buf.String())
}

func TestExecuteRemove_LayoutFailureStillRemovesByPath(t *testing.T) {
	var removed string
	gitMock := &mockGit{
		listWorktreesFn: func() ([]git.Worktree, error) {
			return []git.Worktree{
				testWorktreeWithBranch("/repo", "main"),
				testWorktreeWithBranch("/old/wt-legacy", "feature/legacy"),
			}, nil
		},
		isWorktreeDirtyFn: func(_ string) (bool, error) { return false, nil },
		removeWorktreeFn: func(path string, _ bool) error {
			removed = path
			return nil
		},
		deleteBranchFn:   func(_ string, _ bool) error { return nil },
		pruneWorktreesFn: func() error { return nil },
	}

	cfg := defaultTestConfig()
	cfg.Worktree.Root = "$GROVE_TEST_UNSET_ROOT/.worktrees"

	var buf bytes.Buffer
	ctx := &removeContext{cfg: cfg, gitClient: gitMock, logger: testLogger(), mainWorktreePath: "/repo"}
	require.NoError(t, executeRemove(&buf, ctx, "wt-legacy", false, false))

	assert.Equal(t, "/old/wt-legacy", removed)
}

func TestResolveTarget_ComparesResolvedPaths(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	require.NoError(t, os.MkdirAll(filepath.Join(real, "wt-feature"), 0o755))
	link := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(real, link))

	resolvedReal, err := filepath.EvalSymlinks(real)
	require.NoError(t, err)
	worktrees := []git.Worktree{
		testWorktreeWithBranch(filepath.Join(resolvedReal, "wt-feature"), "feature/add-auth"),
		testWorktreeWithBranch(filepath.Join(resolvedReal, "wt-other"), "feature/other"),
	}

	t.Run("absolute path through a symlink", func(t *testing.T) {
		wt, err := resolveTarget(filepath.Join(link, "wt-feature"), worktrees, "")
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(resolvedReal, "wt-feature"), wt.AbsolutePath)
	})

	t.Run("space path through a symlink", func(t *testing.T) {
		wt, err := resolveTarget("wt-feature", worktrees, filepath.Join(link, "wt-feature"))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(resolvedReal, "wt-feature"), wt.AbsolutePath)
	})

	t.Run("missing space path falls through", func(t *testing.T) {
		wt, err := resolveTarget("wt-other", worktrees, filepath.Join(link, "wt-missing"))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(resolvedReal, "wt-other"), wt.AbsolutePath)
	})
}

func TestResolveTarget(t *testing.T) {
	worktrees := []git.Worktree{
		testWorktreeWithBranch("/workspace/main", "main"),
		testWorktreeWithBranch("/workspace/wt-feature", "feature/add-auth"),
		testWorktreeDetached("/workspace/wt-detached"),
		testWorktreeWithBranch("/root/grove/github.com/acme/app/wt-shared", "feature/shared"),
		testWorktreeWithBranch("/root/claude/github.com/acme/app/wt-shared", "feature/shared-agent"),
		testWorktreeWithBranch("/root/grove/github.com/acme/app/wt-branchy", "wt-branchy"),
		testWorktreeWithBranch("/root/claude/github.com/acme/app/wt-branchy", "feature/branchy"),
	}

	tests := []struct {
		name           string
		spacePath      string
		target         string
		wantErr        bool
		wantErrContain string
		wantPath       string
	}{
		{
			name:     "absolute path",
			target:   "/workspace/wt-feature",
			wantPath: "/workspace/wt-feature",
		},
		{
			name:     "directory name",
			target:   "wt-feature",
			wantPath: "/workspace/wt-feature",
		},
		{
			name:     "branch name",
			target:   "feature/add-auth",
			wantPath: "/workspace/wt-feature",
		},
		{
			name:     "directory name in a nested layout",
			target:   "wt-detached",
			wantPath: "/workspace/wt-detached",
		},
		{
			name:      "shared name resolves in the active space",
			target:    "wt-shared",
			spacePath: "/root/claude/github.com/acme/app/wt-shared",
			wantPath:  "/root/claude/github.com/acme/app/wt-shared",
		},
		{
			name:      "space path takes precedence over a unique name elsewhere",
			target:    "wt-feature",
			spacePath: "/workspace/wt-feature",
			wantPath:  "/workspace/wt-feature",
		},
		{
			name:      "space path with no worktree falls back to a unique name",
			target:    "wt-feature",
			spacePath: "/root/grove/github.com/acme/app/wt-feature",
			wantPath:  "/workspace/wt-feature",
		},
		{
			name:     "shared name matching a branch wins over ambiguity",
			target:   "wt-branchy",
			wantPath: "/root/grove/github.com/acme/app/wt-branchy",
		},
		{
			name:           "shared name outside the active space is ambiguous",
			target:         "wt-shared",
			spacePath:      "/root/pi/github.com/acme/app/wt-shared",
			wantErr:        true,
			wantErrContain: "/root/claude/github.com/acme/app/wt-shared",
		},
		{
			name:           "shared name without a space path is ambiguous",
			target:         "wt-shared",
			wantErr:        true,
			wantErrContain: "/root/grove/github.com/acme/app/wt-shared",
		},
		{
			name:           "not found",
			target:         "nonexistent",
			wantErr:        true,
			wantErrContain: "no worktree found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wt, err := resolveTarget(tt.target, worktrees, tt.spacePath)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContain)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPath, wt.AbsolutePath)
			}
		})
	}
}

func TestRemoveWorktreeAndBranch(t *testing.T) {
	tests := []struct {
		branchName      string
		deleteBranchErr error
		deleteCalled    bool
		name            string
		removeCalled    bool
		removeErr       error
		wantErr         bool
		wantErrContain  string
	}{
		{
			branchName:   "feature/test",
			deleteCalled: true,
			name:         "removes worktree and branch",
			removeCalled: true,
		},
		{
			name:         "no branch name skips branch deletion",
			removeCalled: true,
		},
		{
			branchName:     "feature/test",
			name:           "remove worktree error",
			removeCalled:   true,
			removeErr:      errors.New("remove failed"),
			wantErr:        true,
			wantErrContain: "failed to remove worktree",
		},
		{
			branchName:      "feature/test",
			deleteBranchErr: errors.New("error: branch 'feature/test' not found"),
			deleteCalled:    true,
			name:            "branch not found error is ignored",
			removeCalled:    true,
		},
		{
			branchName:      "feature/test",
			deleteBranchErr: errors.New("permission denied"),
			deleteCalled:    true,
			name:            "branch delete error propagates",
			removeCalled:    true,
			wantErr:         true,
			wantErrContain:  "failed to delete branch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deleteCalled, removeCalled bool

			mock := &mockGit{
				deleteBranchFn: func(_ string, _ bool) error {
					deleteCalled = true
					return tt.deleteBranchErr
				},
				removeWorktreeFn: func(_ string, _ bool) error {
					removeCalled = true
					return tt.removeErr
				},
			}

			err := removeWorktreeAndBranch(mock, "/workspace/wt-test", tt.branchName)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.removeCalled, removeCalled, "removeCalled")
			assert.Equal(t, tt.deleteCalled, deleteCalled, "deleteCalled")
		})
	}
}
