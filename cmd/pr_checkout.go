package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
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
	prCheckoutCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt for reconstruction")
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

	yes, _ := cmd.Flags().GetBool("yes")

	ctx := &prCheckoutContext{
		cfg:         rt.cfg,
		ghClient:    rt.newUncachedGitHubClient(),
		gitClient:   rt.gitClient,
		skipConfirm: yes,
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
	cfg         config.Config
	confirmFn   func(title string) (bool, error)
	ghClient    github.GitHub
	gitClient   git.Git
	skipConfirm bool
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
			if prInfo.State == github.PRStateMerged && prInfo.MergeCommitSHA != "" {
				return promptAndReconstruct(stdout, stderr, ctx, namer, prInfo, remote)
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

func promptAndReconstruct(stdout, stderr io.Writer, ctx *prCheckoutContext, namer *naming.PullRequestNamer, prInfo github.PullRequest, remote string) error {
	if !ctx.skipConfirm {
		title := fmt.Sprintf(
			"PR #%d is merged and the branch has been deleted.\n\n"+
				"Grove can reconstruct a local branch by replaying the squash merge commit\n"+
				"onto its parent. This only produces correct results for squash-merged PRs.\n\n"+
				"Reconstruct branch?",
			prInfo.Number,
		)

		confirmed, err := confirmReconstruction(ctx, title)
		if err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return err
		}
		if !confirmed {
			return nil
		}
	}

	return reconstructFromMergeCommit(stdout, stderr, ctx, namer, prInfo, remote)
}

func confirmReconstruction(ctx *prCheckoutContext, title string) (bool, error) {
	if ctx.confirmFn != nil {
		return ctx.confirmFn(title)
	}

	var confirmed bool
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Affirmative("Yes").
				Negative("No").
				Value(&confirmed),
		),
	).Run()
	return confirmed, err
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

	_, _ = fmt.Fprintf(stderr, "Reconstructing branch from squash merge commit...\n")

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

	baseRef := prInfo.MergeCommitSHA + "^1"

	if err := ctx.gitClient.CreateWorktreeForNewBranchFromRef(recreatedBranch, wtPath, baseRef); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	cleanup := func() {
		_, _ = fmt.Fprintf(stderr, "Cleaning up partial reconstruction...\n")
		_ = ctx.gitClient.RemoveWorktree(wtPath, true)
		_ = ctx.gitClient.DeleteBranch(recreatedBranch, true)
	}

	if err := ctx.gitClient.MergeSquashRef(wtPath, prInfo.MergeCommitSHA); err != nil {
		cleanup()
		return fmt.Errorf("failed to apply merge commit changes: %w", err)
	}

	commitMsg := fmt.Sprintf("PR #%d: %s", prInfo.Number, prInfo.Title)
	if err := ctx.gitClient.CommitAll(wtPath, commitMsg); err != nil {
		cleanup()
		return fmt.Errorf("failed to commit reconstructed changes: %w", err)
	}

	_, err = fmt.Fprintln(stdout, wtPath)
	return err
}
