package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/log/v2"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <phrase>",
		Short: "Create a new branch and worktree from a descriptive phrase",
		Long: `Create creates a new git branch and worktree from a descriptive phrase.

By default, the new branch is created from the current HEAD. Use --from to
specify a different starting point (any git ref: branch, tag, or commit SHA).
Use --from-remote-primary to fetch and start from the latest remote primary
branch without updating the primary worktree; this is recommended for automation.
Use --reuse to attach to an existing branch/worktree for the phrase instead of
failing when it already exists.

Creation mode flags are mutually exclusive; choose at most one:
  --from <ref>
  --from-remote-primary
  --reuse

The phrase is converted to a branch name using the configured slugify rules
and prefix. A worktree is then created with the configured worktree naming.
Use the global --worktree-prefix flag to override the configured worktree
directory prefix for a single invocation.

Example:
  grove create "add user authentication"
  grove create "fix bug in login"
  grove create "hotfix" --from main
  grove create "backport" --from v1.2.0
  grove create "experiment" --from origin/develop
  grove create "add user authentication" --from-remote-primary
  grove create "add user authentication" --reuse
  grove create "run tests" --worktree-prefix subagent-

Note: The create command takes a single quoted string argument.

To check out an existing pull request, use 'grove pr checkout' instead.`,
		Args:    cobra.ExactArgs(1),
		GroupID: "worktree",
		RunE:    runCreate,
	}
	cmd.Flags().String("from", "", "git ref (branch, tag, or commit) to create the new branch from (default: HEAD)")
	cmd.Flags().Bool("from-remote-primary", false, "fetch the default remote's primary branch and create the new branch from it")
	cmd.Flags().Bool("reuse", false, "reuse existing worktree if one already exists for this phrase")
	cmd.MarkFlagsMutuallyExclusive("from", "from-remote-primary", "reuse")
	return cmd
}

type createContext struct {
	baseRef           string
	cfg               config.Config
	fromRemotePrimary bool
	gitClient         git.Git
	logger            *log.Logger
	reuse             bool
}

func runCreate(cmd *cobra.Command, args []string) error {
	phrase := args[0]

	fromRef, err := cmd.Flags().GetString("from")
	if err != nil {
		return err
	}

	reuse, err := cmd.Flags().GetBool("reuse")
	if err != nil {
		return err
	}

	fromRemotePrimary, err := cmd.Flags().GetBool("from-remote-primary")
	if err != nil {
		return err
	}

	rt, err := loadCommandRuntime(cmd)
	if err != nil {
		return err
	}

	ctx := &createContext{
		baseRef:           fromRef,
		cfg:               rt.cfg,
		fromRemotePrimary: fromRemotePrimary,
		gitClient:         rt.gitClient,
		logger:            rt.logger,
		reuse:             reuse,
	}

	return executeCreate(cmd.OutOrStdout(), ctx, phrase)
}

func executeCreate(stdout io.Writer, ctx *createContext, phrase string) error {
	if err := validateCreatePhrase(phrase); err != nil {
		return err
	}

	namer := naming.NewLocalBranchNamer(ctx.cfg.LocalBranch, ctx.cfg.Slugify)

	workspacePath, err := ctx.gitClient.GetWorkspacePath()
	if err != nil {
		return fmt.Errorf("failed to get workspace path: %w", err)
	}

	branchName := namer.GenerateBranchName(phrase)
	if branchName == "" || branchName == ctx.cfg.LocalBranch.BranchPrefix {
		return fmt.Errorf(`phrase %q produces an empty branch name after slugification

Please provide a phrase with at least one alphanumeric character.
Examples:
  grove create "add user auth"
  grove create "fix-bug-123"`, phrase)
	}

	exists, err := ctx.gitClient.BranchExists(branchName, false)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if exists {
		if !ctx.reuse {
			return fmt.Errorf("branch %q already exists; to use it: git worktree add <path> %s", branchName, branchName)
		}
		return reuseExistingBranch(stdout, ctx, namer, branchName, workspacePath)
	}

	worktreeName := namer.GenerateWorktreeName(branchName)
	worktreePath := filepath.Join(workspacePath, worktreeName)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path %q already exists; to remove it: git worktree remove %s", worktreePath, worktreeName)
	}

	baseRef, err := resolveCreateBaseRef(ctx)
	if err != nil {
		return err
	}

	if err := ctx.gitClient.CreateWorktreeForNewBranchFromRef(branchName, worktreePath, baseRef); err != nil {
		return fmt.Errorf("failed to create branch and worktree: %w", err)
	}

	_, err = fmt.Fprintln(stdout, worktreePath)
	return err
}

func validateCreatePhrase(phrase string) error {
	if strings.TrimSpace(phrase) == "" {
		return errors.New("phrase cannot be empty")
	}
	return nil
}

func resolveCreateBaseRef(ctx *createContext) (string, error) {
	if ctx.fromRemotePrimary {
		return resolveRemotePrimaryBaseRef(ctx.gitClient)
	}
	return ctx.baseRef, nil
}

func resolveRemotePrimaryBaseRef(gitClient git.Git) (string, error) {
	remoteName, err := gitClient.GetDefaultRemote("origin")
	if err != nil {
		return "", fmt.Errorf("failed to determine default remote: %w", err)
	}

	branchName, err := gitClient.GetRemoteDefaultBranch(remoteName)
	if err != nil {
		return "", fmt.Errorf("failed to determine default branch for remote %q: %w", remoteName, err)
	}
	if branchName == "" {
		return "", fmt.Errorf("could not determine default branch for remote %q; ensure the remote HEAD/default branch is configured", remoteName)
	}

	if err := gitClient.FetchRemoteTrackingBranch(remoteName, branchName); err != nil {
		return "", fmt.Errorf("failed to fetch default branch %q from remote %q: %w", branchName, remoteName, err)
	}

	return remoteName + "/" + branchName, nil
}

func reuseExistingBranch(stdout io.Writer, ctx *createContext, namer *naming.LocalBranchNamer, branchName, workspacePath string) error {
	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	foundStale := false
	for _, wt := range worktrees {
		if wt.Ref == nil {
			continue
		}
		if branch, ok := wt.Ref.FullBranch(); ok && branch.Name == branchName {
			if _, statErr := os.Stat(wt.AbsolutePath); statErr != nil {
				ctx.logger.WithPrefix("create").Warn("stale worktree entry found", "branch", branchName, "path", wt.AbsolutePath)
				foundStale = true
				continue
			}
			ctx.logger.WithPrefix("create").Warn("reusing existing worktree", "branch", branchName, "path", wt.AbsolutePath)
			_, err = fmt.Fprintln(stdout, wt.AbsolutePath)
			return err
		}
	}

	if foundStale {
		if err := ctx.gitClient.PruneWorktrees(); err != nil {
			return fmt.Errorf("failed to prune stale worktrees: %w", err)
		}
	}

	worktreeName := namer.GenerateWorktreeName(branchName)
	worktreePath := filepath.Join(workspacePath, worktreeName)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path %q already exists; to remove it: git worktree remove %s", worktreePath, worktreeName)
	}

	ctx.logger.WithPrefix("create").Warn("creating worktree for existing branch", "branch", branchName, "path", worktreePath)
	if err := ctx.gitClient.CreateWorktreeForExistingBranch(branchName, worktreePath); err != nil {
		return fmt.Errorf("failed to create worktree for existing branch: %w", err)
	}

	_, err = fmt.Fprintln(stdout, worktreePath)
	return err
}
