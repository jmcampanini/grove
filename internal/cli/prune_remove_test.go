package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
