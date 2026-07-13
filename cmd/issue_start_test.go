package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestLocalBranch(name string) git.LocalBranch {
	return git.NewLocalBranch(name, "", "", false, 0, 0, git.Commit{})
}

func TestStartIssueWorktree(t *testing.T) {
	tests := []struct {
		name           string
		issueInfo      github.Issue
		gitMock        *mockGit
		cfg            config.Config
		wantErr        bool
		wantErrContain string
		wantStdout     string
	}{
		{
			name: "existing branch is reused without fetching",
			issueInfo: github.Issue{
				Number: 123,
				State:  github.IssueStateOpen,
				Title:  "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return []git.LocalBranch{createTestLocalBranch("issue/123-add-auth")}, nil
				},
				fetchRemoteTrackingBranchFn: func(remoteName, branchName string) error {
					t.Error("FetchRemoteTrackingBranch should not be called when an issue branch exists")
					return nil
				},
				createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
					t.Error("CreateWorktreeForNewBranchFromRef should not be called when an issue branch exists")
					return nil
				},
				createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
					assert.Equal(t, "issue/123-add-auth", branchName)
					assert.Equal(t, "/workspace/issue-123-add-auth", worktreeAbsPath)
					return nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/issue-123-add-auth",
		},
		{
			name: "existing branch is reused even after issue title edit",
			issueInfo: github.Issue{
				Number: 123,
				State:  github.IssueStateOpen,
				Title:  "Completely renamed title",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return []git.LocalBranch{createTestLocalBranch("issue/123-add-auth")}, nil
				},
				createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
					assert.Equal(t, "issue/123-add-auth", branchName)
					assert.Equal(t, "/workspace/issue-123-add-auth", worktreeAbsPath)
					return nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/issue-123-add-auth",
		},
		{
			name: "new branch created from fetched remote primary",
			issueInfo: github.Issue{
				Number: 123,
				State:  github.IssueStateOpen,
				Title:  "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				fetchRemoteTrackingBranchFn: func(remoteName, branchName string) error {
					assert.Equal(t, "origin", remoteName)
					assert.Equal(t, "main", branchName)
					return nil
				},
				createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
					assert.Equal(t, "issue/123-add-auth", newBranchName)
					assert.Equal(t, "/workspace/issue-123-add-auth", worktreeAbsPath)
					assert.Equal(t, "origin/main", baseRef)
					return nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/issue-123-add-auth",
		},
		{
			name: "closed issue proceeds with worktree creation",
			issueInfo: github.Issue{
				Number:      123,
				State:       github.IssueStateClosed,
				StateReason: github.IssueStateReasonCompleted,
				Title:       "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/issue-123-add-auth",
		},
		{
			name: "long title is capped in branch and worktree names",
			issueInfo: github.Issue{
				Number: 7,
				State:  github.IssueStateOpen,
				Title:  "Fix login crash when the password field is empty (regression!!)",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
					assert.Equal(t, "issue/7-fix-login-crash-when-the-password-field", newBranchName)
					assert.Equal(t, "/workspace/issue-7-fix-login-crash-when-the-password-field", worktreeAbsPath)
					return nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/issue-7-fix-login-crash-when-the-password-field",
		},
		{
			name: "number-only template",
			issueInfo: github.Issue{
				Number: 456,
				State:  github.IssueStateOpen,
				Title:  "Anything",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
					assert.Equal(t, "issue/456", newBranchName)
					assert.Equal(t, "/workspace/issue-456", worktreeAbsPath)
					return nil
				},
			},
			cfg: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.Issue.BranchTemplate = "issue/{{.Number}}"
				return cfg
			}(),
			wantErr:    false,
			wantStdout: "/workspace/issue-456",
		},
		{
			name: "remote default branch unresolved",
			issueInfo: github.Issue{
				Number: 123,
				State:  github.IssueStateOpen,
				Title:  "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				getRemoteDefaultBranchFn: func(remoteName string) (string, error) {
					return "", nil
				},
			},
			cfg:            defaultTestConfig(),
			wantErr:        true,
			wantErrContain: "could not determine default branch",
		},
		{
			name: "list local branches error",
			issueInfo: github.Issue{
				Number: 123,
				State:  github.IssueStateOpen,
				Title:  "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return nil, assert.AnError
				},
			},
			cfg:            defaultTestConfig(),
			wantErr:        true,
			wantErrContain: "failed to list local branches",
		},
		{
			name: "worktree creation error",
			issueInfo: github.Issue{
				Number: 123,
				State:  github.IssueStateOpen,
				Title:  "Add auth",
			},
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
					return assert.AnError
				},
			},
			cfg:            defaultTestConfig(),
			wantErr:        true,
			wantErrContain: "failed to create branch and worktree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer

			ctx := &issueStartContext{
				cfg:       tt.cfg,
				ghClient:  &mockGitHub{},
				gitClient: tt.gitMock,
			}

			err := startIssueWorktree(&stdout, ctx, tt.issueInfo)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			// stdout must carry the worktree path and nothing else: shells
			// and scripts consume it directly.
			assert.Equal(t, tt.wantStdout, strings.TrimSpace(stdout.String()))
		})
	}
}

func TestStartIssueWorktree_ExistingWorktree(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{name: "returns existing path", title: "Add auth"},
		{name: "found even after title edit", title: "Completely renamed title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wtPath := filepath.Join(t.TempDir(), "issue-123-add-auth")
			require.NoError(t, os.MkdirAll(wtPath, 0o755))

			gitMock := &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{createTestWorktree(wtPath, "issue/123-add-auth")}, nil
				},
				createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
					t.Error("CreateWorktreeForNewBranchFromRef should not be called when a worktree exists")
					return nil
				},
				createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
					t.Error("CreateWorktreeForExistingBranch should not be called when a worktree exists")
					return nil
				},
			}

			var stdout bytes.Buffer
			ctx := &issueStartContext{cfg: defaultTestConfig(), ghClient: &mockGitHub{}, gitClient: gitMock}

			err := startIssueWorktree(&stdout, ctx, github.Issue{Number: 123, State: github.IssueStateOpen, Title: tt.title})

			require.NoError(t, err)
			assert.Equal(t, wtPath, strings.TrimSpace(stdout.String()))
		})
	}
}

func TestStartIssueWorktree_StaleWorktreeEntry(t *testing.T) {
	stalePath := filepath.Join(t.TempDir(), "issue-123-add-auth") // registered but never created on disk

	pruned := false
	gitMock := &mockGit{
		listWorktreesFn: func() ([]git.Worktree, error) {
			return []git.Worktree{createTestWorktree(stalePath, "issue/123-add-auth")}, nil
		},
		pruneWorktreesFn: func() error {
			pruned = true
			return nil
		},
		createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
			assert.Equal(t, "issue/123-add-auth", newBranchName)
			assert.Equal(t, "/workspace/issue-123-add-auth", worktreeAbsPath)
			return nil
		},
	}

	var stdout bytes.Buffer
	ctx := &issueStartContext{cfg: defaultTestConfig(), ghClient: &mockGitHub{}, gitClient: gitMock}

	err := startIssueWorktree(&stdout, ctx, github.Issue{Number: 123, State: github.IssueStateOpen, Title: "Add auth"})

	require.NoError(t, err)
	assert.True(t, pruned, "stale worktree entries should be pruned")
	assert.Equal(t, "/workspace/issue-123-add-auth", strings.TrimSpace(stdout.String()))
}

func TestStartIssueWorktree_WorktreePathCollision(t *testing.T) {
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "issue-123-add-auth"), 0o755))

	gitMock := &mockGit{
		listWorktreesFn: func() ([]git.Worktree, error) {
			return []git.Worktree{}, nil
		},
		getWorkspacePathFn: func() (string, error) {
			return workspace, nil
		},
		createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
			t.Error("CreateWorktreeForNewBranchFromRef should not be called on path collision")
			return nil
		},
	}

	var stdout bytes.Buffer
	ctx := &issueStartContext{cfg: defaultTestConfig(), ghClient: &mockGitHub{}, gitClient: gitMock}

	err := startIssueWorktree(&stdout, ctx, github.Issue{Number: 123, State: github.IssueStateOpen, Title: "Add auth"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
	assert.Empty(t, stdout.String())
}
