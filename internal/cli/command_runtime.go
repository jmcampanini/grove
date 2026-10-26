package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/repository"
	"github.com/spf13/cobra"
)

var errNotGitRepo = errors.New("grove must be run inside a git repository")

// configLoadMode selects how config orchestration treats missing or broken
// git context at the anchor directory.
type configLoadMode int

const (
	// requireRuntime fails when the anchor is not inside a repository,
	// matching the contract of commands that mutate git state.
	requireRuntime configLoadMode = iota
	// reportGracefully falls back to bootstrap discovery when git context is
	// unavailable, so reporting and naming commands work anywhere. It never
	// mutates state and performs only read-only git queries.
	reportGracefully
)

// loadedConfig is the result of one config orchestration pass.
type loadedConfig struct {
	cfg              config.Config
	gitDir           string // anchor for later git operations
	mainWorktreePath string // empty when no repository context was found
	report           configloader.LoadReport
}

// loadConfigAt is the single config orchestration path. It resolves git
// context at the anchor directory, discovers config file candidates, and
// loads defaults, files, and root persistent flags in that precedence order.
func loadConfigAt(cmd *cobra.Command, logger *log.Logger, anchor string, mode configLoadMode) (*loadedConfig, error) {
	ctx := cmd.Context()
	flags := cmd.Root().PersistentFlags()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	if xdgConfigDir := os.Getenv("XDG_CONFIG_HOME"); xdgConfigDir != "" && !filepath.IsAbs(xdgConfigDir) {
		logger.Debug("ignoring relative XDG_CONFIG_HOME, using default config home", "value", xdgConfigDir)
	}

	loadForWorktree := func(gitDir, worktreeRoot, mainWorktreePath string) (*loadedConfig, error) {
		paths := config.Paths(anchor, worktreeRoot, mainWorktreePath, homeDir)
		cfg, report, err := config.Load(paths, flags)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		logger.Debug("config loaded", "paths", paths, "sources", report.LoadedFiles)

		return &loadedConfig{
			cfg:              cfg,
			gitDir:           gitDir,
			mainWorktreePath: mainWorktreePath,
			report:           report,
		}, nil
	}

	defaultTimeout := config.DefaultConfig().Git.Timeout
	anchorGit := git.New(ctx, false, anchor, defaultTimeout, logger)

	worktreeRoot, err := anchorGit.GetWorktreeRoot()
	if err != nil {
		if mode == requireRuntime {
			return nil, fmt.Errorf("find worktree root: %w", err)
		}
		logger.Debug("git error during config discovery, treating as no repository", "err", err)
		worktreeRoot = ""
	}

	if worktreeRoot != "" {
		mainWorktreePath, err := anchorGit.GetMainWorktreePath()
		if err != nil {
			if mode == requireRuntime {
				return nil, fmt.Errorf("failed to get main worktree path: %w", err)
			}
			logger.Debug("failed to get main worktree path, using worktree root only", "err", err)
			mainWorktreePath = worktreeRoot
		}
		return loadForWorktree(anchor, worktreeRoot, mainWorktreePath)
	}

	if mode == requireRuntime {
		logger.Debug("not in a git repository", "cwd", anchor)
		return nil, errNotGitRepo
	}

	logger.Debug("not in a git repository, using bootstrap config discovery", "cwd", anchor)
	bootstrapPaths := config.BootstrapConfigPaths(anchor, homeDir)
	bootstrapCfg, bootstrapReport, err := config.Load(bootstrapPaths, flags)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	logger.Debug("bootstrap config loaded", "paths", bootstrapPaths, "sources", bootstrapReport.LoadedFiles)
	return &loadedConfig{cfg: bootstrapCfg, gitDir: anchor, report: bootstrapReport}, nil
}

// loadReportingConfig resolves effective configuration for reporting and
// naming commands: same discovery and precedence as loadCommandRuntime, but
// it degrades to bootstrap discovery instead of failing when the anchor is
// not inside a repository.
func loadReportingConfig(cmd *cobra.Command, anchor string) (config.Config, configloader.LoadReport, error) {
	loaded, err := loadConfigAt(cmd, commandLogger(cmd), anchor, reportGracefully)
	if err != nil {
		return config.Config{}, configloader.LoadReport{}, err
	}
	return loaded.cfg, loaded.report, nil
}

func loadCommandRuntime(cmd *cobra.Command) (*repository.Runtime, error) {
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
	return &repository.Runtime{
		Config:           loaded.cfg,
		ConfigReport:     loaded.report,
		Context:          ctx,
		Directory:        loaded.gitDir,
		GitClient:        git.New(ctx, false, loaded.gitDir, loaded.cfg.Git.Timeout, logger),
		Logger:           logger,
		MainWorktreePath: loaded.mainWorktreePath,
	}, nil
}
