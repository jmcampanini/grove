package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/naming"
	"github.com/jmcampanini/grove/internal/pr"
	"github.com/jmcampanini/grove/internal/worktree"
	"github.com/spf13/cobra"
)

func newPRCheckoutCmd(services commandServices) *cobra.Command {
	return &cobra.Command{
		Use:   "checkout <number>",
		Short: "Check out a pull request into a local worktree",
		Long: `Check out a pull request into a local worktree and print the worktree
path on stdout.

The local branch is named by pull_request.branch_template (the pull
request's head branch by default) and the directory by
pull_request.worktree_template; run grove help naming for the variables.
When a worktree for the pull request already exists, its path is printed
and nothing is created. Otherwise the pull request head is fetched from
the default remote into the local branch when that branch does not exist
yet, and the worktree is created for it. A merged or closed pull request
is checked out with a warning on stderr. Nothing prompts.

` + helpWorktreeCreate + `

` + helpGHUncached + `

` + helpFailure + `

To start new local work that has no pull request yet, use grove create.`,
		Example: `  grove pr checkout 42
  grove pr list --fzf | fzf --delimiter '\t' --with-nth 3 --accept-nth 1 \
    | xargs grove pr checkout`,
		Args: numberArgs("pull request"),
		RunE: services.runPRCheckout,
	}
}

func (services commandServices) runPRCheckout(cmd *cobra.Command, args []string) error {
	prNum, err := parseNumberOperand(args[0])
	if err != nil {
		return err
	}

	rt, err := services.loadRuntime(cmd)
	if err != nil {
		return err
	}

	ctx := &prCheckoutContext{
		cfg:       rt.Config,
		ghClient:  rt.NewUncachedGitHubClient(),
		gitClient: rt.GitClient,
		logger:    rt.Logger,
	}

	prInfo, err := ctx.ghClient.GetPullRequest(prNum)
	if err != nil {
		return fmt.Errorf("failed to get pull request: %w", err)
	}

	if prInfo.State == github.PRStateMerged || prInfo.State == github.PRStateClosed {
		ctx.logger.WithPrefix("pr").Warn("checking out non-open pull request", "number", prInfo.Number, "state", strings.ToLower(string(prInfo.State)))
	}

	return checkoutPRWorktree(cmd.OutOrStdout(), ctx, prInfo)
}

type prCheckoutContext struct {
	cfg       config.Config
	ghClient  github.GitHub
	gitClient git.Git
	logger    *log.Logger
}

func checkoutPRWorktree(stdout io.Writer, ctx *prCheckoutContext, prInfo github.PullRequest) error {
	namer, err := naming.NewPullRequestNamer(ctx.cfg.PullRequest, ctx.cfg.Naming)
	if err != nil {
		return fmt.Errorf("failed to create PR namer: %w", err)
	}

	prData := naming.PullRequestTemplateData{
		Branch: prInfo.BranchName,
		Number: prInfo.Number,
	}
	localBranch, err := namer.GenerateBranchName(prData)
	if err != nil {
		return fmt.Errorf("failed to generate branch name: %w", err)
	}

	worktreeName, err := namer.GenerateWorktreeName(prInfo.Number, prInfo.Title, localBranch)
	if err != nil {
		return fmt.Errorf("failed to generate worktree name: %w", err)
	}

	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	matcher := pr.NewMatcher(namer, ctx.logger)
	existingWorktree := matcher.FindWorktreeForPR(prInfo, worktrees)

	if existingWorktree != nil {
		ctx.logger.WithPrefix("pr").Warn("worktree already exists for pull request", "number", prInfo.Number, "path", existingWorktree.AbsolutePath)
		_, err := fmt.Fprintln(stdout, existingWorktree.AbsolutePath)
		return err
	}

	wtPath, err := worktree.ResolvePath(ctx.cfg, ctx.gitClient, worktreeName)
	if err != nil {
		return err
	}

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
