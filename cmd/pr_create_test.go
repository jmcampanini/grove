package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGitHub implements github.GitHub for testing
type mockGitHub struct {
	getPullRequestFn      func(prNum int) (github.PullRequest, error)
	getPullRequestFilesFn func(prNum int) ([]github.PullRequestFile, error)
	listPullRequestsFn    func(query github.PRQuery, limit int) ([]github.PullRequest, error)
	validateFn            func() error
}

func (m *mockGitHub) GetPullRequest(prNum int) (github.PullRequest, error) {
	if m.getPullRequestFn != nil {
		return m.getPullRequestFn(prNum)
	}
	return github.PullRequest{}, nil
}

func (m *mockGitHub) GetPullRequestByBranch(branchName string) (*github.PullRequest, error) {
	return nil, nil
}

func (m *mockGitHub) GetPullRequestFiles(prNum int) ([]github.PullRequestFile, error) {
	if m.getPullRequestFilesFn != nil {
		return m.getPullRequestFilesFn(prNum)
	}
	return nil, nil
}

func (m *mockGitHub) ListPullRequests(query github.PRQuery, limit int) ([]github.PullRequest, error) {
	if m.listPullRequestsFn != nil {
		return m.listPullRequestsFn(query, limit)
	}
	return nil, nil
}

func (m *mockGitHub) Validate() error {
	if m.validateFn != nil {
		return m.validateFn()
	}
	return nil
}

// mockGit implements git.Git for testing
type mockGit struct {
	branchExistsFn                      func(branchName string, caseInsensitive bool) (bool, error)
	createWorktreeForExistingBranchFn   func(branchName, worktreeAbsPath string) error
	createWorktreeForNewBranchFn        func(newBranchName, worktreeAbsPath string) error
	createWorktreeForNewBranchFromRefFn func(newBranchName, worktreeAbsPath, baseRef string) error
	fetchRemoteBranchFn                 func(remote, remoteRef, localRef string) error
	fetchRemoteFn                       func(remoteName string) (string, error)
	getCurrentBranchFn                  func() (string, error)
	getCommitSubjectFn                  func() (string, error)
	getDefaultRemoteFn                  func(fallback string) (string, error)
	getMainWorktreePathFn               func() (string, error)
	getRepoDefaultBranchFn              func(remoteName string) (string, error)
	getWorkspacePathFn                  func() (string, error)
	getWorktreeRootFn                   func() (string, error)
	listLocalBranchesFn                 func() ([]git.LocalBranch, error)
	listRemoteBranchesFn                func(remoteName string) ([]git.RemoteBranch, error)
	listRemotesFn                       func() ([]string, error)
	listTagsFn                          func() ([]git.Tag, error)
	listWorktreesFn                     func() ([]git.Worktree, error)
	syncTagsFn                          func(remoteName string) error
}

func (m *mockGit) BranchExists(branchName string, caseInsensitive bool) (bool, error) {
	if m.branchExistsFn != nil {
		return m.branchExistsFn(branchName, caseInsensitive)
	}
	return false, nil
}

func (m *mockGit) CreateWorktreeForExistingBranch(branchName, worktreeAbsPath string) error {
	if m.createWorktreeForExistingBranchFn != nil {
		return m.createWorktreeForExistingBranchFn(branchName, worktreeAbsPath)
	}
	return nil
}

func (m *mockGit) CreateWorktreeForNewBranch(newBranchName, worktreeAbsPath string) error {
	if m.createWorktreeForNewBranchFn != nil {
		return m.createWorktreeForNewBranchFn(newBranchName, worktreeAbsPath)
	}
	return nil
}

func (m *mockGit) CreateWorktreeForNewBranchFromRef(newBranchName, worktreeAbsPath, baseRef string) error {
	if m.createWorktreeForNewBranchFromRefFn != nil {
		return m.createWorktreeForNewBranchFromRefFn(newBranchName, worktreeAbsPath, baseRef)
	}
	return nil
}

func (m *mockGit) FetchRemoteBranch(remote, remoteRef, localRef string) error {
	if m.fetchRemoteBranchFn != nil {
		return m.fetchRemoteBranchFn(remote, remoteRef, localRef)
	}
	return nil
}

func (m *mockGit) FetchRemote(remoteName string) (string, error) {
	if m.fetchRemoteFn != nil {
		return m.fetchRemoteFn(remoteName)
	}
	return "", nil
}

func (m *mockGit) GetCurrentBranch() (string, error) {
	if m.getCurrentBranchFn != nil {
		return m.getCurrentBranchFn()
	}
	return "main", nil
}

func (m *mockGit) GetCommitSubject() (string, error) {
	if m.getCommitSubjectFn != nil {
		return m.getCommitSubjectFn()
	}
	return "", nil
}

func (m *mockGit) GetDefaultRemote(fallback string) (string, error) {
	if m.getDefaultRemoteFn != nil {
		return m.getDefaultRemoteFn(fallback)
	}
	return fallback, nil
}

func (m *mockGit) GetMainWorktreePath() (string, error) {
	if m.getMainWorktreePathFn != nil {
		return m.getMainWorktreePathFn()
	}
	return "/workspace/main", nil
}

func (m *mockGit) GetRepoDefaultBranch(remoteName string) (string, error) {
	if m.getRepoDefaultBranchFn != nil {
		return m.getRepoDefaultBranchFn(remoteName)
	}
	return "main", nil
}

func (m *mockGit) GetWorkspacePath() (string, error) {
	if m.getWorkspacePathFn != nil {
		return m.getWorkspacePathFn()
	}
	return "/workspace", nil
}

func (m *mockGit) GetWorktreeRoot() (string, error) {
	if m.getWorktreeRootFn != nil {
		return m.getWorktreeRootFn()
	}
	return "/workspace/main", nil
}

func (m *mockGit) ListLocalBranches() ([]git.LocalBranch, error) {
	if m.listLocalBranchesFn != nil {
		return m.listLocalBranchesFn()
	}
	return nil, nil
}

func (m *mockGit) ListRemoteBranches(remoteName string) ([]git.RemoteBranch, error) {
	if m.listRemoteBranchesFn != nil {
		return m.listRemoteBranchesFn(remoteName)
	}
	return nil, nil
}

func (m *mockGit) ListRemotes() ([]string, error) {
	if m.listRemotesFn != nil {
		return m.listRemotesFn()
	}
	return []string{"origin"}, nil
}

func (m *mockGit) ListTags() ([]git.Tag, error) {
	if m.listTagsFn != nil {
		return m.listTagsFn()
	}
	return nil, nil
}

func (m *mockGit) ListWorktrees() ([]git.Worktree, error) {
	if m.listWorktreesFn != nil {
		return m.listWorktreesFn()
	}
	return nil, nil
}

func (m *mockGit) SyncTags(remoteName string) error {
	if m.syncTagsFn != nil {
		return m.syncTagsFn(remoteName)
	}
	return nil
}

func defaultTestConfig() config.Config {
	return config.DefaultConfig()
}

func createTestWorktree(path string, branchName string) git.Worktree {
	commit := git.NewCommit("abc123", "Test commit", time.Now(), "tester")
	branch := git.NewLocalBranch(branchName, "", path, true, 0, 0, commit)
	return git.Worktree{
		AbsolutePath: path,
		Ref:          branch,
	}
}

func TestCreatePRWorktree(t *testing.T) {
	tests := []struct {
		name           string
		prInfo         github.PullRequest
		gitMock        *mockGit
		cfg            config.Config
		wantErr        bool
		wantErrContain string
		wantStdout     string
		wantStderr     string
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
			wantStderr: "Worktree already exists",
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
				branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
					return true, nil
				},
				fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
					t.Error("FetchRemoteBranch should not be called when branch exists")
					return nil
				},
				createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
					return nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/pr-feature-add-auth",
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
				branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
					assert.Equal(t, "origin", remote)
					assert.Equal(t, "feature/add-auth", remoteRef)
					assert.Equal(t, "feature/add-auth", localRef)
					return nil
				},
				createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
					assert.Equal(t, "feature/add-auth", branchName)
					assert.Equal(t, "/workspace/pr-feature-add-auth", worktreeAbsPath)
					return nil
				},
			},
			cfg:        defaultTestConfig(),
			wantErr:    false,
			wantStdout: "/workspace/pr-feature-add-auth",
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
				branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
					assert.Equal(t, "feature/test", remoteRef)
					assert.Equal(t, "pr/456", localRef)
					return nil
				},
				createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
					assert.Equal(t, "pr/456", branchName)
					assert.Equal(t, "/workspace/pr-456", worktreeAbsPath)
					return nil
				},
			},
			cfg: func() config.Config {
				cfg := config.DefaultConfig()
				cfg.PR.BranchTemplate = "pr/{{.Number}}"
				return cfg
			}(),
			wantErr:    false,
			wantStdout: "/workspace/pr-456",
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
				branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
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
				branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
					return nil
				},
				createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
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
			var stdout, stderr bytes.Buffer

			ctx := &prCreateContext{
				cfg:       tt.cfg,
				ghClient:  &mockGitHub{},
				gitClient: tt.gitMock,
			}

			err := createPRWorktree(&stdout, &stderr, ctx, tt.prInfo)

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

			if tt.wantStderr != "" {
				assert.Contains(t, stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestCreatePRWorktreeDirectBranchMatch(t *testing.T) {
	var stdout, stderr bytes.Buffer

	gitMock := &mockGit{
		listWorktreesFn: func() ([]git.Worktree, error) {
			return []git.Worktree{
				createTestWorktree("/workspace/wt-feature-add-auth", "feature/add-auth"),
			}, nil
		},
	}

	ctx := &prCreateContext{
		cfg:       defaultTestConfig(),
		ghClient:  &mockGitHub{},
		gitClient: gitMock,
	}

	prInfo := github.PullRequest{
		BranchName: "feature/add-auth",
		Number:     123,
		State:      github.PRStateOpen,
		Title:      "Add auth",
	}

	err := createPRWorktree(&stdout, &stderr, ctx, prInfo)
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "/workspace/wt-feature-add-auth")
	assert.Contains(t, stderr.String(), "Worktree already exists")
}
