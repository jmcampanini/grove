package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/naming"
	"github.com/jmcampanini/grove/internal/worktree"
	"github.com/spf13/cobra"
)

func newCreateCmd(services commandServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <phrase>",
		Short: "Create a new branch and worktree from a descriptive phrase",
		Long: `Create a git branch and worktree from one descriptive phrase (quote it)
and print the worktree path on stdout.

By default the new branch starts at the current HEAD. --from starts it at
any git ref: a branch, a tag, or a commit. --from-remote-primary fetches
the default remote and starts at its latest primary branch without
touching the primary worktree, which suits automation. --reuse attaches to
the branch the phrase names when it already exists, creating only the
worktree, instead of failing. A branch or worktree that already exists
fails the command otherwise, and a stale worktree entry found under
--reuse is pruned from git first.

Creation mode flags are mutually exclusive; choose at most one:
  --from <ref>
  --from-remote-primary
  --reuse

The branch is named by local_branch.branch_template and the directory by
local_branch.worktree_template; run grove help naming for the variables.
The global --worktree-template flag replaces the directory template for
one invocation. Nothing prompts.

` + helpWorktreeCreate + `

` + helpFailure + `

To check out an existing pull request, use grove pr checkout.`,
		Example: `  grove create "add user authentication"
  grove create "hotfix" --from main
  grove create "backport" --from v1.2.0
  grove create "experiment" --from origin/develop
  grove create "add user authentication" --from-remote-primary
  grove create "add user authentication" --reuse
  grove create "run tests" --space claude
  grove create "run tests" --worktree-template "subagent-{{.BranchSlug}}"`,
		Args:    phraseArgs("phrase"),
		GroupID: "worktree",
		RunE:    services.runCreate,
	}
	cmd.Flags().String("from", "", "Git ref to start the branch from (default HEAD)")
	cmd.Flags().Bool("from-remote-primary", false, "Start from the fetched remote primary branch")
	cmd.Flags().Bool("reuse", false, "Reuse the branch and worktree the phrase names")
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

func (services commandServices) runCreate(cmd *cobra.Command, args []string) error {
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

	rt, err := services.loadRuntime(cmd)
	if err != nil {
		return err
	}

	ctx := &createContext{
		baseRef:           fromRef,
		cfg:               rt.Config,
		fromRemotePrimary: fromRemotePrimary,
		gitClient:         rt.GitClient,
		logger:            rt.Logger,
		reuse:             reuse,
	}

	return executeCreate(cmd.OutOrStdout(), ctx, phrase)
}

func executeCreate(stdout io.Writer, ctx *createContext, phrase string) error {
	if err := validateCreatePhrase(phrase); err != nil {
		return err
	}

	namer, err := naming.NewLocalBranchNamer(ctx.cfg.LocalBranch, ctx.cfg.Naming)
	if err != nil {
		return fmt.Errorf("failed to initialize local branch namer: %w", err)
	}

	branchName, err := namer.GenerateBranchName(phrase)
	if errors.Is(err, naming.ErrEmptySlug) {
		return fmt.Errorf("phrase %q produces an empty branch name after slugification; use a phrase with at least one letter or digit, such as \"add user auth\"", phrase)
	}
	if err != nil {
		return fmt.Errorf("failed to generate branch name: %w", err)
	}

	exists, err := ctx.gitClient.BranchExists(branchName, false)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if exists {
		if !ctx.reuse {
			return fmt.Errorf("branch %q already exists; to use it: git worktree add <path> %s", branchName, branchName)
		}
		return reuseExistingBranch(stdout, ctx, namer, branchName)
	}

	worktreeName, err := namer.GenerateWorktreeName(branchName)
	if err != nil {
		return fmt.Errorf("failed to generate worktree name for branch %q: %w", branchName, err)
	}
	worktreePath, err := worktree.ResolvePath(ctx.cfg, ctx.gitClient, worktreeName)
	if err != nil {
		return err
	}

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
		return worktree.ResolveRemotePrimaryBaseRef(ctx.gitClient)
	}
	return ctx.baseRef, nil
}

func reuseExistingBranch(stdout io.Writer, ctx *createContext, namer *naming.LocalBranchNamer, branchName string) error {
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

	worktreeName, err := namer.GenerateWorktreeName(branchName)
	if err != nil {
		return fmt.Errorf("failed to generate worktree name for branch %q: %w", branchName, err)
	}
	worktreePath, err := worktree.ResolvePath(ctx.cfg, ctx.gitClient, worktreeName)
	if err != nil {
		return err
	}

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
