package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindPrunable(t *testing.T) {
	workspace := t.TempDir()
	for _, name := range []string{"main", "wt-merged", "wt-closed", "wt-open", "wt-gone", "wt-keep"} {
		require.NoError(t, os.Mkdir(filepath.Join(workspace, name), 0o755))
	}

	remoteBranches := map[string]bool{
		"origin/main":         true,
		"origin/feature/keep": true,
	}

	tests := []struct {
		name        string
		statuses    []worktreeStatus
		wantCount   int
		wantNames   []string
		wantReasons []string
	}{
		{
			name: "merged PR is prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-merged"),
					branchName: "feature/merged",
					pr:         &github.PullRequest{Number: 10, State: github.PRStateMerged},
				},
			},
			wantCount:   1,
			wantNames:   []string{"wt-merged"},
			wantReasons: []string{"PR #10 merged"},
		},
		{
			name: "closed PR is prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-closed"),
					branchName: "feature/closed",
					pr:         &github.PullRequest{Number: 20, State: github.PRStateClosed},
				},
			},
			wantCount:   1,
			wantNames:   []string{"wt-closed"},
			wantReasons: []string{"PR #20 closed"},
		},
		{
			name: "open PR is not prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-open"),
					branchName: "feature/open",
					pr:         &github.PullRequest{Number: 30, State: github.PRStateOpen},
				},
			},
			wantCount: 0,
		},
		{
			name: "upstream gone is prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-gone"),
					branchName: "feature/gone",
					kind:       "local",
					tracking:   trackingInfo{upstream: "origin/feature/gone"},
				},
			},
			wantCount:   1,
			wantNames:   []string{"wt-gone"},
			wantReasons: []string{"upstream gone"},
		},
		{
			name: "upstream exists is not prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-keep"),
					branchName: "feature/keep",
					kind:       "local",
					tracking:   trackingInfo{upstream: "origin/feature/keep"},
				},
			},
			wantCount: 0,
		},
		{
			name: "main worktree is never prunable",
			statuses: []worktreeStatus{
				{
					absPath:    filepath.Join(workspace, "main"),
					branchName: "main",
					isMain:     true,
					pr:         &github.PullRequest{Number: 1, State: github.PRStateMerged},
				},
			},
			wantCount: 0,
		},
		{
			name:      "no prunable worktrees",
			statuses:  []worktreeStatus{{absPath: filepath.Join(workspace, "main"), isMain: true}},
			wantCount: 0,
		},
		{
			name: "multiple prunable reasons",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-merged"),
					branchName: "feature/merged",
					pr:         &github.PullRequest{Number: 10, State: github.PRStateMerged},
				},
				{
					absPath:    filepath.Join(workspace, "wt-gone"),
					branchName: "feature/gone",
					kind:       "local",
					tracking:   trackingInfo{upstream: "origin/feature/gone"},
				},
			},
			wantCount:   2,
			wantNames:   []string{"wt-merged", "wt-gone"},
			wantReasons: []string{"PR #10 merged", "upstream gone"},
		},
		{
			name: "orphaned worktree is prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-orphaned"),
					branchName: "feature/orphaned",
				},
			},
			wantCount:   1,
			wantNames:   []string{"wt-orphaned"},
			wantReasons: []string{"orphaned"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findPrunable(tt.statuses, remoteBranches)

			assert.Len(t, result, tt.wantCount)

			if tt.wantNames != nil {
				for i, name := range tt.wantNames {
					assert.Equal(t, name, result[i].name, "name[%d]", i)
				}
			}

			if tt.wantReasons != nil {
				for i, reason := range tt.wantReasons {
					assert.Equal(t, reason, result[i].reason, "reason[%d]", i)
				}
			}
		})
	}
}

func TestPruneReason(t *testing.T) {
	wtDir := t.TempDir()

	remoteBranches := map[string]bool{
		"origin/main": true,
	}

	tests := []struct {
		name   string
		status worktreeStatus
		want   string
	}{
		{
			name: "merged PR",
			status: worktreeStatus{
				absPath: wtDir,
				pr:      &github.PullRequest{Number: 5, State: github.PRStateMerged},
			},
			want: "PR #5 merged",
		},
		{
			name: "closed PR",
			status: worktreeStatus{
				absPath: wtDir,
				pr:      &github.PullRequest{Number: 7, State: github.PRStateClosed},
			},
			want: "PR #7 closed",
		},
		{
			name: "open PR not prunable",
			status: worktreeStatus{
				absPath: wtDir,
				pr:      &github.PullRequest{Number: 8, State: github.PRStateOpen},
			},
			want: "",
		},
		{
			name: "upstream gone",
			status: worktreeStatus{
				absPath:  wtDir,
				tracking: trackingInfo{upstream: "origin/feature/gone"},
			},
			want: "upstream gone",
		},
		{
			name: "upstream exists",
			status: worktreeStatus{
				absPath:  wtDir,
				tracking: trackingInfo{upstream: "origin/main"},
			},
			want: "",
		},
		{
			name: "no tracking no PR",
			status: worktreeStatus{
				absPath: wtDir,
			},
			want: "",
		},
		{
			name: "orphaned directory",
			status: worktreeStatus{
				absPath: "/nonexistent/wt-orphaned",
			},
			want: "orphaned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pruneReason(tt.status, remoteBranches)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExecuteRemovals(t *testing.T) {
	tests := []struct {
		deleteBranchErr  error
		name             string
		prunables        []prunable
		pruneWorktreeErr error
		removeErr        error
		wantErr          bool
		wantErrContain   string
		wantOutput       string
	}{
		{
			name: "successful removal",
			prunables: []prunable{
				{branchName: "feature/test", name: "wt-test", path: "/workspace/wt-test"},
			},
			wantOutput: "Removed 1 worktree(s): wt-test\n",
		},
		{
			name: "multiple removals",
			prunables: []prunable{
				{branchName: "feature/a", name: "wt-a", path: "/workspace/wt-a"},
				{branchName: "feature/b", name: "wt-b", path: "/workspace/wt-b"},
			},
			wantOutput: "Removed 2 worktree(s): wt-a, wt-b\n",
		},
		{
			name: "removal error is reported",
			prunables: []prunable{
				{branchName: "feature/fail", name: "wt-fail", path: "/workspace/wt-fail"},
			},
			removeErr:      errors.New("permission denied"),
			wantErr:        true,
			wantErrContain: "some removals failed",
		},
		{
			name: "orphaned worktree uses prune path",
			prunables: []prunable{
				{branchName: "feature/orphan", name: "wt-orphan", path: "/workspace/wt-orphan", reason: "orphaned"},
			},
			wantOutput: "Removed 1 worktree(s): wt-orphan\n",
		},
		{
			name: "orphaned prune error is reported",
			prunables: []prunable{
				{branchName: "feature/orphan", name: "wt-orphan", path: "/workspace/wt-orphan", reason: "orphaned"},
			},
			pruneWorktreeErr: errors.New("prune failed"),
			wantErr:          true,
			wantErrContain:   "some removals failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockGit{
				deleteBranchFn:   func(_ string, _ bool) error { return tt.deleteBranchErr },
				pruneWorktreesFn: func() error { return tt.pruneWorktreeErr },
				removeWorktreeFn: func(_ string, _ bool) error { return tt.removeErr },
			}

			var buf bytes.Buffer
			err := executeRemovals(&buf, mock, tt.prunables)

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
