package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/spf13/cobra"
)

var resolveCmd = &cobra.Command{
	Use:   "resolve [path]",
	Short: "Print the absolute path of the primary worktree",
	Long: `Resolve and print the absolute path of the primary worktree for a workspace.

The path argument can be:
  - A worktree directory (returns the primary worktree for that workspace)
  - A workspace directory (parent of worktrees; discovers the primary among children)
  - Omitted (defaults to the current directory)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runResolve,
}

func init() {
	resolveCmd.GroupID = "utility"
	rootCmd.AddCommand(resolveCmd)
}

type resolveContext struct {
	primaryBranches []string
	timeout         time.Duration
}

func runResolve(cmd *cobra.Command, args []string) error {
	targetPath, err := resolveTargetPath(args)
	if err != nil {
		return err
	}

	cfg := config.DefaultConfig()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	if homeDir != "" {
		paths := config.BootstrapConfigPaths(targetPath, homeDir)
		result, loadErr := config.NewDefaultLoader().Load(paths)
		if loadErr != nil {
			return fmt.Errorf("failed to load config: %w", loadErr)
		}
		cfg = result.Config
	}

	ctx := &resolveContext{
		primaryBranches: cfg.Workspace.PrimaryBranches,
		timeout:         cfg.Git.Timeout,
	}

	return executeResolve(cmd.OutOrStdout(), targetPath, ctx)
}

func executeResolve(w io.Writer, targetPath string, ctx *resolveContext) error {
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", targetPath)
		}
		return fmt.Errorf("cannot access path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", targetPath)
	}

	gitDir, err := resolveGitDir(targetPath, ctx)
	if err != nil {
		return err
	}

	g := git.New(false, gitDir, ctx.timeout)
	mainPath, err := g.GetMainWorktreePath()
	if err != nil {
		return fmt.Errorf("failed to resolve primary worktree: %w", err)
	}

	_, err = fmt.Fprintln(w, mainPath)
	return err
}

func resolveGitDir(targetPath string, ctx *resolveContext) (string, error) {
	g := git.New(false, targetPath, ctx.timeout)
	worktreeRoot, err := g.GetWorktreeRoot()
	if err != nil {
		return "", fmt.Errorf("git error: %w", err)
	}

	if worktreeRoot != "" {
		log.Debug("resolve: path is inside a worktree", "worktreeRoot", worktreeRoot)
		return worktreeRoot, nil
	}

	log.Debug("resolve: path is not a worktree, trying workspace discovery", "path", targetPath)
	wsRoot, err := resolveWorkspaceRoot(targetPath, ctx.primaryBranches, ctx.timeout)
	if err != nil {
		return "", fmt.Errorf("%s is not a grove workspace or worktree: %w", targetPath, err)
	}
	return wsRoot, nil
}

func resolveTargetPath(args []string) (string, error) {
	if len(args) > 0 {
		return filepath.Abs(args[0])
	}
	return os.Getwd()
}
