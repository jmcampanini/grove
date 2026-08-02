package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/log/v2"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/issue"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

func newIssueStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [number]",
		Short: "Create a branch and worktree to work on an issue",
		Long: `Create a new branch and worktree to start work on a GitHub issue.

The branch is named from the issue via issue.branch_template. If a worktree
already exists for the issue, its path is printed and nothing new is created.
If a branch for the issue already exists (matched by issue number), it is
checked out into a new worktree as-is. Otherwise a new branch is created from
the latest primary branch of the default remote (fetched first).

To check out an existing pull request instead, use 'grove pr checkout'.`,
		Args: cobra.ExactArgs(1),
		RunE: runIssueStart,
	}
}

func runIssueStart(cmd *cobra.Command, args []string) error {
	issueNum, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid issue number: %s", args[0])
	}

	rt, err := loadCommandRuntime(cmd)
	if err != nil {
		return err
	}

	ctx := &issueStartContext{
		cfg:       rt.cfg,
		ghClient:  rt.newUncachedGitHubClient(),
		gitClient: rt.gitClient,
	}

	issueInfo, err := ctx.ghClient.GetIssue(issueNum)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	if issueInfo.State == github.IssueStateClosed {
		log.WithPrefix("issue").Warn("starting work on a closed issue", "number", issueInfo.Number, "reason", strings.ToLower(string(issueInfo.StateReason)))
	}

	return startIssueWorktree(cmd.OutOrStdout(), ctx, issueInfo)
}

type issueStartContext struct {
	cfg       config.Config
	ghClient  github.GitHub
	gitClient git.Git
}

func startIssueWorktree(stdout io.Writer, ctx *issueStartContext, issueInfo github.Issue) error {
	namer, err := naming.NewIssueNamer(ctx.cfg.Issue, ctx.cfg.Slugify)
	if err != nil {
		return fmt.Errorf("failed to create issue namer: %w", err)
	}

	localBranch, err := namer.GenerateBranchName(issueInfo.Number, issueInfo.Title)
	if err != nil {
		return fmt.Errorf("failed to generate branch name: %w", err)
	}

	handled, err := reuseExistingIssueWorktree(stdout, ctx, namer, issueInfo)
	if handled || err != nil {
		return err
	}

	reusableBranch, err := findReusableIssueBranch(ctx.gitClient, namer, issueInfo)
	if err != nil {
		return err
	}
	branchToUse := localBranch
	branchIsNew := true
	if reusableBranch != "" {
		branchToUse = reusableBranch
		branchIsNew = false
	}

	worktreeName := namer.GenerateWorktreeName(branchToUse)
	if worktreeName == "" {
		return fmt.Errorf("failed to generate worktree name: empty result")
	}

	workspacePath, err := ctx.gitClient.GetWorkspacePath()
	if err != nil {
		return fmt.Errorf("failed to get workspace path: %w", err)
	}

	wtPath := filepath.Join(workspacePath, worktreeName)

	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("worktree path %s already exists (not an issue worktree or different branch)", wtPath)
	}

	if branchIsNew {
		baseRef, err := resolveRemotePrimaryBaseRef(ctx.gitClient)
		if err != nil {
			return err
		}
		if err := ctx.gitClient.CreateWorktreeForNewBranchFromRef(branchToUse, wtPath, baseRef); err != nil {
			return fmt.Errorf("failed to create branch and worktree: %w", err)
		}
	} else {
		log.WithPrefix("issue").Warn("reusing existing branch for issue", "number", issueInfo.Number, "branch", branchToUse)
		if err := ctx.gitClient.CreateWorktreeForExistingBranch(branchToUse, wtPath); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
	}

	_, err = fmt.Fprintln(stdout, wtPath)
	return err
}

// reuseExistingIssueWorktree prints the path of a live worktree already matched
// to the issue and reports handled=true so the caller stops. Stale entries
// (still registered but gone from disk) are pruned so creation can proceed.
func reuseExistingIssueWorktree(stdout io.Writer, ctx *issueStartContext, namer *naming.IssueNamer, issueInfo github.Issue) (bool, error) {
	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return true, fmt.Errorf("failed to list worktrees: %w", err)
	}

	existingWorktree := issue.NewMatcher(namer).FindWorktreeForIssue(issueInfo, worktrees)
	if existingWorktree == nil {
		return false, nil
	}

	if _, statErr := os.Stat(existingWorktree.AbsolutePath); statErr == nil {
		log.WithPrefix("issue").Warn("worktree already exists for issue", "number", issueInfo.Number, "path", existingWorktree.AbsolutePath)
		_, err := fmt.Fprintln(stdout, existingWorktree.AbsolutePath)
		return true, err
	}

	log.WithPrefix("issue").Warn("stale worktree entry found for issue", "number", issueInfo.Number, "path", existingWorktree.AbsolutePath)
	if err := ctx.gitClient.PruneWorktrees(); err != nil {
		return true, fmt.Errorf("failed to prune stale worktrees: %w", err)
	}
	return false, nil
}

// findReusableIssueBranch returns the first local branch matched to the issue
// by number, or "" when none exists. Matching by number rather than exact name
// means a title edit cannot orphan a branch that already has work on it.
func findReusableIssueBranch(gitClient git.Git, namer *naming.IssueNamer, issueInfo github.Issue) (string, error) {
	branches, err := gitClient.ListLocalBranches()
	if err != nil {
		return "", fmt.Errorf("failed to list local branches: %w", err)
	}
	for _, b := range branches {
		if namer.MatchesIssueNumber(b.Name, issueInfo.Number, issueInfo.Title) {
			return b.Name, nil
		}
	}
	return "", nil
}
