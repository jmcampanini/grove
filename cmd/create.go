package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <phrase>",
	Short: "Create a new branch and worktree from a descriptive phrase",
	Long: `Create creates a new git branch and worktree from a descriptive phrase.

By default, the new branch is created from the current HEAD. Use --from to
specify a different starting point (any git ref: branch, tag, or commit SHA).

The phrase is converted to a branch name using the configured slugify rules
and prefix. A worktree is then created with the configured worktree naming.

Example:
  grove create "add user authentication"
  grove create "fix bug in login"
  grove create "hotfix" --from main
  grove create "backport" --from v1.2.0
  grove create "experiment" --from origin/develop

Note: The create command takes a single quoted string argument. The shell wrapper
function (grc) can handle passing arbitrary phrases by quoting the arguments.

To check out an existing pull request, use 'grove pr checkout' instead.`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().String("from", "", "git ref (branch, tag, or commit) to create the new branch from (default: HEAD)")
	createCmd.Flags().Bool("reuse", false, "reuse existing worktree if one already exists for this phrase")
	createCmd.GroupID = "worktree"
	rootCmd.AddCommand(createCmd)
}

type createContext struct {
	baseRef   string
	cfg       config.Config
	gitClient git.Git
	reuse     bool
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

	rt, err := loadCommandRuntime()
	if err != nil {
		return err
	}

	ctx := &createContext{
		baseRef:   fromRef,
		cfg:       rt.cfg,
		gitClient: rt.gitClient,
		reuse:     reuse,
	}

	return executeCreate(cmd.OutOrStdout(), ctx, phrase)
}

func executeCreate(stdout io.Writer, ctx *createContext, phrase string) error {
	if strings.TrimSpace(phrase) == "" {
		return errors.New("phrase cannot be empty")
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

		worktrees, err := ctx.gitClient.ListWorktrees()
		if err != nil {
			return fmt.Errorf("failed to list worktrees: %w", err)
		}

		for _, wt := range worktrees {
			if branch, ok := wt.Ref.FullBranch(); ok && branch.Name == branchName {
				log.WithPrefix("create").Warn("reusing existing worktree", "branch", branchName, "path", wt.AbsolutePath)
				_, err = fmt.Fprintln(stdout, wt.AbsolutePath)
				return err
			}
		}

		return fmt.Errorf("branch %q exists but has no worktree", branchName)
	}

	worktreeName := namer.GenerateWorktreeName(branchName)
	worktreePath := filepath.Join(workspacePath, worktreeName)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path %q already exists; to remove it: git worktree remove %s", worktreePath, worktreeName)
	}

	if err := ctx.gitClient.CreateWorktreeForNewBranchFromRef(branchName, worktreePath, ctx.baseRef); err != nil {
		return fmt.Errorf("failed to create branch and worktree: %w", err)
	}

	_, err = fmt.Fprintln(stdout, worktreePath)
	return err
}
