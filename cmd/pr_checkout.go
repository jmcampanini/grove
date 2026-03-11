package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/log"
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

	if prInfo.State == github.PRStateMerged || prInfo.State == github.PRStateClosed {
		log.WithPrefix("pr").Warn("checking out non-open pull request", "number", prInfo.Number, "state", strings.ToLower(string(prInfo.State)))
	}

	return checkoutPRWorktree(cmd.OutOrStdout(), ctx, prInfo)
}

type prCheckoutContext struct {
	cfg       config.Config
	ghClient  github.GitHub
	gitClient git.Git
}

func checkoutPRWorktree(stdout io.Writer, ctx *prCheckoutContext, prInfo github.PullRequest) error {
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
		log.WithPrefix("pr").Warn("worktree already exists for pull request", "number", prInfo.Number, "path", existingWorktree.AbsolutePath)
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

	branchExists, err := ctx.gitClient.BranchExists(localBranch, false)
	if err != nil {
		return fmt.Errorf("failed to check branch existence: %w", err)
	}

	if !branchExists {
		remote, err := ctx.gitClient.GetDefaultRemote("origin")
		if err != nil {
			return fmt.Errorf("failed to determine remote: %w", err)
		}
		prRef := fmt.Sprintf("refs/pull/%d/head", prInfo.Number)
		if err := ctx.gitClient.FetchRemoteBranch(remote, prRef, localBranch); err != nil {
			return fmt.Errorf("failed to fetch remote branch: %w", err)
		}
	}

	if err := ctx.gitClient.CreateWorktreeForExistingBranch(localBranch, wtPath); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	_, err = fmt.Fprintln(stdout, wtPath)
	return err
}
