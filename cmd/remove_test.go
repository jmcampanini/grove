package cmd

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/jmcampanini/grove-cli/internal/git"
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
				getWorkspacePathFn: func() (string, error) { return "/workspace", nil },
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
				getWorkspacePathFn: func() (string, error) { return "/workspace", nil },
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
				getWorkspacePathFn: func() (string, error) { return "/workspace", nil },
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
				getWorkspacePathFn: func() (string, error) { return "/workspace", nil },
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
				getWorkspacePathFn: func() (string, error) { return "/workspace", nil },
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
				getWorkspacePathFn: func() (string, error) { return "/workspace", nil },
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
				getWorkspacePathFn: func() (string, error) { return "/workspace", nil },
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
				getWorkspacePathFn: func() (string, error) { return "/workspace", nil },
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
				getWorkspacePathFn: func() (string, error) { return "/workspace", nil },
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
				gitClient:        tt.gitMock,
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

func TestResolveTarget(t *testing.T) {
	worktrees := []git.Worktree{
		testWorktreeWithBranch("/workspace/main", "main"),
		testWorktreeWithBranch("/workspace/wt-feature", "feature/add-auth"),
		testWorktreeDetached("/workspace/wt-detached"),
	}

	tests := []struct {
		name           string
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
			name:           "not found",
			target:         "nonexistent",
			wantErr:        true,
			wantErrContain: "no worktree found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wt, err := resolveTarget(tt.target, worktrees, "/workspace")

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
