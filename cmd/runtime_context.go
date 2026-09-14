package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

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
