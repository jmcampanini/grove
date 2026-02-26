package git

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clog "github.com/charmbracelet/log"
	"github.com/stretchr/testify/require"
)

const testTimeout = 30 * time.Second

// newTestGitCli creates a GitCli instance suitable for unit testing.
// The logger discards output and workingDir is set to a placeholder.
func newTestGitCli() *GitCli {
	return &GitCli{
		dryRun:     false,
		log:        clog.New(io.Discard),
		timeout:    testTimeout,
		workingDir: "/nonexistent",
	}
}

// testRepo provides a temporary git repository for integration tests.
type testRepo struct {
	Git     *GitCli
	rootDir string
	t       *testing.T
}

// newTestRepo creates an initialized git repository in a temp directory.
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()

	dir := t.TempDir()

	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	return &testRepo{
		Git:     New(false, dir, testTimeout).(*GitCli),
		rootDir: dir,
		t:       t,
	}
}

// newTestRepoWithDryRun creates an initialized git repository with dry-run mode enabled.
func newTestRepoWithDryRun(t *testing.T) *testRepo {
	t.Helper()

	dir := t.TempDir()

	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	return &testRepo{
		Git:     New(true, dir, testTimeout).(*GitCli),
		rootDir: dir,
		t:       t,
	}
}

// commit creates a new commit and returns the short SHA.
func (r *testRepo) commit(message string) string {
	r.t.Helper()
	filename := filepath.Join(r.rootDir, "file.txt")
	appendToFile(r.t, filename, message+"\n")
	runGit(r.t, r.rootDir, "add", "-A")
	runGit(r.t, r.rootDir, "commit", "-m", message)
	return r.shortSHA("HEAD")
}

// createBranch creates a new branch at current HEAD.
func (r *testRepo) createBranch(name string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "branch", name)
}

// checkout switches to a branch or ref.
func (r *testRepo) checkout(ref string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "checkout", ref)
}

// checkoutDetached checks out a commit in detached HEAD state.
func (r *testRepo) checkoutDetached(ref string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "checkout", "--detach", ref)
}

// createAnnotatedTag creates an annotated tag.
func (r *testRepo) createAnnotatedTag(name, message string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "tag", "-a", name, "-m", message)
}

// createLightweightTag creates a lightweight tag.
func (r *testRepo) createLightweightTag(name string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "tag", name)
}

// createWorktree creates a worktree for an existing branch.
func (r *testRepo) createWorktree(path, branch string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "worktree", "add", path, branch)
}

// shortSHA returns the short SHA for a ref.
func (r *testRepo) shortSHA(ref string) string {
	r.t.Helper()
	return strings.TrimSpace(runGit(r.t, r.rootDir, "rev-parse", "--short", ref))
}

// addRemote adds a local bare clone as a remote and sets up tracking.
func (r *testRepo) addRemote(name string) string {
	r.t.Helper()
	remoteDir := filepath.Join(r.t.TempDir(), name+".git")
	runGit(r.t, r.rootDir, "clone", "--bare", r.rootDir, remoteDir)
	runGit(r.t, r.rootDir, "remote", "add", name, remoteDir)
	runGit(r.t, r.rootDir, "fetch", name)
	runGit(r.t, r.rootDir, "branch", "--set-upstream-to="+name+"/main", "main")
	return remoteDir
}

// setConfig sets a git config value.
func (r *testRepo) setConfig(key, value string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "config", key, value)
}

// path returns the root directory of the test repo (with symlinks resolved).
func (r *testRepo) path() string {
	resolved, err := filepath.EvalSymlinks(r.rootDir)
	if err != nil {
		return r.rootDir
	}
	return resolved
}

// runGit executes a git command and returns stdout.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	require.NoError(t, err, "git %v failed: %s", args, stderr.String())
	return stdout.String()
}

// appendToFile appends content to a file, creating it if necessary.
func appendToFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, f.Close())
	}()
	_, err = f.WriteString(content)
	require.NoError(t, err)
}

// branchNames extracts branch names from a slice of LocalBranch.
func branchNames(branches []LocalBranch) []string {
	names := make([]string, len(branches))
	for i, b := range branches {
		names[i] = b.Name
	}
	return names
}

// tagNames extracts tag names from a slice of Tag.
func tagNames(tags []Tag) []string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names
}

// remoteBranchNames extracts branch names from a slice of RemoteBranch.
func remoteBranchNames(branches []RemoteBranch) []string {
	names := make([]string, len(branches))
	for i, b := range branches {
		names[i] = b.Name
	}
	return names
}

// worktreePaths extracts absolute paths from a slice of Worktree.
func worktreePaths(worktrees []Worktree) []string {
	paths := make([]string, len(worktrees))
	for i, w := range worktrees {
		paths[i] = w.AbsolutePath
	}
	return paths
}

// commitFile creates or appends to a specific file and commits it.
func (r *testRepo) commitFile(filename, content, message string) string {
	r.t.Helper()
	filePath := filepath.Join(r.rootDir, filename)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		r.t.Fatalf("failed to create directory %s: %v", dir, err)
	}
	appendToFile(r.t, filePath, content)
	runGit(r.t, r.rootDir, "add", "-A")
	runGit(r.t, r.rootDir, "commit", "-m", message)
	return r.shortSHA("HEAD")
}

// fullSHA returns the full SHA for a ref.
func (r *testRepo) fullSHA(ref string) string {
	r.t.Helper()
	return strings.TrimSpace(runGit(r.t, r.rootDir, "rev-parse", ref))
}

// mergeNoFF performs a --no-ff merge of the given branch (creates a 2-parent merge commit).
func (r *testRepo) mergeNoFF(branch string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "merge", "--no-ff", "-m", "Merge "+branch, branch)
}

// mergeSquash performs a squash merge of the given branch.
func (r *testRepo) mergeSquash(branch string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "merge", "--squash", branch)
	runGit(r.t, r.rootDir, "commit", "-m", "Squash merge "+branch)
}

// cherryPickRange cherry-picks a range of commits (base..head) onto the current branch.
func (r *testRepo) cherryPickRange(base, head string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "cherry-pick", base+".."+head)
}

// deleteBranch deletes a local branch.
func (r *testRepo) deleteBranch(name string) {
	r.t.Helper()
	runGit(r.t, r.rootDir, "branch", "-D", name)
}

// resolvePath resolves symlinks in a path (useful for macOS /var -> /private/var).
func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
