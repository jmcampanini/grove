package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	clog "github.com/charmbracelet/log"
)

// DefaultPRLimit is the maximum number of pull requests returned by ListPullRequests.
const DefaultPRLimit = 20

// GitHubCli provides GitHub operations by executing the gh CLI.
type GitHubCli struct {
	log        *clog.Logger
	timeout    time.Duration
	workingDir string
}

var _ GitHub = &GitHubCli{}

// New creates a new GitHubCli instance that executes gh commands
// in the specified working directory.
func New(workingDir string, timeout time.Duration) GitHub {
	return &GitHubCli{
		log:        clog.Default().WithPrefix("github"),
		timeout:    timeout,
		workingDir: workingDir,
	}
}

func (g *GitHubCli) executeGhCommand(args ...string) (string, error) {
	g.log.Debug("Executing gh command", "cmd", "gh", "args", args, "workingDir", g.workingDir)

	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = g.workingDir
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			g.log.Warn("gh command timed out", "args", args, "timeout", g.timeout, "error", err)
			return "", fmt.Errorf("gh %s timed out after %s", strings.Join(args, " "), g.timeout)
		}
		g.log.Warn("gh command failed", "args", args, "stderr", stderr.String(), "error", err)
		return "", fmt.Errorf("gh %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	g.log.Debug("gh command succeeded", "args", args, "outputLen", len(output))
	return output, nil
}

func (g *GitHubCli) GetPullRequest(prNum int) (PullRequest, error) {
	args := []string{
		"pr", "view", fmt.Sprintf("%d", prNum),
		"--json", prJsonFields,
	}

	output, err := g.executeGhCommand(args...)
	if err != nil {
		return PullRequest{}, fmt.Errorf("failed to get pull request #%d: %w", prNum, err)
	}

	var pr PullRequest
	if err := json.Unmarshal([]byte(output), &pr); err != nil {
		return PullRequest{}, fmt.Errorf("failed to parse pull request #%d: %w", prNum, err)
	}

	return pr, nil
}

func (g *GitHubCli) GetPullRequestByBranch(branchName string) (*PullRequest, error) {
	args := []string{
		"pr", "list",
		"--head", branchName,
		"--state", "all",
		"--json", prJsonFields,
		"--limit", "1",
	}

	output, err := g.executeGhCommand(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request for branch %s: %w", branchName, err)
	}

	var prs []PullRequest
	if err := json.Unmarshal([]byte(output), &prs); err != nil {
		return nil, fmt.Errorf("failed to parse pull requests for branch %s: %w", branchName, err)
	}

	if len(prs) == 0 {
		return nil, nil
	}

	return &prs[0], nil
}

func (g *GitHubCli) GetPullRequestFiles(prNum int) ([]PullRequestFile, error) {
	args := []string{
		"pr", "view", fmt.Sprintf("%d", prNum),
		"--json", "files",
	}

	output, err := g.executeGhCommand(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get files for pull request #%d: %w", prNum, err)
	}

	// gh returns: {"files": [{"path": "...", "additions": N, "deletions": N}, ...]}
	var result struct {
		Files []PullRequestFile `json:"files"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("failed to parse files for pull request #%d: %w", prNum, err)
	}

	return result.Files, nil
}

func (g *GitHubCli) ListPullRequests(query PRQuery, limit int) ([]PullRequest, error) {
	searchQuery := query.ToSearchQuery()

	args := []string{
		"pr", "list",
		"--search", searchQuery,
		"--json", prJsonFields,
		"--limit", fmt.Sprintf("%d", limit),
	}

	output, err := g.executeGhCommand(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests: %w", err)
	}

	var prs []PullRequest
	if err := json.Unmarshal([]byte(output), &prs); err != nil {
		return nil, fmt.Errorf("failed to parse pull requests: %w", err)
	}

	return prs, nil
}

func (g *GitHubCli) Validate() error {
	// Check if gh is installed
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found: install from https://cli.github.com")
	}

	// Check auth status - gh auth status exits non-zero if not authenticated
	if _, err := g.executeGhCommand("auth", "status"); err != nil {
		return fmt.Errorf("gh CLI not authenticated: run 'gh auth login'")
	}

	return nil
}
