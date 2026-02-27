package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/jmcampanini/grove-cli/internal/pr"
	"github.com/spf13/cobra"
)

var prCheckoutCmd = &cobra.Command{
	Use:   "checkout [number]",
	Short: "Check out a pull request into a local worktree",
	Long: `Check out a pull request into a local worktree.

Note: Only works with PRs from the same repository. Fork PRs are not yet supported.

To start new local work (not from a PR), use 'grove create' instead.`,
	Args: cobra.ExactArgs(1),
	RunE: runPRCheckout,
}

func init() {
	prCmd.AddCommand(prCheckoutCmd)
}

func runPRCheckout(cmd *cobra.Command, args []string) error {
	prNum, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid PR number: %s", args[0])
	}

	rt, err := loadCommandRuntime()
	if err != nil {
		return err
	}

	ctx := &prCheckoutContext{
		cfg:       rt.cfg,
		ghClient:  rt.newUncachedGitHubClient(),
		gitClient: rt.gitClient,
	}

	prInfo, err := ctx.ghClient.GetPullRequest(prNum)
	if err != nil {
		return fmt.Errorf("failed to get pull request: %w", err)
	}

	if prInfo.IsCrossRepository {
		return fmt.Errorf("PR #%d is from a fork, which is not yet supported.\nTip: You can manually add the fork as a remote and create a worktree with 'git worktree add'", prInfo.Number)
	}

	if prInfo.State == github.PRStateMerged || prInfo.State == github.PRStateClosed {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Note: PR #%d is %s\n", prInfo.Number, strings.ToLower(string(prInfo.State)))
	}

	return checkoutPRWorktree(cmd.OutOrStdout(), cmd.ErrOrStderr(), ctx, prInfo)
}

type prCheckoutContext struct {
	cfg       config.Config
	ghClient  github.GitHub
	gitClient git.Git
}

func checkoutPRWorktree(stdout, stderr io.Writer, ctx *prCheckoutContext, prInfo github.PullRequest) error {
	namer, err := naming.NewPullRequestNamer(ctx.cfg.PullRequest, ctx.cfg.Slugify)
	if err != nil {
		return fmt.Errorf("failed to create PR namer: %w", err)
	}

	prData := naming.PullRequestTemplateData{
		BranchName: prInfo.BranchName,
		Number:     prInfo.Number,
	}
	localBranch, err := namer.GenerateBranchName(prData)
	if err != nil {
		return fmt.Errorf("failed to generate branch name: %w", err)
	}

	worktreeName := namer.GenerateWorktreeName(localBranch)
	if worktreeName == "" {
		return fmt.Errorf("failed to generate worktree name: empty result")
	}

	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	matcher := pr.NewMatcher(namer)
	existingWorktree := matcher.FindWorktreeForPR(prInfo, worktrees)

	if existingWorktree != nil {
		_, _ = fmt.Fprintf(stderr, "Worktree already exists\n")
		_, err := fmt.Fprintln(stdout, existingWorktree.AbsolutePath)
		return err
	}

	workspacePath, err := ctx.gitClient.GetWorkspacePath()
	if err != nil {
		return fmt.Errorf("failed to get workspace path: %w", err)
	}

	wtPath := filepath.Join(workspacePath, worktreeName)

	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("worktree path %s already exists (not a PR worktree or different branch)", wtPath)
	}

	return ensureBranchAndCreateWorktree(stdout, stderr, ctx, namer, prInfo, localBranch, wtPath)
}

func ensureBranchAndCreateWorktree(stdout, stderr io.Writer, ctx *prCheckoutContext, namer *naming.PullRequestNamer, prInfo github.PullRequest, localBranch, wtPath string) error {
	branchExists, err := ctx.gitClient.BranchExists(localBranch, false)
	if err != nil {
		return fmt.Errorf("failed to check branch existence: %w", err)
	}

	if !branchExists {
		remote, err := ctx.gitClient.GetDefaultRemote("origin")
		if err != nil {
			return fmt.Errorf("failed to determine remote: %w", err)
		}
		fetchErr := ctx.gitClient.FetchRemoteBranch(remote, prInfo.BranchName, localBranch)
		if fetchErr != nil {
			if prInfo.State == github.PRStateMerged && prInfo.MergeCommitSHA != "" && ctx.cfg.PullRequest.AutoRecreate {
				_, _ = fmt.Fprintf(stderr, "Fetch failed (%v), attempting reconstruction from merge commit...\n", fetchErr)
				return reconstructFromMergeCommit(stdout, stderr, ctx, namer, prInfo, remote)
			}
			return fmt.Errorf("failed to fetch remote branch: %w", fetchErr)
		}
	}

	if err := ctx.gitClient.CreateWorktreeForExistingBranch(localBranch, wtPath); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	_, err = fmt.Fprintln(stdout, wtPath)
	return err
}

func reconstructFromMergeCommit(stdout, stderr io.Writer, ctx *prCheckoutContext, namer *naming.PullRequestNamer, prInfo github.PullRequest, remote string) error {
	prData := naming.PullRequestTemplateData{
		BranchName: prInfo.BranchName,
		Number:     prInfo.Number,
	}

	recreatedBranch, err := namer.GenerateRecreatedBranchName(prData)
	if err != nil {
		return fmt.Errorf("failed to generate recreated branch name: %w", err)
	}

	_, _ = fmt.Fprintf(stderr, "Recreating branch from merge commit...\n")

	if err := ctx.gitClient.FetchRef(remote, prInfo.MergeCommitSHA); err != nil {
		return fmt.Errorf("failed to fetch merge commit: %w", err)
	}

	workspacePath, err := ctx.gitClient.GetWorkspacePath()
	if err != nil {
		return fmt.Errorf("failed to get workspace path: %w", err)
	}

	worktreeName := namer.GenerateWorktreeName(recreatedBranch)
	wtPath := filepath.Join(workspacePath, worktreeName)

	branchExists, err := ctx.gitClient.BranchExists(recreatedBranch, false)
	if err != nil {
		return fmt.Errorf("failed to check branch existence: %w", err)
	}

	if branchExists {
		if err := ctx.gitClient.CreateWorktreeForExistingBranch(recreatedBranch, wtPath); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}
		_, err = fmt.Fprintln(stdout, wtPath)
		return err
	}

	baseRef, err := detectBaseRef(ctx, prInfo)
	if err != nil {
		return fmt.Errorf("failed to detect merge strategy: %w", err)
	}

	if err := ctx.gitClient.CreateWorktreeForNewBranchFromRef(recreatedBranch, wtPath, baseRef); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	if err := ctx.gitClient.MergeSquashRef(wtPath, prInfo.MergeCommitSHA); err != nil {
		return fmt.Errorf("failed to apply merge commit changes: %w", err)
	}

	commitMsg := fmt.Sprintf("PR #%d: %s", prInfo.Number, prInfo.Title)
	if err := ctx.gitClient.CommitAll(wtPath, commitMsg); err != nil {
		return fmt.Errorf("failed to commit reconstructed changes: %w", err)
	}

	_, err = fmt.Fprintln(stdout, wtPath)
	return err
}

func detectBaseRef(ctx *prCheckoutContext, prInfo github.PullRequest) (string, error) {
	parentCount, err := ctx.gitClient.GetCommitParentCount(prInfo.MergeCommitSHA)
	if err != nil {
		return "", fmt.Errorf("failed to get commit parent count: %w", err)
	}

	firstParent := prInfo.MergeCommitSHA + "^1"

	// Merge commits (2+ parents): first parent is the base branch tip
	if parentCount >= 2 {
		return firstParent, nil
	}

	// Single-parent commit with only 1 PR commit: must be squash
	if prInfo.CommitCount <= 1 {
		return firstParent, nil
	}

	// Ambiguous: single-parent with multiple PR commits could be squash or rebase.
	// Compare the merge commit's diff against the PR's total stats to disambiguate.
	files, adds, dels, err := ctx.gitClient.GetDiffStats(firstParent, prInfo.MergeCommitSHA)
	if err != nil {
		return "", fmt.Errorf("failed to get diff stats: %w", err)
	}

	statsMatch := files == prInfo.FilesChanged && adds == prInfo.LinesAdded && dels == prInfo.LinesDeleted
	if statsMatch {
		return firstParent, nil
	}

	// Rebase: merge commit SHA is the last rebased commit; walk back to find the base.
	return fmt.Sprintf("%s~%d", prInfo.MergeCommitSHA, prInfo.CommitCount), nil
}
