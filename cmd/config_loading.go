package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
)

// configLoadMode selects how config orchestration treats missing or broken
// git context at the anchor directory.
type configLoadMode int

const (
	// requireRuntime fails when the anchor is not inside a repository or
	// workspace, matching the contract of commands that mutate git state.
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
	worktreeRoot     string // empty when no repository context was found
}

// loadConfigAt is the single config orchestration path. It resolves git
// context at the anchor directory (falling back to workspace-root probing),
// discovers config file candidates, and loads defaults, files, and root
// persistent flags in that precedence order.
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

	defaultTimeout := config.DefaultConfig().Git.Timeout
	anchorGit := git.New(ctx, false, anchor, defaultTimeout, logger)

	worktreeRoot, err := anchorGit.GetWorktreeRoot()
	if err != nil {
		if mode == requireRuntime {
			return nil, fmt.Errorf("git error: %w", err)
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
		return finishConfigLoad(logger, flags, anchor, anchor, worktreeRoot, mainWorktreePath, homeDir)
	}

	logger.Debug("not in a git repository, attempting workspace root detection", "cwd", anchor)

	bootstrapPaths := config.BootstrapConfigPaths(anchor, homeDir)
	bootstrapCfg, bootstrapReport, err := config.Load(bootstrapPaths, flags)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	logger.Debug("bootstrap config loaded", "paths", bootstrapPaths, "sources", bootstrapReport.LoadedFiles)
	bootstrapResult := &loadedConfig{cfg: bootstrapCfg, gitDir: anchor, report: bootstrapReport}

	workspaceRoot, err := resolveWorkspaceRoot(ctx, logger, anchor, bootstrapCfg.Workspace.PrimaryBranches, defaultTimeout)
	if err != nil {
		if mode == requireRuntime {
			logger.Debug("workspace root detection failed", "err", err)
			return nil, errNotGitRepo
		}
		logger.Debug("workspace root detection failed, using bootstrap config", "err", err)
		return bootstrapResult, nil
	}

	workspaceGit := git.New(ctx, false, workspaceRoot, defaultTimeout, logger)
	mainWorktreePath, err := workspaceGit.GetMainWorktreePath()
	if err != nil {
		if mode == requireRuntime {
			return nil, fmt.Errorf("failed to get main worktree path: %w", err)
		}
		logger.Debug("failed to get main worktree path from workspace root, using bootstrap config", "err", err)
		return bootstrapResult, nil
	}

	logger.Debug("anchored to worktree from workspace root", "anchor", workspaceRoot, "originalCwd", anchor)
	return finishConfigLoad(logger, flags, anchor, workspaceRoot, workspaceRoot, mainWorktreePath, homeDir)
}

func finishConfigLoad(logger *log.Logger, flags *pflag.FlagSet, anchor, gitDir, worktreeRoot, mainWorktreePath, homeDir string) (*loadedConfig, error) {
	paths := config.ConfigPaths(anchor, worktreeRoot, mainWorktreePath, homeDir)
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
		worktreeRoot:     worktreeRoot,
	}, nil
}

// loadReportingConfig resolves effective configuration for reporting and
// naming commands: same discovery and precedence as loadCommandRuntime, but
// it degrades to bootstrap discovery instead of failing when the anchor is
// not inside a repository or workspace.
func loadReportingConfig(cmd *cobra.Command, anchor string) (config.Config, configloader.LoadReport, error) {
	loaded, err := loadConfigAt(cmd, commandLogger(cmd), anchor, reportGracefully)
	if err != nil {
		return config.Config{}, configloader.LoadReport{}, err
	}
	return loaded.cfg, loaded.report, nil
}
