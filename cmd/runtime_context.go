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
	logger := commandLogger(cmd)
	originalCwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	loaded, err := loadConfigAt(cmd, logger, originalCwd, requireRuntime)
	if err != nil {
		return nil, err
	}

	ctx := cmd.Context()
	return &commandRuntime{
		cfg:              loaded.cfg,
		configReport:     loaded.report,
		ctx:              ctx,
		cwd:              loaded.gitDir,
		gitClient:        git.New(ctx, false, loaded.gitDir, loaded.cfg.Git.Timeout, logger),
		logger:           logger,
		mainWorktreePath: loaded.mainWorktreePath,
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
