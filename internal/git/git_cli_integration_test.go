package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// GetWorktreeRoot tests
// =============================================================================

func TestGetWorktreeRoot_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	root, err := repo.Git.GetWorktreeRoot()

	require.NoError(t, err)
	assert.Equal(t, repo.path(), root)
}

func TestGetWorktreeRoot_Integration_FromSubdirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	// Create a subdirectory and use a GitCli pointing to it
	subdir := filepath.Join(repo.path(), "subdir", "nested")
	require.NoError(t, os.MkdirAll(subdir, 0755))

	subdirGit := New(context.Background(), false, subdir, testTimeout, nil).(*GitCli)
	root, err := subdirGit.GetWorktreeRoot()

	require.NoError(t, err)
	assert.Equal(t, repo.path(), root)
}

func TestGetWorktreeRoot_Integration_OutsideRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Use temp dir that's not a git repo
	tmpDir := t.TempDir()
	outsideGit := New(context.Background(), false, tmpDir, testTimeout, nil).(*GitCli)

	root, err := outsideGit.GetWorktreeRoot()

	// Should return empty string, no error (not in a git repo is valid)
	require.NoError(t, err)
	assert.Empty(t, root)
}

// =============================================================================
// GetCurrentBranch tests
// =============================================================================

func TestGetCurrentBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	branch, err := repo.Git.GetCurrentBranch()

	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestGetCurrentBranch_Integration_OnFeatureBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature-x")
	repo.checkout("feature-x")

	branch, err := repo.Git.GetCurrentBranch()

	require.NoError(t, err)
	assert.Equal(t, "feature-x", branch)
}

func TestGetCurrentBranch_Integration_DetachedHEAD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	sha := repo.commit("initial commit")
	repo.checkoutDetached(sha)

	branch, err := repo.Git.GetCurrentBranch()

	require.NoError(t, err)
	assert.Equal(t, "HEAD", branch)
}

// =============================================================================
// GetCommitSubject tests
// =============================================================================

func TestGetCommitSubject_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("This is my commit message")

	subject, err := repo.Git.GetCommitSubject()

	require.NoError(t, err)
	assert.Equal(t, "This is my commit message", subject)
}

func TestGetCommitSubject_Integration_MultiLineMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	// Create a commit with multi-line message
	filename := filepath.Join(repo.path(), "file.txt")
	appendToFile(t, filename, "content\n")
	runGit(t, repo.path(), "add", "-A")
	runGit(t, repo.path(), "commit", "-m", "First line\n\nBody paragraph")

	subject, err := repo.Git.GetCommitSubject()

	require.NoError(t, err)
	assert.Equal(t, "First line", subject)
}

// =============================================================================
// GetMainWorktreePath tests
// =============================================================================

func TestGetMainWorktreePath_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	path, err := repo.Git.GetMainWorktreePath()

	require.NoError(t, err)
	// Compare resolved paths to handle symlinks (e.g., /var -> /private/var on macOS)
	assert.Equal(t, repo.path(), resolvePath(t, path))
}

func TestGetMainWorktreePath_Integration_FromLinkedWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature")

	// Create a linked worktree
	worktreePath := filepath.Join(t.TempDir(), "feature-worktree")
	repo.createWorktree(worktreePath, "feature")

	// Create GitCli pointing to the linked worktree
	linkedGit := New(context.Background(), false, worktreePath, testTimeout, nil).(*GitCli)

	mainPath, err := linkedGit.GetMainWorktreePath()

	require.NoError(t, err)
	// Compare resolved paths to handle symlinks (e.g., /var -> /private/var on macOS)
	assert.Equal(t, repo.path(), resolvePath(t, mainPath))
}

// =============================================================================
// GetWorkspacePath tests
// =============================================================================

func TestGetWorkspacePath_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	workspacePath, err := repo.Git.GetWorkspacePath()

	require.NoError(t, err)
	// Workspace path is the parent of the main worktree
	// Compare resolved paths to handle symlinks (e.g., /var -> /private/var on macOS)
	assert.Equal(t, filepath.Dir(repo.path()), resolvePath(t, workspacePath))
}

func TestGetWorkspacePath_Integration_FromLinkedWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature")

	// Create a linked worktree
	worktreePath := filepath.Join(t.TempDir(), "feature-worktree")
	repo.createWorktree(worktreePath, "feature")

	// Create GitCli pointing to the linked worktree
	linkedGit := New(context.Background(), false, worktreePath, testTimeout, nil).(*GitCli)

	workspacePath, err := linkedGit.GetWorkspacePath()

	require.NoError(t, err)
	// Workspace path should still be the parent of the main worktree
	// Compare resolved paths to handle symlinks (e.g., /var -> /private/var on macOS)
	assert.Equal(t, filepath.Dir(repo.path()), resolvePath(t, workspacePath))
}

func TestGetWorkspacePath_Integration_FromSubdirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	// Create a subdirectory and use a GitCli pointing to it
	subdir := filepath.Join(repo.path(), "subdir", "nested")
	require.NoError(t, os.MkdirAll(subdir, 0755))

	subdirGit := New(context.Background(), false, subdir, testTimeout, nil).(*GitCli)
	workspacePath, err := subdirGit.GetWorkspacePath()

	require.NoError(t, err)
	// Workspace path should be the parent of the main worktree
	// Compare resolved paths to handle symlinks (e.g., /var -> /private/var on macOS)
	assert.Equal(t, filepath.Dir(repo.path()), resolvePath(t, workspacePath))
}

// =============================================================================
// ListLocalBranches tests
// =============================================================================

func TestListLocalBranches_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature-a")
	repo.createBranch("feature-b")

	branches, err := repo.Git.ListLocalBranches()

	require.NoError(t, err)
	assert.Len(t, branches, 3)
	assert.ElementsMatch(t, []string{"main", "feature-a", "feature-b"}, branchNames(branches))
}

func TestListLocalBranches_Integration_EmptyRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Create repo without any commits
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	git := New(context.Background(), false, dir, testTimeout, nil).(*GitCli)
	branches, err := git.ListLocalBranches()

	require.NoError(t, err)
	assert.Empty(t, branches)
}

func TestListLocalBranches_Integration_WithTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")

	// Make a local commit to be ahead
	repo.commit("local commit")

	branches, err := repo.Git.ListLocalBranches()

	require.NoError(t, err)
	require.Len(t, branches, 1)

	main := branches[0]
	assert.Equal(t, "main", main.Name)
	assert.Equal(t, "origin/main", main.UpstreamName)
	assert.Equal(t, 1, main.Ahead)
	assert.Equal(t, 0, main.Behind)
}

func TestListLocalBranches_Integration_CheckedOutBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature")

	branches, err := repo.Git.ListLocalBranches()

	require.NoError(t, err)
	require.Len(t, branches, 2)

	var mainBranch, featureBranch LocalBranch
	for _, b := range branches {
		switch b.Name {
		case "main":
			mainBranch = b
		case "feature":
			featureBranch = b
		}
	}

	assert.True(t, mainBranch.IsCheckedOut)
	assert.False(t, featureBranch.IsCheckedOut)
}

func TestListLocalBranches_Integration_BranchWithSlash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature/my-feature")
	repo.createBranch("bugfix/issue-123")

	branches, err := repo.Git.ListLocalBranches()

	require.NoError(t, err)
	assert.Len(t, branches, 3)
	assert.ElementsMatch(t, []string{"main", "feature/my-feature", "bugfix/issue-123"}, branchNames(branches))
}

// =============================================================================
// ListRemotes tests
// =============================================================================

func TestListRemotes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")

	remotes, err := repo.Git.ListRemotes()

	require.NoError(t, err)
	assert.Equal(t, []string{"origin"}, remotes)
}

func TestListRemotes_Integration_NoRemotes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	remotes, err := repo.Git.ListRemotes()

	require.NoError(t, err)
	assert.Empty(t, remotes)
}

func TestListRemotes_Integration_MultipleRemotes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")

	// Add another remote manually
	upstreamDir := filepath.Join(t.TempDir(), "upstream.git")
	runGit(t, repo.path(), "clone", "--bare", repo.path(), upstreamDir)
	runGit(t, repo.path(), "remote", "add", "upstream", upstreamDir)

	remotes, err := repo.Git.ListRemotes()

	require.NoError(t, err)
	assert.Len(t, remotes, 2)
	assert.ElementsMatch(t, []string{"origin", "upstream"}, remotes)
}

// =============================================================================
// ListRemoteBranches tests
// =============================================================================

func TestListRemoteBranches_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")

	branches, err := repo.Git.ListRemoteBranches("origin")

	require.NoError(t, err)
	// Should have origin/main (origin/HEAD is filtered out)
	names := remoteBranchNames(branches)
	assert.Contains(t, names, "main")
}

// =============================================================================
// ListTags tests
// =============================================================================

func TestListTags_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createAnnotatedTag("v1.0.0", "Release 1.0.0")
	repo.createLightweightTag("v0.1.0")

	tags, err := repo.Git.ListTags()

	require.NoError(t, err)
	assert.Len(t, tags, 2)
	assert.ElementsMatch(t, []string{"v1.0.0", "v0.1.0"}, tagNames(tags))
}

func TestListTags_Integration_NoTags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	tags, err := repo.Git.ListTags()

	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestListTags_Integration_AnnotatedVsLightweight(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createAnnotatedTag("v1.0.0", "Annotated release")
	repo.createLightweightTag("v0.1.0")

	tags, err := repo.Git.ListTags()

	require.NoError(t, err)
	require.Len(t, tags, 2)

	var annotated, lightweight Tag
	for _, tag := range tags {
		switch tag.Name {
		case "v1.0.0":
			annotated = tag
		case "v0.1.0":
			lightweight = tag
		}
	}

	assert.True(t, annotated.IsAnnotated())
	assert.Equal(t, "Annotated release", annotated.Message)
	assert.Equal(t, "Test User", annotated.TaggerName)

	assert.False(t, lightweight.IsAnnotated())
	assert.Empty(t, lightweight.Message)
	assert.Empty(t, lightweight.TaggerName)
}

// =============================================================================
// BranchExists tests
// =============================================================================

func TestBranchExists_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature")

	exists, err := repo.Git.BranchExists("feature", false)
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.Git.BranchExists("nonexistent", false)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestBranchExists_Integration_CaseInsensitive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("Feature")

	// Case sensitive - should not match
	exists, err := repo.Git.BranchExists("feature", false)
	require.NoError(t, err)
	assert.False(t, exists)

	// Case insensitive - should match
	exists, err = repo.Git.BranchExists("feature", true)
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.Git.BranchExists("FEATURE", true)
	require.NoError(t, err)
	assert.True(t, exists)
}

// =============================================================================
// RefExists tests
// =============================================================================

func TestRefExists_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature")

	exists, err := repo.Git.RefExists("feature")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.Git.RefExists("HEAD")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.Git.RefExists("nonexistent-ref")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRefExists_Integration_RemoteTrackingRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")

	exists, err := repo.Git.RefExists("origin/main")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.Git.RefExists("origin/nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

// =============================================================================
// ListWorktrees tests
// =============================================================================

func TestListWorktrees_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	worktrees, err := repo.Git.ListWorktrees()

	require.NoError(t, err)
	assert.Len(t, worktrees, 1)
	assert.Equal(t, repo.path(), worktrees[0].AbsolutePath)
}

func TestListWorktrees_Integration_WithLinkedWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature-a")
	repo.createBranch("feature-b")

	worktreeA := filepath.Join(t.TempDir(), "worktree-a")
	worktreeB := filepath.Join(t.TempDir(), "worktree-b")
	repo.createWorktree(worktreeA, "feature-a")
	repo.createWorktree(worktreeB, "feature-b")

	worktrees, err := repo.Git.ListWorktrees()

	require.NoError(t, err)
	assert.Len(t, worktrees, 3)

	paths := worktreePaths(worktrees)
	assert.Contains(t, paths, repo.path())
	assert.Contains(t, paths, resolvePath(t, worktreeA))
	assert.Contains(t, paths, resolvePath(t, worktreeB))
}

func TestListWorktrees_Integration_DetachedHEAD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	sha := repo.commit("initial commit")
	repo.createBranch("feature")

	// Create a worktree and then detach it
	worktreePath := filepath.Join(t.TempDir(), "detached-worktree")
	runGit(t, repo.path(), "worktree", "add", "--detach", worktreePath, sha)

	worktrees, err := repo.Git.ListWorktrees()

	require.NoError(t, err)
	assert.Len(t, worktrees, 2)

	// Find the detached worktree (paths may have symlinks resolved differently)
	resolvedWorktreePath := resolvePath(t, worktreePath)
	var detachedWorktree *Worktree
	for i, wt := range worktrees {
		if wt.AbsolutePath == resolvedWorktreePath {
			detachedWorktree = &worktrees[i]
			break
		}
	}

	require.NotNil(t, detachedWorktree, "could not find detached worktree at %s", resolvedWorktreePath)
	assert.NotNil(t, detachedWorktree.Ref)
	// Should be a Commit since it's detached
	assert.Equal(t, WorktreeRefTypeCommit, detachedWorktree.Ref.Type())
}

func TestListWorktrees_Integration_DetachedHEAD_WithAnnotatedTag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createAnnotatedTag("v1.0.0", "Release")

	worktreePath := filepath.Join(t.TempDir(), "tag-worktree")
	runGit(t, repo.path(), "worktree", "add", "--detach", worktreePath, "v1.0.0")

	worktrees, err := repo.Git.ListWorktrees()

	require.NoError(t, err)
	assert.Len(t, worktrees, 2)

	resolvedPath := resolvePath(t, worktreePath)
	var tagWorktree *Worktree
	for i, wt := range worktrees {
		if wt.AbsolutePath == resolvedPath {
			tagWorktree = &worktrees[i]
			break
		}
	}

	require.NotNil(t, tagWorktree, "could not find tag worktree at %s", resolvedPath)
	require.NotNil(t, tagWorktree.Ref)
	assert.Equal(t, WorktreeRefTypeTag, tagWorktree.Ref.Type())
	tag, ok := tagWorktree.Ref.FullTag()
	require.True(t, ok)
	require.NotNil(t, tag)
	assert.Equal(t, "v1.0.0", tag.Name)
}

func TestListWorktrees_Integration_DetachedHEAD_WithLightweightTag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createLightweightTag("v0.1.0")

	worktreePath := filepath.Join(t.TempDir(), "lightweight-tag-worktree")
	runGit(t, repo.path(), "worktree", "add", "--detach", worktreePath, "v0.1.0")

	worktrees, err := repo.Git.ListWorktrees()

	require.NoError(t, err)
	assert.Len(t, worktrees, 2)

	resolvedPath := resolvePath(t, worktreePath)
	var tagWorktree *Worktree
	for i, wt := range worktrees {
		if wt.AbsolutePath == resolvedPath {
			tagWorktree = &worktrees[i]
			break
		}
	}

	require.NotNil(t, tagWorktree, "could not find tag worktree at %s", resolvedPath)
	require.NotNil(t, tagWorktree.Ref)
	assert.Equal(t, WorktreeRefTypeTag, tagWorktree.Ref.Type())
	tag, ok := tagWorktree.Ref.FullTag()
	require.True(t, ok)
	require.NotNil(t, tag)
	assert.Equal(t, "v0.1.0", tag.Name)
}

// =============================================================================
// GetDefaultRemote tests
// =============================================================================

func TestGetDefaultRemote_Integration_WithPushDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")
	repo.setConfig("remote.pushDefault", "origin")

	remote, err := repo.Git.GetDefaultRemote("fallback")

	require.NoError(t, err)
	assert.Equal(t, "origin", remote)
}

func TestGetDefaultRemote_Integration_Fallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	remote, err := repo.Git.GetDefaultRemote("my-fallback")

	require.NoError(t, err)
	assert.Equal(t, "my-fallback", remote)
}

// =============================================================================
// GetRepoDefaultBranch tests
// =============================================================================

func TestGetRepoDefaultBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")

	// Set the remote HEAD
	runGit(t, repo.path(), "remote", "set-head", "origin", "main")

	branch, err := repo.Git.GetRepoDefaultBranch("origin")

	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestGetRepoDefaultBranch_Integration_RemoteExistsWithoutHead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")

	// Remote exists but HEAD is not set - should return empty string, not error
	// (remote HEAD is only set after fetch or explicit set-head)
	runGit(t, repo.path(), "remote", "set-head", "origin", "-d")

	branch, err := repo.Git.GetRepoDefaultBranch("origin")

	require.NoError(t, err)
	assert.Empty(t, branch)
}

func TestGetRepoDefaultBranch_Integration_RemoteDoesNotExist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	// No remote configured - should return an error
	branch, err := repo.Git.GetRepoDefaultBranch("nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	assert.Empty(t, branch)
}

// =============================================================================
// GetRemoteDefaultBranch tests
// =============================================================================

func TestGetRemoteDefaultBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")

	branch, err := repo.Git.GetRemoteDefaultBranch("origin")

	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestGetRemoteDefaultBranch_Integration_MissingLocalRemoteHead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")
	runGit(t, repo.path(), "remote", "set-head", "origin", "-d")

	branch, err := repo.Git.GetRemoteDefaultBranch("origin")

	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestGetRemoteDefaultBranch_Integration_StaleLocalRemoteHead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	remoteDir := repo.addRemote("origin")
	runGit(t, repo.path(), "remote", "set-head", "origin", "main")
	runGit(t, remoteDir, "branch", "develop", "main")
	runGit(t, remoteDir, "symbolic-ref", "HEAD", "refs/heads/develop")

	branch, err := repo.Git.GetRemoteDefaultBranch("origin")

	require.NoError(t, err)
	assert.Equal(t, "develop", branch)
}

func TestGetRemoteDefaultBranch_Integration_RemoteDoesNotExist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	branch, err := repo.Git.GetRemoteDefaultBranch("nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
	assert.Empty(t, branch)
}

// =============================================================================
// CreateWorktreeForNewBranch tests
// =============================================================================

func TestCreateWorktreeForNewBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	worktreePath := filepath.Join(t.TempDir(), "new-feature")
	err := repo.Git.CreateWorktreeForNewBranch("new-feature", worktreePath)

	require.NoError(t, err)

	// Verify worktree exists
	_, err = os.Stat(worktreePath)
	require.NoError(t, err)

	// Verify branch exists
	exists, err := repo.Git.BranchExists("new-feature", false)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCreateWorktreeForNewBranch_Integration_DryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepoWithDryRun(t)
	repo.commit("initial commit")

	worktreePath := filepath.Join(t.TempDir(), "dry-run-feature")
	err := repo.Git.CreateWorktreeForNewBranch("dry-run-feature", worktreePath)

	require.NoError(t, err)

	// In dry-run mode, worktree should NOT be created
	_, err = os.Stat(worktreePath)
	assert.True(t, os.IsNotExist(err))

	// Branch should NOT exist
	// Need a non-dry-run git to check
	realGit := New(context.Background(), false, repo.path(), testTimeout, nil).(*GitCli)
	exists, err := realGit.BranchExists("dry-run-feature", false)
	require.NoError(t, err)
	assert.False(t, exists)
}

// =============================================================================
// CreateWorktreeForNewBranchFromRef tests
// =============================================================================

func TestCreateWorktreeForNewBranchFromRef_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	firstSHA := repo.commit("first commit")
	repo.commit("second commit")

	worktreePath := filepath.Join(t.TempDir(), "from-ref-feature")
	err := repo.Git.CreateWorktreeForNewBranchFromRef("from-ref-feature", worktreePath, firstSHA)

	require.NoError(t, err)

	// Verify worktree exists
	_, err = os.Stat(worktreePath)
	require.NoError(t, err)

	// Verify the branch is at the first commit
	currentSHA := strings.TrimSpace(runGit(t, worktreePath, "rev-parse", "--short", "HEAD"))
	assert.Equal(t, firstSHA, currentSHA)
}

func TestCreateWorktreeForNewBranchFromRef_Integration_EmptyRef(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("first commit")
	headSHA := repo.commit("second commit")

	worktreePath := filepath.Join(t.TempDir(), "empty-ref-feature")
	err := repo.Git.CreateWorktreeForNewBranchFromRef("empty-ref-feature", worktreePath, "")

	require.NoError(t, err)

	// With empty ref, should be at HEAD
	currentSHA := strings.TrimSpace(runGit(t, worktreePath, "rev-parse", "--short", "HEAD"))
	assert.Equal(t, headSHA, currentSHA)
}

// =============================================================================
// CreateWorktreeForExistingBranch tests
// =============================================================================

func TestCreateWorktreeForExistingBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("existing-branch")

	worktreePath := filepath.Join(t.TempDir(), "existing-branch-worktree")
	err := repo.Git.CreateWorktreeForExistingBranch("existing-branch", worktreePath)

	require.NoError(t, err)

	// Verify worktree exists
	_, err = os.Stat(worktreePath)
	require.NoError(t, err)

	// Verify it's on the correct branch
	worktreeGit := New(context.Background(), false, worktreePath, testTimeout, nil).(*GitCli)
	branch, err := worktreeGit.GetCurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "existing-branch", branch)
}

// =============================================================================
// FetchRemoteBranch tests
// =============================================================================

func TestFetchRemoteBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	remoteDir := repo.addRemote("origin")

	// Create a new branch in the remote
	runGit(t, remoteDir, "branch", "remote-feature")

	err := repo.Git.FetchRemoteBranch("origin", "remote-feature", "refs/heads/fetched-feature")

	require.NoError(t, err)

	// Verify the local branch exists
	exists, err := repo.Git.BranchExists("fetched-feature", false)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestFetchRemoteTrackingBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	remoteDir := repo.addRemote("origin")

	// Create a new branch in the remote after the initial fetch.
	runGit(t, remoteDir, "branch", "remote-feature")

	err := repo.Git.FetchRemoteTrackingBranch("origin", "remote-feature")

	require.NoError(t, err)

	remoteRefExists, err := repo.Git.RefExists("refs/remotes/origin/remote-feature")
	require.NoError(t, err)
	assert.True(t, remoteRefExists)

	localBranchExists, err := repo.Git.BranchExists("remote-feature", false)
	require.NoError(t, err)
	assert.False(t, localBranchExists)
}

// =============================================================================
// FetchRemote tests
// =============================================================================

func TestFetchRemote_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")

	output, err := repo.Git.FetchRemote("origin")

	require.NoError(t, err)
	// Output may be empty if nothing new to fetch, that's OK
	_ = output
}

func TestFetchRemote_Integration_DryRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepoWithDryRun(t)
	repo.commit("initial commit")
	// Don't actually add remote - in dry run it won't matter

	output, err := repo.Git.FetchRemote("origin")

	require.NoError(t, err)
	assert.Contains(t, output, "Would execute")
}

// =============================================================================
// SyncTags tests
// =============================================================================

func TestSyncTags_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	remoteDir := repo.addRemote("origin")

	// Create a tag in the remote
	runGit(t, remoteDir, "tag", "v1.0.0")

	err := repo.Git.SyncTags("origin")

	require.NoError(t, err)

	// Verify the tag was fetched
	tags, err := repo.Git.ListTags()
	require.NoError(t, err)
	assert.Contains(t, tagNames(tags), "v1.0.0")
}

func TestSyncTags_Integration_EmptyRemoteName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.addRemote("origin")
	repo.setConfig("remote.pushDefault", "origin")

	// Should use GetDefaultRemote fallback
	err := repo.Git.SyncTags("")

	require.NoError(t, err)
}

func TestSyncTags_Integration_RemoteOnlyTag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	remoteDir := repo.addRemote("origin")

	// Create a commit reachable only via a tag in the remote.
	tree := strings.TrimSpace(runGit(t, remoteDir, "rev-parse", "main^{tree}"))
	remoteCommit := strings.TrimSpace(runGit(t, remoteDir, "-c", "user.name=Test User", "-c", "user.email=test@example.com", "commit-tree", tree, "-m", "remote-only commit"))
	runGit(t, remoteDir, "tag", "v-remote-only", remoteCommit)

	err := repo.Git.SyncTags("origin")

	require.NoError(t, err)

	tags, err := repo.Git.ListTags()
	require.NoError(t, err)
	assert.Contains(t, tagNames(tags), "v-remote-only")
}

// =============================================================================
// RemoveWorktree tests
// =============================================================================

func TestRemoveWorktree_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature-remove")

	worktreePath := filepath.Join(t.TempDir(), "wt-remove")
	repo.createWorktree(worktreePath, "feature-remove")

	err := repo.Git.RemoveWorktree(resolvePath(t, worktreePath), false)

	require.NoError(t, err)

	_, statErr := os.Stat(worktreePath)
	assert.True(t, os.IsNotExist(statErr))

	worktrees, err := repo.Git.ListWorktrees()
	require.NoError(t, err)
	for _, wt := range worktrees {
		assert.NotEqual(t, resolvePath(t, worktreePath), wt.AbsolutePath)
	}
}

func TestRemoveWorktree_Integration_Force(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("feature-dirty")

	worktreePath := filepath.Join(t.TempDir(), "wt-dirty")
	repo.createWorktree(worktreePath, "feature-dirty")

	appendToFile(t, filepath.Join(resolvePath(t, worktreePath), "dirty.txt"), "uncommitted\n")

	err := repo.Git.RemoveWorktree(resolvePath(t, worktreePath), false)
	require.Error(t, err)

	err = repo.Git.RemoveWorktree(resolvePath(t, worktreePath), true)
	require.NoError(t, err)

	_, statErr := os.Stat(worktreePath)
	assert.True(t, os.IsNotExist(statErr))
}

// =============================================================================
// DeleteBranch tests
// =============================================================================

func TestDeleteBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("branch-to-delete")

	err := repo.Git.DeleteBranch("branch-to-delete", false)
	require.NoError(t, err)

	exists, err := repo.Git.BranchExists("branch-to-delete", false)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestDeleteBranch_Integration_Force(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("unmerged-branch")
	repo.checkout("unmerged-branch")
	repo.commit("unmerged commit")
	repo.checkout("main")

	err := repo.Git.DeleteBranch("unmerged-branch", false)
	require.Error(t, err)

	err = repo.Git.DeleteBranch("unmerged-branch", true)
	require.NoError(t, err)

	exists, err := repo.Git.BranchExists("unmerged-branch", false)
	require.NoError(t, err)
	assert.False(t, exists)
}

// =============================================================================
// IsWorktreeDirty tests
// =============================================================================

func TestIsWorktreeDirty_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")

	dirty, err := repo.Git.IsWorktreeDirty(repo.path())
	require.NoError(t, err)
	assert.False(t, dirty)

	appendToFile(t, filepath.Join(repo.path(), "untracked.txt"), "new file\n")

	dirty, err = repo.Git.IsWorktreeDirty(repo.path())
	require.NoError(t, err)
	assert.True(t, dirty)
}

// =============================================================================
// PruneWorktrees tests
// =============================================================================

func TestPruneWorktrees_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	repo := newTestRepo(t)
	repo.commit("initial commit")
	repo.createBranch("stale-branch")

	worktreePath := filepath.Join(t.TempDir(), "wt-stale")
	repo.createWorktree(worktreePath, "stale-branch")

	require.NoError(t, os.RemoveAll(worktreePath))

	porcelainBefore := strings.TrimSpace(runGit(t, repo.path(), "worktree", "list", "--porcelain"))
	assert.Contains(t, porcelainBefore, "wt-stale")

	err := repo.Git.PruneWorktrees()
	require.NoError(t, err)

	porcelainAfter := strings.TrimSpace(runGit(t, repo.path(), "worktree", "list", "--porcelain"))
	assert.NotContains(t, porcelainAfter, "wt-stale")
}
