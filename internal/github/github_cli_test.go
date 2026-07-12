package github

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTimeout = 30 * time.Second

func TestNew(t *testing.T) {
	gh := New(context.Background(), "/some/path", 60*time.Second, nil)

	require.NotNil(t, gh)

	// Verify it implements the interface (checked at compile time by var _ GitHub = &GitHubCli{})
	_, ok := gh.(*GitHubCli)
	assert.True(t, ok, "expected *GitHubCli")
}

func TestDefaultPRLimit(t *testing.T) {
	assert.Equal(t, 20, DefaultPRLimit)
}

// skipIfGhNotAvailable skips the test if gh CLI is not installed or not authenticated.
func skipIfGhNotAvailable(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not available")
	}

	// Check if authenticated
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		t.Skip("gh CLI not authenticated")
	}
}

// skipIfNotInGitRepo skips the test if not running in a git repository.
func skipIfNotInGitRepo(t *testing.T) {
	t.Helper()

	cmd := exec.Command("git", "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		t.Skip("not in a git repository")
	}
}

// Integration tests - these require gh to be installed and authenticated,
// and require running in a git repository with a GitHub remote.

func TestGitHubCli_GetPullRequest_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfGhNotAvailable(t)
	skipIfNotInGitRepo(t)

	// This test requires a known PR number in the repository.
	// Skip if we can't determine a valid PR to test against.
	t.Skip("integration test requires a known PR number - run manually with a specific PR")
}

func TestGitHubCli_GetPullRequestByBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfGhNotAvailable(t)
	skipIfNotInGitRepo(t)

	gh := New(context.Background(), ".", testTimeout, nil)

	// Test with a branch that likely doesn't have a PR
	pr, err := gh.GetPullRequestByBranch("nonexistent-branch-12345")
	require.NoError(t, err)
	assert.Nil(t, pr, "expected nil for nonexistent branch")
}

func TestGitHubCli_ListPullRequests_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	skipIfGhNotAvailable(t)
	skipIfNotInGitRepo(t)

	gh := New(context.Background(), ".", testTimeout, nil)

	// List open PRs (may return empty list, which is fine)
	prs, err := gh.ListPullRequests(PRQuery{State: PRStateOpen}, DefaultPRLimit)
	require.NoError(t, err)
	assert.NotNil(t, prs, "expected non-nil slice (even if empty)")

	// Verify limit is respected
	assert.LessOrEqual(t, len(prs), DefaultPRLimit)
}
