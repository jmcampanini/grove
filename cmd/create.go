package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <phrase>",
	Short: "Create a new branch and worktree",
	Long: `Create creates a new git branch and worktree from a descriptive phrase.

The new branch is created from the current HEAD (the commit you're currently on).
The phrase is converted to a branch name using the configured slugify rules
and prefix. A worktree is then created with the configured worktree naming.

Example:
  grove create "add user authentication"
  grove create "fix bug in login"

Note: The create command takes a single quoted string argument. The shell wrapper
function (grc) can handle passing arbitrary phrases by quoting the arguments.`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)
}

type createContext struct {
	cfg       config.Config
	gitClient git.Git
}

func runCreate(cmd *cobra.Command, args []string) error {
	phrase := args[0]

	rt, err := loadCommandRuntime()
	if err != nil {
		return err
	}

	ctx := &createContext{
		cfg:       rt.cfg,
		gitClient: rt.gitClient,
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
		return fmt.Errorf("branch %q already exists; to use it: git worktree add <path> %s", branchName, branchName)
	}

	worktreeName := namer.GenerateWorktreeName(branchName)
	worktreePath := filepath.Join(workspacePath, worktreeName)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path %q already exists; to remove it: git worktree remove %s", worktreePath, worktreeName)
	}

	if err := ctx.gitClient.CreateWorktreeForNewBranchFromRef(branchName, worktreePath, ""); err != nil {
		return fmt.Errorf("failed to create branch and worktree: %w", err)
	}

	_, err = fmt.Fprintln(stdout, worktreePath)
	return err
}
