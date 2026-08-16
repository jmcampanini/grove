package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/spf13/cobra"
)

func newResolveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resolve [path]",
		Short: "Print the absolute path of the primary worktree",
		Long: `Resolve and print the absolute path of the primary worktree for a workspace.

The path argument can be:
  - A worktree directory (returns the primary worktree for that workspace)
  - A workspace directory (parent of worktrees; discovers the primary among children)
  - Omitted (defaults to the current directory)`,
		Args:    cobra.MaximumNArgs(1),
		RunE:    runResolve,
		GroupID: "utility",
	}
}

type resolveContext struct {
	ctx             context.Context
	logger          *log.Logger
	primaryBranches []string
	timeout         time.Duration
}

func runResolve(cmd *cobra.Command, args []string) error {
	targetPath, err := resolveTargetPath(args)
	if err != nil {
		return err
	}

	cfg, _, err := loadReportingConfig(cmd, targetPath)
	if err != nil {
		return err
	}

	ctx := &resolveContext{
		ctx:             cmd.Context(),
		logger:          commandLogger(cmd),
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

	g := git.New(ctx.ctx, false, gitDir, ctx.timeout, ctx.logger)
	mainPath, err := g.GetMainWorktreePath()
	if err != nil {
		return fmt.Errorf("failed to resolve primary worktree: %w", err)
	}

	_, err = fmt.Fprintln(w, mainPath)
	return err
}

func resolveGitDir(targetPath string, ctx *resolveContext) (string, error) {
	g := git.New(ctx.ctx, false, targetPath, ctx.timeout, ctx.logger)
	worktreeRoot, err := g.GetWorktreeRoot()
	if err != nil {
		return "", fmt.Errorf("git error: %w", err)
	}

	if worktreeRoot != "" {
		ctx.logger.Debug("resolve: path is inside a worktree", "worktreeRoot", worktreeRoot)
		return worktreeRoot, nil
	}

	ctx.logger.Debug("resolve: path is not a worktree, trying workspace discovery", "path", targetPath)
	wsRoot, err := resolveWorkspaceRoot(ctx.ctx, ctx.logger, targetPath, ctx.primaryBranches, ctx.timeout)
	if err != nil {
		return "", fmt.Errorf("%s is not a grove workspace or worktree: %w", targetPath, err)
	}
	return wsRoot, nil
}

func resolveTargetPath(args []string) (string, error) {
	if len(args) > 0 {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return "", fmt.Errorf("failed to resolve path %q: %w", args[0], err)
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	return cwd, nil
}
