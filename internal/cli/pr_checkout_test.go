package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckoutPRWorktree(t *testing.T) {
	tests := []struct {
		name           string
		prInfo         github.PullRequest
		gitMock        *mockGit
		cfg            config.Config
		wantErr        bool
		wantErrContain string
		wantStdout     string
	}{
		{
			name: "existing worktree returns path",
			prInfo: github.PullRequest{
				BranchName: "feature/add-auth",
				Number:     123,
				State:      github.PRStateOpen,
				Title:      "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						createTestWorktree("/workspace/pr-feature-add-auth", "feature/add-auth"),
					}, nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/pr-feature-add-auth",
		},
		{
			name: "branch exists skips fetch",
			prInfo: github.PullRequest{
				BranchName: "feature/add-auth",
				Number:     123,
				State:      github.PRStateOpen,
				Title:      "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				branchExistsFn: func(_ string, _ bool) (bool, error) {
					return true, nil
				},
				fetchRemoteBranchFn: func(_, _, _ string) error {
					t.Error("FetchRemoteBranch should not be called when branch exists")
					return nil
				},
				createWorktreeForExistingBranchFn: func(_, _ string) error {
					return nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/pr-123-add-auth",
		},
		{
			name: "new worktree creation success",
			prInfo: github.PullRequest{
				BranchName: "feature/add-auth",
				Number:     123,
				State:      github.PRStateOpen,
				Title:      "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				branchExistsFn: func(_ string, _ bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
					assert.Equal(t, "origin", remote)
					assert.Equal(t, "refs/pull/123/head", remoteRef)
					assert.Equal(t, "feature/add-auth", localRef)
					return nil
				},
				createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
					assert.Equal(t, "feature/add-auth", branchName)
					assert.Equal(t, "/workspace/pr-123-add-auth", worktreeAbsPath)
					return nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/pr-123-add-auth",
		},
		{
			name: "merged PR with deleted branch uses PR ref",
			prInfo: github.PullRequest{
				BranchName: "feature/old-branch",
				Number:     789,
				State:      github.PRStateMerged,
				Title:      "Old merged PR",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				branchExistsFn: func(_ string, _ bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
					assert.Equal(t, "origin", remote)
					assert.Equal(t, "refs/pull/789/head", remoteRef)
					assert.Equal(t, "feature/old-branch", localRef)
					return nil
				},
				createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
					assert.Equal(t, "feature/old-branch", branchName)
					assert.Equal(t, "/workspace/pr-789-old-merged-pr", worktreeAbsPath)
					return nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/pr-789-old-merged-pr",
		},
		{
			name: "PR number template generates different branch name",
			prInfo: github.PullRequest{
				BranchName: "feature/test",
				Number:     456,
				State:      github.PRStateOpen,
				Title:      "Test PR",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				branchExistsFn: func(_ string, _ bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(_, remoteRef, localRef string) error {
					assert.Equal(t, "refs/pull/456/head", remoteRef)
					assert.Equal(t, "pr/456", localRef)
					return nil
				},
				createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
					assert.Equal(t, "pr/456", branchName)
					assert.Equal(t, "/workspace/pr-456-test-pr", worktreeAbsPath)
					return nil
				},
			},
			cfg: func() config.Config {
				cfg := defaultTestConfig()
				cfg.PullRequest.BranchTemplate = "pr/{{.Number}}"
				return cfg
			}(),
			wantErr:    false,
			wantStdout: "/workspace/pr-456-test-pr",
		},
		{
			name: "worktree generation error occurs before git operations",
			prInfo: github.PullRequest{
				BranchName: "feature/add-auth",
				Number:     123,
				State:      github.PRStateOpen,
				Title:      "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					t.Error("ListWorktrees should not be called after worktree generation fails")
					return nil, nil
				},
			},
			cfg: func() config.Config {
				cfg := defaultTestConfig()
				cfg.PullRequest.WorktreeTemplate = `{{if eq .Number 123}}{{index "" 1}}{{else}}ok{{end}}`
				return cfg
			}(),
			wantErr:        true,
			wantErrContain: "failed to generate worktree name",
		},
		{
			name: "fetch error",
			prInfo: github.PullRequest{
				BranchName: "feature/add-auth",
				Number:     123,
				State:      github.PRStateOpen,
				Title:      "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				branchExistsFn: func(_ string, _ bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(_, _, _ string) error {
					return assert.AnError
				},
			},
			cfg:            defaultTestConfig(),
			wantErr:        true,
			wantErrContain: "failed to fetch remote branch",
		},
		{
			name: "worktree creation error",
			prInfo: github.PullRequest{
				BranchName: "feature/add-auth",
				Number:     123,
				State:      github.PRStateOpen,
				Title:      "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				branchExistsFn: func(_ string, _ bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(_, _, _ string) error {
					return nil
				},
				createWorktreeForExistingBranchFn: func(_, _ string) error {
					return assert.AnError
				},
			},
			cfg:            defaultTestConfig(),
			wantErr:        true,
			wantErrContain: "failed to create worktree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer

			ctx := &prCheckoutContext{
				cfg:       tt.cfg,
				ghClient:  &mockGitHub{},
				gitClient: tt.gitMock,
				logger:    testLogger(),
			}

			err := checkoutPRWorktree(&stdout, ctx, tt.prInfo)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				require.NoError(t, err)
			}

			if tt.wantStdout != "" {
				assert.Contains(t, strings.TrimSpace(stdout.String()), tt.wantStdout)
			}
		})
	}
}

func TestCheckoutPRWorktreeDirectBranchMatch(t *testing.T) {
	var stdout bytes.Buffer

	gitMock := &mockGit{
		listWorktreesFn: func() ([]git.Worktree, error) {
			return []git.Worktree{
				createTestWorktree("/workspace/wt-feature-add-auth", "feature/add-auth"),
			}, nil
		},
	}

	ctx := &prCheckoutContext{
		cfg:       defaultTestConfig(),
		ghClient:  &mockGitHub{},
		gitClient: gitMock,
		logger:    testLogger(),
	}

	prInfo := github.PullRequest{
		BranchName: "feature/add-auth",
		Number:     123,
		State:      github.PRStateOpen,
		Title:      "Add auth",
	}

	err := checkoutPRWorktree(&stdout, ctx, prInfo)
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "/workspace/wt-feature-add-auth")
}
