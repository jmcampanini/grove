package cli

import (
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
)

type mockGitHub struct {
	getIssueFn               func(issueNum int) (github.Issue, error)
	getPullRequestByBranchFn func(branchName string) (*github.PullRequest, error)
	getPullRequestFn         func(prNum int) (github.PullRequest, error)
	listIssuesFn             func(query github.IssueQuery, limit int) ([]github.Issue, error)
	listPullRequestsFn       func(query github.PRQuery, limit int) ([]github.PullRequest, error)
}

func (m *mockGitHub) GetIssue(issueNum int) (github.Issue, error) {
	if m.getIssueFn != nil {
		return m.getIssueFn(issueNum)
	}
	return github.Issue{}, nil
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

func (m *mockGitHub) ListIssues(query github.IssueQuery, limit int) ([]github.Issue, error) {
	if m.listIssuesFn != nil {
		return m.listIssuesFn(query, limit)
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
	createWorktreeForExistingBranchFn   func(branchName, worktreeAbsPath string) error
	createWorktreeForNewBranchFn        func(newBranchName, worktreeAbsPath string) error
	createWorktreeForNewBranchFromRefFn func(newBranchName, worktreeAbsPath, baseRef string) error
	deleteBranchFn                      func(name string, force bool) error
	fetchRemoteBranchFn                 func(remote, remoteRef, localRef string) error
	fetchRemoteFn                       func(remoteName string) (string, error)
	fetchRemoteTrackingBranchFn         func(remoteName, branchName string) error
	getCurrentBranchFn                  func() (string, error)
	getCommitSubjectFn                  func() (string, error)
	getDefaultRemoteFn                  func(fallback string) (string, error)
	getMainWorktreePathFn               func() (string, error)
	getRemoteDefaultBranchFn            func(remoteName string) (string, error)
	getRepoDefaultBranchFn              func(remoteName string) (string, error)
	getStatusFn                         func(absPath string) (string, error)
	getRemoteURLFn                      func(remoteName string) (string, error)
	getWorktreeRootFn                   func() (string, error)
	isWorktreeDirtyFn                   func(absPath string) (bool, error)
	listLocalBranchesFn                 func() ([]git.LocalBranch, error)
	listRemoteBranchesFn                func(remoteName string) ([]git.RemoteBranch, error)
	listRemotesFn                       func() ([]string, error)
	listTagsFn                          func() ([]git.Tag, error)
	listWorktreesFn                     func() ([]git.Worktree, error)
	mergeFn                             func(ref string) (string, error)
	pruneWorktreesFn                    func() error
	refExistsFn                         func(ref string) (bool, error)
	removeWorktreeFn                    func(absPath string, force bool) error
	resetHardFn                         func(ref string) error
	syncTagsFn                          func(remoteName string) error
}

func (m *mockGit) BranchExists(branchName string, caseInsensitive bool) (bool, error) {
	if m.branchExistsFn != nil {
		return m.branchExistsFn(branchName, caseInsensitive)
	}
	return false, nil
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

func (m *mockGit) FetchRemoteTrackingBranch(remoteName, branchName string) error {
	if m.fetchRemoteTrackingBranchFn != nil {
		return m.fetchRemoteTrackingBranchFn(remoteName, branchName)
	}
	return nil
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

func (m *mockGit) GetRemoteDefaultBranch(remoteName string) (string, error) {
	if m.getRemoteDefaultBranchFn != nil {
		return m.getRemoteDefaultBranchFn(remoteName)
	}
	return "main", nil
}

func (m *mockGit) GetRepoDefaultBranch(remoteName string) (string, error) {
	if m.getRepoDefaultBranchFn != nil {
		return m.getRepoDefaultBranchFn(remoteName)
	}
	return "main", nil
}

func (m *mockGit) GetStatus(absPath string) (string, error) {
	if m.getStatusFn != nil {
		return m.getStatusFn(absPath)
	}
	return "", nil
}

func (m *mockGit) GetRemoteURL(remoteName string) (string, error) {
	if m.getRemoteURLFn != nil {
		return m.getRemoteURLFn(remoteName)
	}
	return "git@github.com:acme/app.git", nil
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

func (m *mockGit) Merge(ref string) (string, error) {
	if m.mergeFn != nil {
		return m.mergeFn(ref)
	}
	return "", nil
}

func (m *mockGit) PruneWorktrees() error {
	if m.pruneWorktreesFn != nil {
		return m.pruneWorktreesFn()
	}
	return nil
}

func (m *mockGit) RefExists(ref string) (bool, error) {
	if m.refExistsFn != nil {
		return m.refExistsFn(ref)
	}
	return true, nil
}

func (m *mockGit) RemoveWorktree(absPath string, force bool) error {
	if m.removeWorktreeFn != nil {
		return m.removeWorktreeFn(absPath, force)
	}
	return nil
}

func (m *mockGit) ResetHard(ref string) error {
	if m.resetHardFn != nil {
		return m.resetHardFn(ref)
	}
	return nil
}

func (m *mockGit) SyncTags(remoteName string) error {
	if m.syncTagsFn != nil {
		return m.syncTagsFn(remoteName)
	}
	return nil
}
