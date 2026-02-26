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

type mockGitHub struct {
	getPullRequestByBranchFn func(branchName string) (*github.PullRequest, error)
	getPullRequestFn         func(prNum int) (github.PullRequest, error)
	listPullRequestsFn       func(query github.PRQuery, limit int) ([]github.PullRequest, error)
}

func (m *mockGitHub) GetPullRequest(prNum int) (github.PullRequest, error) {
	if m.getPullRequestFn != nil {
		return m.getPullRequestFn(prNum)
	}
	return github.PullRequest{}, nil
}

func (m *mockGitHub) GetPullRequestActivity(_, _ string, _ int) ([]github.ReviewThread, []github.TimelineEvent, error) {
	return nil, nil, nil
}

func (m *mockGitHub) GetPullRequestByBranch(branchName string) (*github.PullRequest, error) {
	if m.getPullRequestByBranchFn != nil {
		return m.getPullRequestByBranchFn(branchName)
	}
	return nil, nil
}

func (m *mockGitHub) ListPullRequests(query github.PRQuery, limit int) ([]github.PullRequest, error) {
	if m.listPullRequestsFn != nil {
		return m.listPullRequestsFn(query, limit)
	}
	return nil, nil
}

type mockGit struct {
	branchExistsFn                      func(branchName string, caseInsensitive bool) (bool, error)
	commitAllFn                         func(worktreeAbsPath, message string) error
	createWorktreeForExistingBranchFn   func(branchName, worktreeAbsPath string) error
	createWorktreeForNewBranchFn        func(newBranchName, worktreeAbsPath string) error
	createWorktreeForNewBranchFromRefFn func(newBranchName, worktreeAbsPath, baseRef string) error
	deleteBranchFn                      func(name string, force bool) error
	fetchRefFn                          func(remote, ref string) error
	fetchRemoteBranchFn                 func(remote, remoteRef, localRef string) error
	fetchRemoteFn                       func(remoteName string) (string, error)
	getCommitParentCountFn              func(sha string) (int, error)
	getCommitSubjectFn                  func() (string, error)
	getCurrentBranchFn                  func() (string, error)
	getDefaultRemoteFn                  func(fallback string) (string, error)
	getDiffStatsFn                      func(base, head string) (int, int, int, error)
	getMainWorktreePathFn               func() (string, error)
	getRepoDefaultBranchFn              func(remoteName string) (string, error)
	getWorkspacePathFn                  func() (string, error)
	getWorktreeRootFn                   func() (string, error)
	isWorktreeDirtyFn                   func(absPath string) (bool, error)
	listLocalBranchesFn                 func() ([]git.LocalBranch, error)
	listRemoteBranchesFn                func(remoteName string) ([]git.RemoteBranch, error)
	listRemotesFn                       func() ([]string, error)
	listTagsFn                          func() ([]git.Tag, error)
	listWorktreesFn                     func() ([]git.Worktree, error)
	mergeSquashRefFn                    func(worktreeAbsPath, ref string) error
	pruneWorktreesFn                    func() error
	removeWorktreeFn                    func(absPath string, force bool) error
	syncTagsFn                          func(remoteName string) error
}

func (m *mockGit) BranchExists(branchName string, caseInsensitive bool) (bool, error) {
	if m.branchExistsFn != nil {
		return m.branchExistsFn(branchName, caseInsensitive)
	}
	return false, nil
}

func (m *mockGit) CommitAll(worktreeAbsPath, message string) error {
	if m.commitAllFn != nil {
		return m.commitAllFn(worktreeAbsPath, message)
	}
	return nil
}

func (m *mockGit) DeleteBranch(name string, force bool) error {
	if m.deleteBranchFn != nil {
		return m.deleteBranchFn(name, force)
	}
	return nil
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

func (m *mockGit) FetchRef(remote, ref string) error {
	if m.fetchRefFn != nil {
		return m.fetchRefFn(remote, ref)
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

func (m *mockGit) GetCommitParentCount(sha string) (int, error) {
	if m.getCommitParentCountFn != nil {
		return m.getCommitParentCountFn(sha)
	}
	return 1, nil
}

func (m *mockGit) GetCommitSubject() (string, error) {
	if m.getCommitSubjectFn != nil {
		return m.getCommitSubjectFn()
	}
	return "", nil
}

func (m *mockGit) GetCurrentBranch() (string, error) {
	if m.getCurrentBranchFn != nil {
		return m.getCurrentBranchFn()
	}
	return "main", nil
}

func (m *mockGit) GetDefaultRemote(fallback string) (string, error) {
	if m.getDefaultRemoteFn != nil {
		return m.getDefaultRemoteFn(fallback)
	}
	return fallback, nil
}

func (m *mockGit) GetDiffStats(base, head string) (int, int, int, error) {
	if m.getDiffStatsFn != nil {
		return m.getDiffStatsFn(base, head)
	}
	return 0, 0, 0, nil
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

func (m *mockGit) IsWorktreeDirty(absPath string) (bool, error) {
	if m.isWorktreeDirtyFn != nil {
		return m.isWorktreeDirtyFn(absPath)
	}
	return false, nil
}

func (m *mockGit) MergeSquashRef(worktreeAbsPath, ref string) error {
	if m.mergeSquashRefFn != nil {
		return m.mergeSquashRefFn(worktreeAbsPath, ref)
	}
	return nil
}

func (m *mockGit) PruneWorktrees() error {
	if m.pruneWorktreesFn != nil {
		return m.pruneWorktreesFn()
	}
	return nil
}

func (m *mockGit) RemoveWorktree(absPath string, force bool) error {
	if m.removeWorktreeFn != nil {
		return m.removeWorktreeFn(absPath, force)
	}
	return nil
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

func TestCheckoutPRWorktree(t *testing.T) {
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
				cfg.PullRequest.BranchTemplate = "pr/{{.Number}}"
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

			ctx := &prCheckoutContext{
				cfg:       tt.cfg,
				ghClient:  &mockGitHub{},
				gitClient: tt.gitMock,
			}

			err := checkoutPRWorktree(&stdout, &stderr, ctx, tt.prInfo)

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

func TestCheckoutPRWorktree_SquashReconstruction(t *testing.T) {
	var stdout, stderr bytes.Buffer

	gitMock := &mockGit{
		listWorktreesFn: func() ([]git.Worktree, error) {
			return []git.Worktree{}, nil
		},
		branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
			return false, nil
		},
		fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
			return assert.AnError
		},
		fetchRefFn: func(remote, ref string) error {
			assert.Equal(t, "origin", remote)
			assert.Equal(t, "abc123", ref)
			return nil
		},
		getCommitParentCountFn: func(sha string) (int, error) {
			return 1, nil
		},
		createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
			assert.Equal(t, "recreated-16-feature/fast-init", newBranchName)
			assert.Equal(t, "/workspace/pr-recreated-16-feature-fast-init", worktreeAbsPath)
			assert.Equal(t, "abc123^1", baseRef)
			return nil
		},
		mergeSquashRefFn: func(worktreeAbsPath, ref string) error {
			assert.Equal(t, "/workspace/pr-recreated-16-feature-fast-init", worktreeAbsPath)
			assert.Equal(t, "abc123", ref)
			return nil
		},
		commitAllFn: func(worktreeAbsPath, message string) error {
			assert.Equal(t, "/workspace/pr-recreated-16-feature-fast-init", worktreeAbsPath)
			assert.Equal(t, "PR #16: Fast init", message)
			return nil
		},
	}

	ctx := &prCheckoutContext{
		cfg:       defaultTestConfig(),
		ghClient:  &mockGitHub{},
		gitClient: gitMock,
	}

	prInfo := github.PullRequest{
		BranchName:     "feature/fast-init",
		MergeCommitSHA: "abc123",
		Number:         16,
		State:          github.PRStateMerged,
		Title:          "Fast init",
	}

	err := checkoutPRWorktree(&stdout, &stderr, ctx, prInfo)
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "/workspace/pr-recreated-16-feature-fast-init")
	assert.Contains(t, stderr.String(), "Branch deleted from remote, recreating from merge commit...")
}

func TestCheckoutPRWorktree_MergeCommitReconstruction(t *testing.T) {
	var stdout, stderr bytes.Buffer

	gitMock := &mockGit{
		listWorktreesFn: func() ([]git.Worktree, error) {
			return []git.Worktree{}, nil
		},
		branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
			return false, nil
		},
		fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
			return assert.AnError
		},
		fetchRefFn: func(remote, ref string) error {
			assert.Equal(t, "origin", remote)
			assert.Equal(t, "merge123", ref)
			return nil
		},
		getCommitParentCountFn: func(sha string) (int, error) {
			return 2, nil
		},
		createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
			assert.Equal(t, "recreated-42-feature/two-parents", newBranchName)
			assert.Equal(t, "/workspace/pr-recreated-42-feature-two-parents", worktreeAbsPath)
			assert.Equal(t, "merge123^1", baseRef)
			return nil
		},
		mergeSquashRefFn: func(worktreeAbsPath, ref string) error {
			assert.Equal(t, "/workspace/pr-recreated-42-feature-two-parents", worktreeAbsPath)
			assert.Equal(t, "merge123", ref)
			return nil
		},
		commitAllFn: func(worktreeAbsPath, message string) error {
			assert.Equal(t, "/workspace/pr-recreated-42-feature-two-parents", worktreeAbsPath)
			assert.Equal(t, "PR #42: Two parent merge", message)
			return nil
		},
	}

	ctx := &prCheckoutContext{
		cfg:       defaultTestConfig(),
		ghClient:  &mockGitHub{},
		gitClient: gitMock,
	}

	prInfo := github.PullRequest{
		BranchName:     "feature/two-parents",
		MergeCommitSHA: "merge123",
		Number:         42,
		State:          github.PRStateMerged,
		Title:          "Two parent merge",
	}

	err := checkoutPRWorktree(&stdout, &stderr, ctx, prInfo)
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "/workspace/pr-recreated-42-feature-two-parents")
	assert.Contains(t, stderr.String(), "Branch deleted from remote, recreating from merge commit...")
}

func TestCheckoutPRWorktree_AmbiguousReconstruction(t *testing.T) {
	tests := []struct {
		name       string
		prInfo     github.PullRequest
		diffStats  [3]int // files, adds, dels
		wantBranch string
		wantBase   string
		wantStdout string
	}{
		{
			name: "rebase detected via stats mismatch",
			prInfo: github.PullRequest{
				BranchName:     "feature/rebase-test",
				CommitCount:    3,
				FilesChanged:   3,
				LinesAdded:     9,
				LinesDeleted:   0,
				MergeCommitSHA: "rebase789",
				Number:         29,
				State:          github.PRStateMerged,
				Title:          "Rebase test",
			},
			diffStats:  [3]int{1, 3, 0},
			wantBranch: "recreated-29-feature/rebase-test",
			wantBase:   "rebase789~3",
			wantStdout: "/workspace/pr-recreated-29-feature-rebase-test",
		},
		{
			name: "multi-commit squash detected via stats match",
			prInfo: github.PullRequest{
				BranchName:     "feature/squash-multi",
				CommitCount:    3,
				FilesChanged:   3,
				LinesAdded:     9,
				LinesDeleted:   0,
				MergeCommitSHA: "squash456",
				Number:         30,
				State:          github.PRStateMerged,
				Title:          "Squash multi",
			},
			diffStats:  [3]int{3, 9, 0},
			wantBranch: "recreated-30-feature/squash-multi",
			wantBase:   "squash456^1",
			wantStdout: "/workspace/pr-recreated-30-feature-squash-multi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			gitMock := &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
					return assert.AnError
				},
				fetchRefFn: func(remote, ref string) error {
					return nil
				},
				getCommitParentCountFn: func(sha string) (int, error) {
					return 1, nil
				},
				getDiffStatsFn: func(base, head string) (int, int, int, error) {
					return tt.diffStats[0], tt.diffStats[1], tt.diffStats[2], nil
				},
				createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
					assert.Equal(t, tt.wantBranch, newBranchName)
					assert.Equal(t, tt.wantBase, baseRef)
					return nil
				},
				mergeSquashRefFn: func(worktreeAbsPath, ref string) error {
					return nil
				},
				commitAllFn: func(worktreeAbsPath, message string) error {
					return nil
				},
			}

			ctx := &prCheckoutContext{
				cfg:       defaultTestConfig(),
				ghClient:  &mockGitHub{},
				gitClient: gitMock,
			}

			err := checkoutPRWorktree(&stdout, &stderr, ctx, tt.prInfo)
			require.NoError(t, err)

			assert.Contains(t, stdout.String(), tt.wantStdout)
		})
	}
}

func TestDetectBaseRef(t *testing.T) {
	tests := []struct {
		name        string
		prInfo      github.PullRequest
		parentCount int
		diffFiles   int
		diffAdds    int
		diffDels    int
		wantRef     string
	}{
		{
			name: "merge commit (2 parents)",
			prInfo: github.PullRequest{
				MergeCommitSHA: "abc123",
				CommitCount:    3,
			},
			parentCount: 2,
			wantRef:     "abc123^1",
		},
		{
			name: "single commit squash",
			prInfo: github.PullRequest{
				MergeCommitSHA: "abc123",
				CommitCount:    1,
			},
			parentCount: 1,
			wantRef:     "abc123^1",
		},
		{
			name: "multi-commit squash (stats match)",
			prInfo: github.PullRequest{
				MergeCommitSHA: "abc123",
				CommitCount:    3,
				FilesChanged:   5,
				LinesAdded:     100,
				LinesDeleted:   20,
			},
			parentCount: 1,
			diffFiles:   5,
			diffAdds:    100,
			diffDels:    20,
			wantRef:     "abc123^1",
		},
		{
			name: "rebase merge (stats mismatch)",
			prInfo: github.PullRequest{
				MergeCommitSHA: "abc123",
				CommitCount:    3,
				FilesChanged:   5,
				LinesAdded:     100,
				LinesDeleted:   20,
			},
			parentCount: 1,
			diffFiles:   2,
			diffAdds:    30,
			diffDels:    5,
			wantRef:     "abc123~3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &prCheckoutContext{
				gitClient: &mockGit{
					getCommitParentCountFn: func(sha string) (int, error) {
						return tt.parentCount, nil
					},
					getDiffStatsFn: func(base, head string) (int, int, int, error) {
						return tt.diffFiles, tt.diffAdds, tt.diffDels, nil
					},
				},
			}

			ref, err := detectBaseRef(ctx, tt.prInfo)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRef, ref)
		})
	}
}

func TestCheckoutPRWorktree_FetchFailsNoReconstruction(t *testing.T) {
	tests := []struct {
		name   string
		cfg    config.Config
		prInfo github.PullRequest
	}{
		{
			name: "not merged",
			cfg:  defaultTestConfig(),
			prInfo: github.PullRequest{
				BranchName: "feature/test",
				Number:     99,
				State:      github.PRStateOpen,
				Title:      "Test PR",
			},
		},
		{
			name: "auto recreate disabled",
			cfg: func() config.Config {
				cfg := defaultTestConfig()
				cfg.PullRequest.AutoRecreate = false
				return cfg
			}(),
			prInfo: github.PullRequest{
				BranchName:     "feature/test",
				MergeCommitSHA: "abc123",
				Number:         99,
				State:          github.PRStateMerged,
				Title:          "Test PR",
			},
		},
		{
			name: "no merge commit SHA",
			cfg:  defaultTestConfig(),
			prInfo: github.PullRequest{
				BranchName: "feature/test",
				Number:     99,
				State:      github.PRStateMerged,
				Title:      "Test PR",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			gitMock := &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{}, nil
				},
				branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
					return false, nil
				},
				fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
					return assert.AnError
				},
			}

			ctx := &prCheckoutContext{
				cfg:       tt.cfg,
				ghClient:  &mockGitHub{},
				gitClient: gitMock,
			}

			err := checkoutPRWorktree(&stdout, &stderr, ctx, tt.prInfo)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to fetch remote branch")
		})
	}
}

func TestCheckoutPRWorktree_ReconstructionBranchAlreadyExists(t *testing.T) {
	var stdout, stderr bytes.Buffer

	var createdWorktreeForBranch string
	gitMock := &mockGit{
		listWorktreesFn: func() ([]git.Worktree, error) {
			return []git.Worktree{}, nil
		},
		branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
			if branchName == "recreated-16-feature/fast-init" {
				return true, nil
			}
			return false, nil
		},
		fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
			return assert.AnError
		},
		fetchRefFn: func(remote, ref string) error {
			return nil
		},
		getCommitParentCountFn: func(sha string) (int, error) {
			return 1, nil
		},
		createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
			createdWorktreeForBranch = branchName
			return nil
		},
	}

	ctx := &prCheckoutContext{
		cfg:       defaultTestConfig(),
		ghClient:  &mockGitHub{},
		gitClient: gitMock,
	}

	prInfo := github.PullRequest{
		BranchName:     "feature/fast-init",
		MergeCommitSHA: "abc123",
		Number:         16,
		State:          github.PRStateMerged,
		Title:          "Fast init",
	}

	err := checkoutPRWorktree(&stdout, &stderr, ctx, prInfo)
	require.NoError(t, err)

	assert.Equal(t, "recreated-16-feature/fast-init", createdWorktreeForBranch)
	assert.Contains(t, stdout.String(), "/workspace/pr-recreated-16-feature-fast-init")
}

func TestCheckoutPRWorktree_ExistingRecreatedWorktree(t *testing.T) {
	var stdout, stderr bytes.Buffer

	gitMock := &mockGit{
		listWorktreesFn: func() ([]git.Worktree, error) {
			return []git.Worktree{
				createTestWorktree("/workspace/pr-recreated-16-feature-fast-init", "recreated-16-feature/fast-init"),
			}, nil
		},
	}

	ctx := &prCheckoutContext{
		cfg:       defaultTestConfig(),
		ghClient:  &mockGitHub{},
		gitClient: gitMock,
	}

	prInfo := github.PullRequest{
		BranchName:     "feature/fast-init",
		MergeCommitSHA: "abc123",
		Number:         16,
		State:          github.PRStateMerged,
		Title:          "Fast init",
	}

	err := checkoutPRWorktree(&stdout, &stderr, ctx, prInfo)
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "/workspace/pr-recreated-16-feature-fast-init")
	assert.Contains(t, stderr.String(), "Worktree already exists")
}

func TestCheckoutPRWorktreeDirectBranchMatch(t *testing.T) {
	var stdout, stderr bytes.Buffer

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
	}

	prInfo := github.PullRequest{
		BranchName: "feature/add-auth",
		Number:     123,
		State:      github.PRStateOpen,
		Title:      "Add auth",
	}

	err := checkoutPRWorktree(&stdout, &stderr, ctx, prInfo)
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "/workspace/wt-feature-add-auth")
	assert.Contains(t, stderr.String(), "Worktree already exists")
}
