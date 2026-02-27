package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	clog "github.com/charmbracelet/log"
)

// GitCli provides high-level git operations by executing real git commands via the git CLI.
type GitCli struct {
	dryRun     bool
	log        *clog.Logger
	timeout    time.Duration
	workingDir string
}

var _ Git = &GitCli{}

// New creates a new GitCli instance that executes git commands in the specified working directory.
func New(dryRun bool, workingDir string, timeout time.Duration) Git {
	return &GitCli{
		dryRun:     dryRun,
		log:        clog.Default().WithPrefix("git"),
		timeout:    timeout,
		workingDir: workingDir,
	}
}

func (g *GitCli) executeGitCommand(args ...string) (string, error) {
	g.log.Debug("Executing git command", "cmd", "git", "args", args, "workingDir", g.workingDir)

	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.workingDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			g.log.Warn("git command timed out", "args", args, "timeout", g.timeout, "error", err)
			return "", fmt.Errorf("git %s timed out after %s", strings.Join(args, " "), g.timeout)
		}
		g.log.Debug("Git command failed", "args", args, "stderr", stderr.String(), "error", err)
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	g.log.Debug("Git command succeeded", "args", args, "output", output)
	return output, nil
}

// executeMutatingGitCommand runs a git command that modifies state, unless in dry-run mode.
func (g *GitCli) executeMutatingGitCommand(errContext string, args ...string) error {
	if g.dryRun {
		g.log.Info("Would execute git command", "cmd", "git", "args", args)
		return nil
	}
	if _, err := g.executeGitCommand(args...); err != nil {
		return fmt.Errorf("%s: %w", errContext, err)
	}
	return nil
}

// executeMutatingGitCommandWithOutput runs a git command that modifies state and returns its output.
func (g *GitCli) executeMutatingGitCommandWithOutput(errContext string, args ...string) (string, error) {
	if g.dryRun {
		g.log.Info("Would execute git command", "cmd", "git", "args", args)
		return fmt.Sprintf("Would execute: git %s", strings.Join(args, " ")), nil
	}
	output, err := g.executeGitCommand(args...)
	if err != nil {
		return output, fmt.Errorf("%s: %w", errContext, err)
	}
	return output, nil
}

func (g *GitCli) CommitAll(worktreeAbsPath, message string) error {
	g.log.Info("Committing all changes", "path", worktreeAbsPath, "message", message)
	args := []string{"-C", worktreeAbsPath, "commit", "--no-gpg-sign", "-am", message}
	return g.executeMutatingGitCommand("failed to commit all changes", args...)
}

func (g *GitCli) FetchRef(remote, ref string) error {
	g.log.Info("Fetching ref", "remote", remote, "ref", ref)
	args := []string{"fetch", remote, ref}
	return g.executeMutatingGitCommand("failed to fetch ref", args...)
}

func (g *GitCli) GetCommitParentCount(sha string) (int, error) {
	output, err := g.executeGitCommand("cat-file", "-p", sha)
	if err != nil {
		return 0, fmt.Errorf("failed to get commit info for %s: %w", sha, err)
	}

	isCommit := false
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "tree ") {
			isCommit = true
		}
		if strings.HasPrefix(line, "parent ") {
			count++
		}
	}
	if !isCommit {
		return 0, fmt.Errorf("object %s is not a commit", sha)
	}
	return count, nil
}

func (g *GitCli) GetDiffStats(base, head string) (DiffStats, error) {
	output, err := g.executeGitCommand("diff", "--shortstat", base, head)
	if err != nil {
		return DiffStats{}, fmt.Errorf("failed to get diff stats for %s..%s: %w", base, head, err)
	}
	if output == "" {
		return DiffStats{}, nil
	}
	var stats DiffStats
	for _, part := range strings.Split(output, ",") {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		n, parseErr := strconv.Atoi(fields[0])
		if parseErr != nil {
			continue
		}
		switch {
		case strings.Contains(part, "file"):
			stats.FilesChanged = n
		case strings.Contains(part, "insertion"):
			stats.Additions = n
		case strings.Contains(part, "deletion"):
			stats.Deletions = n
		}
	}
	if stats.FilesChanged == 0 && stats.Additions == 0 && stats.Deletions == 0 {
		return DiffStats{}, fmt.Errorf("failed to parse diff stats from output: %q", output)
	}
	return stats, nil
}

func (g *GitCli) MergeSquashRef(worktreeAbsPath, ref string) error {
	g.log.Info("Squash merging ref", "path", worktreeAbsPath, "ref", ref)
	args := []string{"-C", worktreeAbsPath, "merge", "--squash", ref}
	return g.executeMutatingGitCommand("failed to squash merge ref", args...)
}

func (g *GitCli) GetMainWorktreePath() (string, error) {
	commonDir, err := g.executeGitCommand("rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("failed to get git common dir: %w", err)
	}

	absCommonDir := commonDir
	if !filepath.IsAbs(commonDir) {
		absCommonDir = filepath.Join(g.workingDir, commonDir)
	}

	absCommonDir, err = filepath.Abs(absCommonDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	absCommonDir = filepath.Clean(absCommonDir)
	mainWorktree := filepath.Dir(absCommonDir)

	g.log.Debug("Resolved main worktree path", "commonDir", commonDir, "mainWorktree", mainWorktree)
	return mainWorktree, nil
}

func (g *GitCli) GetWorkspacePath() (string, error) {
	mainWorktreePath, err := g.GetMainWorktreePath()
	if err != nil {
		return "", fmt.Errorf("failed to get main worktree path: %w", err)
	}
	return filepath.Dir(mainWorktreePath), nil
}

func (g *GitCli) GetWorktreeRoot() (string, error) {
	output, err := g.executeGitCommand("rev-parse", "--show-toplevel")
	if err != nil {
		if strings.Contains(err.Error(), "not a git repo") {
			// Not in a git repo - this is a valid state, not an error
			return "", nil
		}
		return "", fmt.Errorf("git command failed: %w", err)
	}
	return output, nil
}

func (g *GitCli) GetCurrentBranch() (string, error) {
	output, err := g.executeGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return output, nil
}

func (g *GitCli) GetCommitSubject() (string, error) {
	output, err := g.executeGitCommand("log", "-1", "--format=%s")
	if err != nil {
		return "", fmt.Errorf("failed to get commit subject: %w", err)
	}
	return output, nil
}

func (g *GitCli) GetDefaultRemote(fallback string) (string, error) {
	output, err := g.executeGitCommand("config", "--get", "remote.pushDefault")
	if err == nil && output != "" {
		g.log.Debug("Found remote.pushDefault", "remote", output)
		return output, nil
	}

	g.log.Debug("No remote.pushDefault configured, using fallback", "fallback", fallback)
	return fallback, nil
}

// remoteExists checks if a remote with the given name is configured.
func (g *GitCli) remoteExists(remoteName string) (bool, error) {
	_, err := g.executeGitCommand("remote", "get-url", remoteName)
	if err == nil {
		return true, nil
	}

	if strings.Contains(err.Error(), "No such remote") {
		return false, nil
	}

	return false, err
}

func (g *GitCli) GetRepoDefaultBranch(remoteName string) (string, error) {
	if exists, err := g.remoteExists(remoteName); err != nil {
		return "", fmt.Errorf("failed to check remote existence: %w", err)
	} else if !exists {
		return "", fmt.Errorf("remote '%s' does not exist", remoteName)
	}

	output, err := g.executeGitCommand("rev-parse", "--abbrev-ref", remoteName+"/HEAD")
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision") {
			g.log.Debug("Remote HEAD not configured", "remoteName", remoteName)
			return "", nil
		}
		return "", fmt.Errorf("failed to get remote HEAD: %w", err)
	}

	branchName := strings.TrimPrefix(output, remoteName+"/")
	return branchName, nil
}

func (g *GitCli) ListLocalBranches() ([]LocalBranch, error) {
	format := `branch %(refname:short)
checkedOut %(if)%(HEAD)%(then)true%(else)false%(end)
commit %(objectname:short)
%(if)%(upstream:short)%(then)upstream %(upstream:short)
%(end)%(if)%(upstream:track)%(then)track %(upstream:track)
%(end)committedOn %(committerdate:iso-strict)
committedBy %(committername)
subject %(contents:subject)
worktreepath %(worktreepath)
`
	output, err := g.executeGitCommand("for-each-ref", "--format="+format, "refs/heads/")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	if output == "" {
		return []LocalBranch{}, nil
	}

	return parseBranchesFromFormat(output), nil
}

// parseBranchesFromFormat parses the output of `git for-each-ref` with a custom format
// and returns a slice of LocalBranch structs with all metadata.
func parseBranchesFromFormat(output string) []LocalBranch {
	blocks := splitIntoBlocks(output)
	branches := make([]LocalBranch, 0, len(blocks))

	for _, block := range blocks {
		branch := parseBranchBlock(block)
		if branch.Name != "" {
			branches = append(branches, branch)
		}
	}

	return branches
}

// splitIntoBlocks splits porcelain output into blocks separated by blank lines.
// Each block is a slice of non-empty lines.
func splitIntoBlocks(output string) [][]string {
	var blocks [][]string
	var currentBlock []string

	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			currentBlock = append(currentBlock, line)
			continue
		}

		// found blank line, new block
		if len(currentBlock) > 0 {
			blocks = append(blocks, currentBlock)
			currentBlock = nil
		}
	}

	// handle last block if output doesn't end with blank line
	if len(currentBlock) > 0 {
		blocks = append(blocks, currentBlock)
	}

	return blocks
}

func parseBranchBlock(lines []string) LocalBranch {
	fields := parseLineFields(lines)

	name := fields["branch"]
	isCheckedOut := fields["checkedOut"] == "true"
	sha := fields["commit"]
	upstreamName := fields["upstream"]
	ahead, behind := parseTrackInfo(fields["track"])
	committedOn := parseISO8601Date(fields["committedOn"])
	committedBy := fields["committedBy"]
	subject := fields["subject"]
	worktreeAbsolutePath := fields["worktreepath"]

	commit := NewCommit(sha, subject, committedOn, committedBy)
	return NewLocalBranch(name, upstreamName, worktreeAbsolutePath, isCheckedOut, ahead, behind, commit)
}

// parseTrackInfo parses the upstream track info string like "[ahead 3, behind 2]"
func parseTrackInfo(track string) (ahead, behind int) {
	if track == "" || track == "[gone]" {
		return 0, 0
	}
	if n, _ := fmt.Sscanf(track, "[ahead %d, behind %d]", &ahead, &behind); n == 2 {
		return ahead, behind
	}
	if n, _ := fmt.Sscanf(track, "[ahead %d]", &ahead); n == 1 {
		return ahead, 0
	}
	if n, _ := fmt.Sscanf(track, "[behind %d]", &behind); n == 1 {
		return 0, behind
	}
	return 0, 0
}

// getCommitBySHA retrieves full commit information for a given SHA.
// Used when parsing worktrees with detached HEAD.
func (g *GitCli) getCommitBySHA(sha string) (Commit, error) {
	// Format: subject<NUL>committer date ISO<NUL>committer name
	// NUL separators handle multi-line subjects more robustly than line-based parsing
	format := "%s%x00%cI%x00%cn"
	output, err := g.executeGitCommand("log", "-1", "--format="+format, sha)
	if err != nil {
		return Commit{}, fmt.Errorf("failed to get commit info for %s: %w", sha, err)
	}

	parts := strings.Split(output, "\x00")
	if len(parts) != 3 {
		return Commit{}, fmt.Errorf("unexpected commit format for %s: got %d parts", sha, len(parts))
	}

	subject := parts[0]
	committedOn := time.Time{}
	if parsed, err := time.Parse(time.RFC3339, parts[1]); err == nil {
		committedOn = parsed
	} else {
		g.log.Debug("failed to parse commit date", "sha", sha, "date", parts[1], "error", err)
	}
	committedBy := parts[2]

	return NewCommit(sha, subject, committedOn, committedBy), nil
}

func (g *GitCli) ListRemoteBranches(remoteName string) ([]RemoteBranch, error) {
	format := `ref %(refname:short)
commit %(objectname:short)
committedOn %(committerdate:iso-strict)
committedBy %(committername)
subject %(contents:subject)
`
	output, err := g.executeGitCommand("for-each-ref", "--format="+format, "refs/remotes/"+remoteName+"/")
	if err != nil {
		return nil, fmt.Errorf("failed to list remote branches: %w", err)
	}

	if output == "" {
		return []RemoteBranch{}, nil
	}

	return parseRemoteBranchesFromFormat(output), nil
}

func parseRemoteBranchesFromFormat(output string) []RemoteBranch {
	blocks := splitIntoBlocks(output)
	branches := make([]RemoteBranch, 0, len(blocks))

	for _, block := range blocks {
		branch := parseRemoteBranchBlock(block)
		// Skip symbolic HEAD ref (e.g., origin/HEAD -> origin/main)
		if branch.Name != "" && branch.Name != "HEAD" {
			branches = append(branches, branch)
		}
	}

	return branches
}

func parseRemoteBranchBlock(lines []string) RemoteBranch {
	fields := parseLineFields(lines)

	// ref is "origin/main", split on first "/" to get remote and branch name
	var name, remoteName string
	if ref := fields["ref"]; ref != "" {
		if idx := strings.Index(ref, "/"); idx != -1 {
			remoteName = ref[:idx]
			name = ref[idx+1:]
		}
	}

	sha := fields["commit"]
	committedOn := parseISO8601Date(fields["committedOn"])
	committedBy := fields["committedBy"]
	subject := fields["subject"]

	commit := NewCommit(sha, subject, committedOn, committedBy)
	return NewRemoteBranch(name, remoteName, commit)
}

func (g *GitCli) ListRemotes() ([]string, error) {
	output, err := g.executeGitCommand("remote")
	if err != nil {
		return nil, fmt.Errorf("failed to list remotes: %w", err)
	}

	if output == "" {
		return []string{}, nil
	}

	lines := strings.Split(output, "\n")
	remotes := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			remotes = append(remotes, trimmed)
		}
	}
	return remotes, nil
}

func (g *GitCli) SyncTags(remoteName string) error {
	if remoteName == "" {
		var err error
		remoteName, err = g.GetDefaultRemote("origin")
		if err != nil {
			return fmt.Errorf("failed to get default remote: %w", err)
		}
	}

	g.log.Info("Syncing tags from remote", "remote", remoteName)
	args := []string{"fetch", remoteName, "--prune", "--prune-tags", "--tags"}
	return g.executeMutatingGitCommand("failed to sync tags from remote", args...)
}

func (g *GitCli) ListTags() ([]Tag, error) {
	format := `name %(refname:short)
objecttype %(objecttype)
objectsha %(objectname)
derefsha %(*objectname)
taggername %(taggername)
taggeremail %(taggeremail)
taggedon %(taggerdate:iso-strict)
message %(contents:subject)
committedby %(*committername)
committedon %(*committerdate:iso-strict)
committerdate %(committerdate:iso-strict)
commitsubject %(*subject)
`
	output, err := g.executeGitCommand("for-each-ref", "--format="+format, "refs/tags/")
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	if output == "" {
		return []Tag{}, nil
	}

	return parseTagsFromFormat(output), nil
}

// parseISO8601Date attempts to parse an ISO 8601 date string and returns zero time on failure.
func parseISO8601Date(dateStr string) time.Time {
	if parsed, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return parsed
	}
	return time.Time{}
}

func parseTagsFromFormat(output string) []Tag {
	blocks := splitIntoBlocks(output)
	tags := make([]Tag, 0, len(blocks))

	for _, block := range blocks {
		tag := parseTagBlock(block)
		if tag.Name != "" {
			tags = append(tags, tag)
		}
	}

	return tags
}

// parseLineFields extracts key-value pairs from lines in "key value" format.
// Lines without a space are treated as keys with empty string values.
func parseLineFields(lines []string) map[string]string {
	fields := make(map[string]string, len(lines))
	for _, line := range lines {
		if idx := strings.Index(line, " "); idx != -1 {
			fields[line[:idx]] = line[idx+1:]
		} else if line != "" {
			fields[line] = ""
		}
	}
	return fields
}

func parseTagBlock(lines []string) Tag {
	fields := parseLineFields(lines)

	name := fields["name"]
	objectType := fields["objecttype"]
	objectSHA := fields["objectsha"]
	derefSHA := fields["derefsha"]
	taggerName := fields["taggername"]
	taggerEmail := fields["taggeremail"]
	taggedOn := parseISO8601Date(fields["taggedon"])
	message := fields["message"]
	committedBy := fields["committedby"]
	committedOn := parseISO8601Date(fields["committedon"])
	committerDate := parseISO8601Date(fields["committerdate"])
	commitSubject := fields["commitsubject"]

	// Fallback: use committerDate if committedOn is zero (lightweight tags)
	if committedOn.IsZero() && !committerDate.IsZero() {
		committedOn = committerDate
	}

	// Determine the actual commit SHA and metadata
	// For lightweight tags (objecttype=commit): objectSHA IS the commit
	// For annotated tags (objecttype=tag): derefSHA is the dereferenced commit SHA
	var actualCommitSHA string
	var actualMessage string
	if objectType == "commit" {
		// Lightweight tag - objectSHA IS the commit
		actualCommitSHA = objectSHA
		// For lightweight tags, commit metadata comes from the commit itself
		// The %(*...) placeholders are empty, so use the tag's message as subject fallback
		if commitSubject == "" {
			commitSubject = message
		}
		// Don't set message for lightweight tags - they don't have tag messages
		actualMessage = ""
	} else {
		// Annotated tag - use dereferenced commit SHA
		actualCommitSHA = derefSHA
		actualMessage = message
	}

	commit := NewCommit(actualCommitSHA, commitSubject, committedOn, committedBy)
	return NewTag(name, commit, actualMessage, taggerName, taggerEmail, taggedOn)
}

func (g *GitCli) BranchExists(branchName string, caseInsensitive bool) (bool, error) {
	branches, err := g.ListLocalBranches()
	if err != nil {
		return false, err
	}

	for _, branch := range branches {
		if caseInsensitive {
			if strings.EqualFold(branch.Name, branchName) {
				return true, nil
			}
		} else {
			if branch.Name == branchName {
				return true, nil
			}
		}
	}

	return false, nil
}

func (g *GitCli) ListWorktrees() ([]Worktree, error) {
	branches, err := g.ListLocalBranches()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches for worktree lookup: %w", err)
	}

	tags, err := g.ListTags()
	if err != nil {
		return nil, fmt.Errorf("failed to list tags for worktree lookup: %w", err)
	}

	branchMap := make(map[string]LocalBranch, len(branches))
	for _, b := range branches {
		branchMap[b.Name] = b
	}

	tagMap := make(map[string]Tag, len(tags))
	for _, t := range tags {
		tagMap[t.Commit().SHA] = t
	}

	output, err := g.executeGitCommand("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return g.parseWorktreesFromPorcelain(output, branchMap, tagMap)
}

// parseWorktreesFromPorcelain parses the output of `git worktree list --porcelain`
func (g *GitCli) parseWorktreesFromPorcelain(output string, branchMap map[string]LocalBranch, tagMap map[string]Tag) ([]Worktree, error) {
	blocks := splitIntoBlocks(output)
	worktrees := make([]Worktree, 0, len(blocks))

	for _, block := range blocks {
		worktree, err := g.parseWorktreeBlock(block, branchMap, tagMap)
		if err != nil {
			return nil, err
		}
		if worktree.AbsolutePath != "" {
			worktrees = append(worktrees, worktree)
		}
	}

	return worktrees, nil
}

func (g *GitCli) parseWorktreeBlock(lines []string, branchMap map[string]LocalBranch, tagMap map[string]Tag) (Worktree, error) {
	fields := parseLineFields(lines)

	absolutePath := fields["worktree"]
	sha := fields["HEAD"]

	// Bare worktrees don't have a ref, skip
	if _, isBare := fields["bare"]; isBare {
		return Worktree{AbsolutePath: absolutePath}, nil
	}

	branchName := strings.TrimPrefix(fields["branch"], "refs/heads/")
	_, detached := fields["detached"]

	worktree := Worktree{AbsolutePath: absolutePath}

	if branchName != "" {
		branch, ok := branchMap[branchName]
		if !ok {
			return Worktree{}, fmt.Errorf("worktree branch '%s' at sha '%s' not found in local map", branchName, sha)
		}
		worktree.Ref = &branch
	} else if detached {
		if tag, ok := tagMap[sha]; ok {
			worktree.Ref = &tag
		} else {
			// TODO: figure out a way to get the commit information without needing a reference to g GitCli
			commit, err := g.getCommitBySHA(sha)
			if err != nil {
				return Worktree{}, fmt.Errorf("failed to get commit for detached worktree: %w", err)
			}
			worktree.Ref = &commit
		}
	}

	return worktree, nil
}

func (g *GitCli) CreateWorktreeForNewBranch(newBranchName, worktreeAbsPath string) error {
	g.log.Info("Creating worktree for new branch", "branch", newBranchName, "path", worktreeAbsPath)
	args := []string{"worktree", "add", "-b", newBranchName, worktreeAbsPath}
	return g.executeMutatingGitCommand("failed to create worktree for new branch", args...)
}

func (g *GitCli) CreateWorktreeForNewBranchFromRef(newBranchName, worktreeAbsPath, baseRef string) error {
	g.log.Info("Creating worktree for new branch from ref", "branch", newBranchName, "path", worktreeAbsPath, "baseRef", baseRef)
	args := []string{"worktree", "add", "-b", newBranchName, worktreeAbsPath}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	return g.executeMutatingGitCommand("failed to create worktree for new branch from ref", args...)
}

func (g *GitCli) CreateWorktreeForExistingBranch(branchName, worktreeAbsPath string) error {
	g.log.Info("Creating worktree for existing branch", "branch", branchName, "path", worktreeAbsPath)
	args := []string{"worktree", "add", worktreeAbsPath, branchName}
	return g.executeMutatingGitCommand("failed to create worktree for existing branch", args...)
}

func (g *GitCli) FetchRemoteBranch(remote, remoteRef, localRef string) error {
	g.log.Info("Fetching remote branch", "remote", remote, "remoteRef", remoteRef, "localRef", localRef)
	refSpec := remoteRef + ":" + localRef
	args := []string{"fetch", remote, refSpec}
	return g.executeMutatingGitCommand("failed to fetch remote branch", args...)
}

func (g *GitCli) FetchRemote(remoteName string) (string, error) {
	g.log.Info("Fetching from remote", "remote", remoteName)
	args := []string{"fetch", remoteName, "--prune", "--prune-tags", "--tags"}
	return g.executeMutatingGitCommandWithOutput("failed to fetch from remote", args...)
}

func (g *GitCli) DeleteBranch(name string, force bool) error {
	g.log.Info("Deleting branch", "branch", name, "force", force)
	flag := "-d"
	if force {
		flag = "-D"
	}
	return g.executeMutatingGitCommand("failed to delete branch", "branch", flag, name)
}

func (g *GitCli) IsWorktreeDirty(absPath string) (bool, error) {
	g.log.Debug("Checking worktree dirty state", "path", absPath)
	output, err := g.executeGitCommand("-C", absPath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check worktree dirty state: %w", err)
	}
	return output != "", nil
}

func (g *GitCli) PruneWorktrees() error {
	g.log.Info("Pruning stale worktree metadata")
	return g.executeMutatingGitCommand("failed to prune worktrees", "worktree", "prune")
}

func (g *GitCli) RemoveWorktree(absPath string, force bool) error {
	g.log.Info("Removing worktree", "path", absPath, "force", force)
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, absPath)
	return g.executeMutatingGitCommand("failed to remove worktree", args...)
}
