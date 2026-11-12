package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	clog "charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/process"
)

// CLI implements GitHub by running the gh executable with prompts disabled.
type CLI struct {
	ctx        context.Context
	log        *clog.Logger
	timeout    time.Duration
	workingDir string
}

var _ GitHub = &CLI{}

// New creates a CLI that executes gh commands in the specified working
// directory. A nil logger falls back to the default logger.
func New(ctx context.Context, workingDir string, timeout time.Duration, logger *clog.Logger) GitHub {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = clog.Default()
	}
	return &CLI{
		ctx:        ctx,
		log:        logger.WithPrefix("github"),
		timeout:    timeout,
		workingDir: workingDir,
	}
}

func (g *CLI) runGhProcess(args ...string) (string, error) {
	g.log.Debug("Executing gh command", "cmd", "gh", "args", args, "workingDir", g.workingDir)

	result, err := process.Run(g.ctx, process.Spec{
		Args: args,
		Dir:  g.workingDir,
		Env:  append(os.Environ(), "GH_PROMPT_DISABLED=1"),
		Name: "gh",
	}, g.timeout)
	if err != nil {
		command := strings.Join(args, " ")
		switch {
		case errors.Is(err, process.ErrTimedOut):
			g.log.Debug("gh command timed out", "args", args, "timeout", g.timeout, "error", err)
			return "", fmt.Errorf("gh %s timed out after %s: %w", command, g.timeout, err)
		case errors.Is(err, process.ErrCanceled):
			g.log.Debug("gh command canceled", "args", args, "error", err)
			return "", fmt.Errorf("gh %s canceled: %w", command, err)
		case errors.Is(err, process.ErrOutputLimitExceeded):
			var limitErr *process.OutputLimitError
			if errors.As(err, &limitErr) {
				g.log.Debug("gh command output limit exceeded", "args", args, "stream", limitErr.Stream, "limit", limitErr.Limit)
				return "", fmt.Errorf("gh %s %s exceeded %d MiB output limit: %w", command, limitErr.Stream, limitErr.Limit/(1024*1024), err)
			}
		}
		g.log.Debug("gh command failed", "args", args, "error", err)
		if stderr := strings.TrimSpace(string(result.Stderr)); stderr != "" {
			return "", fmt.Errorf("gh %s failed: %w: %s", command, err, stderr)
		}
		return "", fmt.Errorf("gh %s failed: %w", command, err)
	}

	output := strings.TrimSpace(string(result.Stdout))
	g.log.Debug("gh command succeeded", "args", args, "outputLen", len(output))
	return output, nil
}

// GetIssue fetches one issue through gh issue view.
func (g *CLI) GetIssue(issueNum int) (Issue, error) {
	args := []string{
		"issue", "view", fmt.Sprintf("%d", issueNum),
		"--json", issueViewJSONFields,
	}

	output, err := g.runGhProcess(args...)
	if err != nil {
		return Issue{}, fmt.Errorf("failed to get issue #%d: %w", issueNum, err)
	}

	var issue Issue
	if err := json.Unmarshal([]byte(output), &issue); err != nil {
		return Issue{}, fmt.Errorf("failed to parse issue #%d: %w", issueNum, err)
	}

	return issue, nil
}

// GetPullRequest fetches one pull request through gh pr view.
func (g *CLI) GetPullRequest(prNum int) (PullRequest, error) {
	args := []string{
		"pr", "view", fmt.Sprintf("%d", prNum),
		"--json", prJSONFields,
	}

	output, err := g.runGhProcess(args...)
	if err != nil {
		return PullRequest{}, fmt.Errorf("failed to get pull request #%d: %w", prNum, err)
	}

	var pr PullRequest
	if err := json.Unmarshal([]byte(output), &pr); err != nil {
		return PullRequest{}, fmt.Errorf("failed to parse pull request #%d: %w", prNum, err)
	}

	return pr, nil
}

// GetPullRequestByBranch returns the pull request gh pr list reports first
// for head branch branchName, in any state, or nil when there is none.
func (g *CLI) GetPullRequestByBranch(branchName string) (*PullRequest, error) {
	args := []string{
		"pr", "list",
		"--head", branchName,
		"--state", "all",
		"--json", prJSONFields,
		"--limit", "1",
	}

	output, err := g.runGhProcess(args...)
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
