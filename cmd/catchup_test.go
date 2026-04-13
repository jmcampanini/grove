package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteCatchup(t *testing.T) {
	tests := []struct {
		gitMock        *mockGit
		name           string
		wantErr        bool
		wantErrContain string
		wantOutput     string
	}{
		{
			name: "clean feature branch merges successfully",
			gitMock: &mockGit{
				getCurrentBranchFn:     func() (string, error) { return "feature/my-branch", nil },
				getRepoDefaultBranchFn: func(string) (string, error) { return "main", nil },
				fetchRemoteFn:          func(string) (string, error) { return "", nil },
				getWorktreeRootFn:      func() (string, error) { return "/workspace/wt-my-branch", nil },
				isWorktreeDirtyFn:      func(string) (bool, error) { return false, nil },
				mergeFn:                func(string) (string, error) { return "Updating abc1234..def5678\nFast-forward", nil },
			},
			wantOutput: "Updating abc1234..def5678\nFast-forward\nMerged origin/main into feature/my-branch\n",
		},
		{
			name: "detached HEAD returns error",
			gitMock: &mockGit{
				getCurrentBranchFn: func() (string, error) { return "HEAD", nil },
			},
			wantErr:        true,
			wantErrContain: "detached HEAD",
		},
		{
			name: "on root branch returns error",
			gitMock: &mockGit{
				getCurrentBranchFn:     func() (string, error) { return "main", nil },
				getRepoDefaultBranchFn: func(string) (string, error) { return "main", nil },
			},
			wantErr:        true,
			wantErrContain: "already on root branch",
		},
		{
			name: "root branch not configured returns error",
			gitMock: &mockGit{
				getCurrentBranchFn:     func() (string, error) { return "feature/x", nil },
				getRepoDefaultBranchFn: func(string) (string, error) { return "", nil },
			},
			wantErr:        true,
			wantErrContain: "could not determine root branch",
		},
		{
			name: "dirty worktree returns error",
			gitMock: &mockGit{
				getCurrentBranchFn:     func() (string, error) { return "feature/x", nil },
				getRepoDefaultBranchFn: func(string) (string, error) { return "main", nil },
				getWorktreeRootFn:      func() (string, error) { return "/workspace/wt-x", nil },
				isWorktreeDirtyFn:      func(string) (bool, error) { return true, nil },
			},
			wantErr:        true,
			wantErrContain: "uncommitted changes",
		},
		{
			name: "merge failure propagates error",
			gitMock: &mockGit{
				getCurrentBranchFn:     func() (string, error) { return "feature/x", nil },
				getRepoDefaultBranchFn: func(string) (string, error) { return "main", nil },
				fetchRemoteFn:          func(string) (string, error) { return "", nil },
				getWorktreeRootFn:      func() (string, error) { return "/workspace/wt-x", nil },
				isWorktreeDirtyFn:      func(string) (bool, error) { return false, nil },
				mergeFn:                func(string) (string, error) { return "", errors.New("merge conflict") },
			},
			wantErr:        true,
			wantErrContain: "failed to merge",
		},
		{
			name: "already up to date merge",
			gitMock: &mockGit{
				getCurrentBranchFn:     func() (string, error) { return "feature/x", nil },
				getRepoDefaultBranchFn: func(string) (string, error) { return "main", nil },
				fetchRemoteFn:          func(string) (string, error) { return "", nil },
				getWorktreeRootFn:      func() (string, error) { return "/workspace/wt-x", nil },
				isWorktreeDirtyFn:      func(string) (bool, error) { return false, nil },
				mergeFn:                func(string) (string, error) { return "Already up to date.", nil },
			},
			wantOutput: "Already up to date.\nMerged origin/main into feature/x\n",
		},
		{
			name: "develop as root branch",
			gitMock: &mockGit{
				getCurrentBranchFn:     func() (string, error) { return "feature/x", nil },
				getDefaultRemoteFn:     func(string) (string, error) { return "origin", nil },
				getRepoDefaultBranchFn: func(string) (string, error) { return "develop", nil },
				fetchRemoteFn:          func(string) (string, error) { return "", nil },
				getWorktreeRootFn:      func() (string, error) { return "/workspace/wt-x", nil },
				isWorktreeDirtyFn:      func(string) (bool, error) { return false, nil },
				mergeFn:                func(string) (string, error) { return "", nil },
			},
			wantOutput: "Merged origin/develop into feature/x\n",
		},
		{
			name: "merge is called with correct ref",
			gitMock: &mockGit{
				getCurrentBranchFn:     func() (string, error) { return "feature/x", nil },
				getRepoDefaultBranchFn: func(string) (string, error) { return "main", nil },
				fetchRemoteFn:          func(string) (string, error) { return "", nil },
				getWorktreeRootFn:      func() (string, error) { return "/workspace/wt-x", nil },
				isWorktreeDirtyFn:      func(string) (bool, error) { return false, nil },
				mergeFn: func(ref string) (string, error) {
					if ref != "origin/main" {
						return "", fmt.Errorf("unexpected merge ref: %s", ref)
					}
					return "", nil
				},
			},
			wantOutput: "Merged origin/main into feature/x\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			ctx := &catchupContext{gitClient: tt.gitMock}

			err := executeCatchup(&buf, ctx)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			if tt.wantOutput != "" {
				assert.Equal(t, tt.wantOutput, buf.String())
			}
		})
	}
}
