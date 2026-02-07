package cmd

import (
	"errors"
	"fmt"
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

func runCreate(cmd *cobra.Command, args []string) error {
	phrase := args[0]

	if strings.TrimSpace(phrase) == "" {
		return errors.New("phrase cannot be empty")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	gitClient := git.New(false, cwd, config.DefaultConfig().Git.Timeout)

	worktreeRoot, err := gitClient.GetWorktreeRoot()
	if err != nil {
		return fmt.Errorf("git error: %w", err)
	}
	if worktreeRoot == "" {
		return errors.New("grove must be run inside a git repository")
	}

	mainWorktreePath, err := gitClient.GetMainWorktreePath()
	if err != nil {
		return fmt.Errorf("failed to get main worktree path: %w", err)
	}

	configPaths := config.ConfigPaths(cwd, worktreeRoot, mainWorktreePath, homeDir)
	loader := config.NewDefaultLoader()
	loadResult, err := loader.Load(configPaths)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := loadResult.Config

	// recreate the git client using the config timeout
	gitClient = git.New(false, cwd, cfg.Git.Timeout)

	branchGen := naming.NewBranchNameGenerator(cfg.Branch, cfg.Slugify)
	branchName := branchGen.Generate(phrase)

	if branchName == "" || branchName == cfg.Branch.NewPrefix {
		return fmt.Errorf(`phrase %q produces an empty branch name after slugification

Please provide a phrase with at least one alphanumeric character.
Examples:
  grove create "add user auth"
  grove create "fix-bug-123"`, phrase)
	}

	exists, err := gitClient.BranchExists(branchName, false)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if exists {
		return fmt.Errorf("branch %q already exists; to use it: git worktree add <path> %s", branchName, branchName)
	}

	worktreeNamer := naming.NewLocalBranchNamer(cfg.LocalBranch, cfg.Slugify)
	worktreeName := worktreeNamer.Generate(branchName)

	workspacePath, err := gitClient.GetWorkspacePath()
	if err != nil {
		return fmt.Errorf("failed to get workspace path: %w", err)
	}
	worktreePath := filepath.Join(workspacePath, worktreeName)

	if _, err := os.Stat(worktreePath); err == nil {
		return fmt.Errorf("worktree path %q already exists; to remove it: git worktree remove %s", worktreePath, worktreeName)
	}

	if err := gitClient.CreateWorktreeForNewBranchFromRef(branchName, worktreePath, ""); err != nil {
		return fmt.Errorf("failed to create branch and worktree: %w", err)
	}

	_, err = fmt.Fprintln(cmd.OutOrStdout(), worktreePath)
	return err
}
