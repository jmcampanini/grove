package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

var checkoutCmd = &cobra.Command{
	Use:   "checkout <branch>",
	Short: "Check out an existing branch into a new worktree",
	Long: `Check out an existing local or remote branch into a new worktree.

For local branches, specify the branch name directly:
  grove checkout feature/fix-login
  grove checkout my-experiment

For remote branches, prefix with the remote name:
  grove checkout origin/feature/fix-login
  grove checkout upstream/hotfix-123

The first path segment is checked against known git remotes. If it matches
a remote, the remainder is treated as the branch name on that remote.

Use --fetch to sync the remote before checking out:
  grove checkout origin/feature/fix-login --fetch

To start new work (create a new branch), use 'grove create' instead.
To check out a pull request by number, use 'grove pr checkout' instead.`,
	Args: cobra.ExactArgs(1),
	RunE: runCheckout,
}

func init() {
	checkoutCmd.Flags().Bool("fetch", false, "fetch from the remote before checkout (requires a remote ref)")
	checkoutCmd.GroupID = "worktree"
	rootCmd.AddCommand(checkoutCmd)
}

type checkoutContext struct {
	cfg       config.Config
	fetch     bool
	gitClient git.Git
}

type parsedRef struct {
	branchName string
	remoteName string
}

func (p parsedRef) isRemote() bool {
	return p.remoteName != ""
}

func runCheckout(cmd *cobra.Command, args []string) error {
	fetch, err := cmd.Flags().GetBool("fetch")
	if err != nil {
		return err
	}

	rt, err := loadCommandRuntime()
	if err != nil {
		return err
	}

	ctx := &checkoutContext{
		cfg:       rt.cfg,
		fetch:     fetch,
		gitClient: rt.gitClient,
	}

	return executeCheckout(cmd.OutOrStdout(), ctx, args[0])
}

func executeCheckout(stdout io.Writer, ctx *checkoutContext, ref string) error {
	if strings.TrimSpace(ref) == "" {
		return errors.New("branch name cannot be empty")
	}

	parsed, err := parseRef(ctx.gitClient, ref)
	if err != nil {
		return fmt.Errorf("failed to parse ref: %w", err)
	}

	if ctx.fetch && !parsed.isRemote() {
		return fmt.Errorf("--fetch requires a remote ref (e.g., 'origin/%s')", ref)
	}

	if parsed.isRemote() {
		return checkoutRemoteBranch(stdout, ctx, parsed)
	}
	return checkoutLocalBranch(stdout, ctx, parsed.branchName)
}

func parseRef(gitClient git.Git, ref string) (parsedRef, error) {
	candidateRemote, branchName, hasSlash := strings.Cut(ref, "/")
	if !hasSlash {
		return parsedRef{branchName: ref}, nil
	}

	remotes, err := gitClient.ListRemotes()
	if err != nil {
		return parsedRef{}, fmt.Errorf("failed to list remotes: %w", err)
	}

	if !slices.Contains(remotes, candidateRemote) {
		return parsedRef{branchName: ref}, nil
	}

	if branchName == "" {
		return parsedRef{}, fmt.Errorf("branch name cannot be empty after remote prefix %q", candidateRemote)
	}
	return parsedRef{
		branchName: branchName,
		remoteName: candidateRemote,
	}, nil
}

func checkoutLocalBranch(stdout io.Writer, ctx *checkoutContext, branchName string) error {
	exists, err := ctx.gitClient.BranchExists(branchName, false)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if !exists {
		remote, err := ctx.gitClient.GetDefaultRemote("origin")
		if err != nil {
			return fmt.Errorf("failed to determine remote: %w", err)
		}
		return fmt.Errorf("branch %q not found locally; did you mean %q?", branchName, remote+"/"+branchName)
	}

	return createWorktreeForBranch(stdout, ctx, branchName)
}

func checkoutRemoteBranch(stdout io.Writer, ctx *checkoutContext, parsed parsedRef) error {
	if ctx.fetch {
		log.WithPrefix("checkout").Info("fetching remote", "remote", parsed.remoteName)
		if _, err := ctx.gitClient.FetchRemote(parsed.remoteName); err != nil {
			return fmt.Errorf("failed to fetch remote %q: %w", parsed.remoteName, err)
		}
	}

	exists, err := ctx.gitClient.BranchExists(parsed.branchName, false)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}

	if !exists || ctx.fetch {
		log.WithPrefix("checkout").Info("fetching branch from remote", "remote", parsed.remoteName, "branch", parsed.branchName)
		if err := ctx.gitClient.FetchRemoteBranch(parsed.remoteName, parsed.branchName, parsed.branchName); err != nil {
			return fmt.Errorf("failed to fetch branch %q from remote %q: %w", parsed.branchName, parsed.remoteName, err)
		}
	}

	return createWorktreeForBranch(stdout, ctx, parsed.branchName)
}

func createWorktreeForBranch(stdout io.Writer, ctx *checkoutContext, branchName string) error {
	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	for _, wt := range worktrees {
		if wt.Ref == nil {
			continue
		}
		if branch, ok := wt.Ref.FullBranch(); ok && branch.Name == branchName {
			return fmt.Errorf("worktree already exists for branch %q at %s", branchName, wt.AbsolutePath)
		}
	}

	namer := naming.NewLocalBranchNamer(ctx.cfg.LocalBranch, ctx.cfg.Slugify)

	workspacePath, err := ctx.gitClient.GetWorkspacePath()
	if err != nil {
		return fmt.Errorf("failed to get workspace path: %w", err)
	}

	worktreeName := namer.GenerateWorktreeName(branchName)
	if worktreeName == "" {
		return fmt.Errorf("failed to generate worktree name for branch %q", branchName)
	}

	worktreePath := filepath.Join(workspacePath, worktreeName)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path %q already exists on disk", worktreePath)
	}

	if err := ctx.gitClient.CreateWorktreeForExistingBranch(branchName, worktreePath); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	_, err = fmt.Fprintln(stdout, worktreePath)
	return err
}
