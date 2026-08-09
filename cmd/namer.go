package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/spf13/cobra"
)

func newNamerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "namer",
		Short:   "Generate branch and worktree names from a phrase",
		GroupID: "utility",
	}

	cmd.AddCommand(
		newNamerBranchCmd(),
		newNamerSlugCmd(),
		newNamerWorktreeCmd(),
	)

	return cmd
}

type namerContext struct {
	namer *naming.LocalBranchNamer
}

func loadNamingConfig(cmd *cobra.Command) (config.Config, error) {
	ctx := cmd.Context()
	logger := commandLogger(cmd)
	cwd, err := os.Getwd()
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to get user home directory: %w", err)
	}

	defaultTimeout := config.DefaultConfig().Git.Timeout
	g := git.New(ctx, false, cwd, defaultTimeout, logger)

	// Unlike loadCommandRuntime, git errors (timeout, corrupt index) are treated
	// the same as "not in a repo" so the namer can degrade gracefully to defaults.
	paths := config.BootstrapConfigPaths(cwd, homeDir)
	if root, gitErr := g.GetWorktreeRoot(); gitErr == nil && root != "" {
		if mainPath, gitErr := g.GetMainWorktreePath(); gitErr == nil {
			paths = config.ConfigPaths(cwd, root, mainPath, homeDir)
		} else {
			logger.Debug("namer: failed to get main worktree path, using worktree root only", "err", gitErr)
			paths = config.ConfigPaths(cwd, root, root, homeDir)
		}
	} else {
		logger.Debug("namer: not in a git repository, attempting workspace root detection", "cwd", cwd)
		paths = resolveNamerConfigPaths(ctx, logger, cwd, homeDir, paths, defaultTimeout)
	}

	cfg, _, err := config.LoadFilesWithFlags(paths, cmd.Root().PersistentFlags())
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}

func resolveNamerConfigPaths(ctx context.Context, logger *log.Logger, cwd, homeDir string, fallback []string, timeout time.Duration) []string {
	bootstrapCfg, _, err := config.LoadFiles(fallback)
	if err != nil {
		logger.Debug("namer: failed to load bootstrap config", "err", err)
		return fallback
	}

	wsRoot, err := resolveWorkspaceRoot(ctx, logger, cwd, bootstrapCfg.Workspace.PrimaryBranches, timeout)
	if err != nil {
		logger.Debug("namer: workspace root detection failed, using bootstrap config", "err", err)
		return fallback
	}

	wsGit := git.New(ctx, false, wsRoot, timeout, logger)
	mainPath, err := wsGit.GetMainWorktreePath()
	if err != nil {
		logger.Debug("namer: failed to get main worktree path from workspace root", "err", err)
		return fallback
	}

	return config.ConfigPaths(cwd, wsRoot, mainPath, homeDir)
}

func generateBranchName(ctx *namerContext, phrase string) (string, error) {
	if strings.TrimSpace(phrase) == "" {
		return "", errors.New("phrase cannot be empty")
	}

	name, err := ctx.namer.GenerateBranchName(phrase)
	if err != nil {
		return "", fmt.Errorf("failed to generate branch name for phrase %q: %w", phrase, err)
	}

	return name, nil
}
