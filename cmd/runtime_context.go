package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"charm.land/log/v2"
	"github.com/jmcampanini/go-config-loader/configloader"

	"github.com/jmcampanini/grove/internal/cache"
	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/spf13/cobra"
)

var errNotGitRepo = errors.New("grove must be run inside a git repository")

type commandRuntime struct {
	cfg              config.Config
	configReport     configloader.LoadReport
	ctx              context.Context
	cwd              string
	gitClient        git.Git
	logger           *log.Logger
	mainWorktreePath string
}

func (rt *commandRuntime) newUncachedGitHubClient() github.GitHub {
	return github.New(rt.ctx, rt.cwd, rt.cfg.Git.Timeout, nil, rt.logger)
}

func (rt *commandRuntime) newCachedGitHubClient() (github.GitHub, error) {
	dir, err := cache.DefaultDir()
	if err != nil {
		return nil, err
	}
	c := cache.New(dir, rt.cfg.GitHub.PreviewCacheTTL, rt.logger)
	return github.New(rt.ctx, rt.cwd, rt.cfg.Git.Timeout, c, rt.logger), nil
}

func loadCommandRuntime(cmd *cobra.Command) (*commandRuntime, error) {
	ctx := cmd.Context()
	logger := commandLogger(cmd)
	originalCwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	defaultTimeout := config.DefaultConfig().Git.Timeout
	gitDir := originalCwd
	initGit := git.New(ctx, false, gitDir, defaultTimeout, logger)

	worktreeRoot, err := initGit.GetWorktreeRoot()
	if err != nil {
		return nil, fmt.Errorf("git error: %w", err)
	}

	if worktreeRoot == "" {
		logger.Debug("not in a git repository, attempting workspace root detection", "cwd", originalCwd)

		bootstrapPaths := config.BootstrapConfigPaths(originalCwd, homeDir)
		bootstrapCfg, bootstrapReport, err := config.LoadFiles(bootstrapPaths)
		if err != nil {
			return nil, fmt.Errorf("failed to load bootstrap config: %w", err)
		}
		logger.Debug("bootstrap config loaded", "paths", bootstrapPaths, "sources", bootstrapReport.LoadedFiles)

		worktreeRoot, err = resolveWorkspaceRoot(ctx, logger, originalCwd, bootstrapCfg.Workspace.PrimaryBranches, defaultTimeout)
		if err != nil {
			logger.Debug("workspace root detection failed", "err", err)
			return nil, errNotGitRepo
		}

		gitDir = worktreeRoot
		initGit = git.New(ctx, false, gitDir, defaultTimeout, logger)
		logger.Debug("anchored to worktree from workspace root", "anchor", gitDir, "originalCwd", originalCwd)
	}

	mainWorktreePath, err := initGit.GetMainWorktreePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get main worktree path: %w", err)
	}

	configPaths := config.ConfigPaths(originalCwd, worktreeRoot, mainWorktreePath, homeDir)
	cfg, report, err := config.LoadFilesWithFlags(configPaths, cmd.Root().PersistentFlags())
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	logger.Debug("config loaded", "paths", configPaths, "sources", report.LoadedFiles)

	return &commandRuntime{
		cfg:              cfg,
		configReport:     report,
		ctx:              ctx,
		cwd:              gitDir,
		gitClient:        git.New(ctx, false, gitDir, cfg.Git.Timeout, logger),
		logger:           logger,
		mainWorktreePath: mainWorktreePath,
	}, nil
}

// resolveWorkspaceRoot probes immediate children of cwd for a valid git worktree.
// Returns the worktree root of the first child directory that has a .git marker and is a valid git repo.
func resolveWorkspaceRoot(ctx context.Context, logger *log.Logger, cwd string, primaryBranches []string, timeout time.Duration) (string, error) {
	if len(primaryBranches) == 0 {
		return "", errors.New("no primary branches configured")
	}

	logger.Debug("probing for primary worktree", "candidates", primaryBranches)

	for _, name := range primaryBranches {
		candidate := filepath.Join(cwd, name)
		if !hasGitMarker(candidate) {
			continue
		}

		testGit := git.New(ctx, false, candidate, timeout, logger)
		testRoot, err := testGit.GetWorktreeRoot()
		if err != nil {
			logger.Debug("candidate git error", "dir", candidate, "err", err)
			continue
		}
		if testRoot == "" {
			logger.Debug("candidate has .git marker but is not a valid repo", "dir", candidate)
			continue
		}

		logger.Debug("found valid worktree", "dir", candidate)
		return testRoot, nil
	}

	return "", errors.New("no valid worktree found in workspace root")
}

// hasGitMarker checks if a directory contains a .git file or directory.
func hasGitMarker(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
