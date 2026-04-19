package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View grove log information",
	Long: `Grove writes file logs to the path configured by log.file. The default
follows the XDG Base Directory Specification:

  $XDG_STATE_HOME/grove/grove.log
  ~/.local/state/grove/grove.log   (fallback when XDG_STATE_HOME is unset)

File logging is disabled when log.file is the empty string, or when the
home directory cannot be determined. Pass --debug on any grove command
to raise the log level to debug for that invocation.

Subcommands:
  grove logs path   Print the log file path
  grove logs tail   Print the last lines of the log file`,
}

func init() {
	logsCmd.GroupID = "config"
	rootCmd.AddCommand(logsCmd)
}

func loadLogConfig() (config.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to get user home directory: %w", err)
	}

	defaultTimeout := config.DefaultConfig().Git.Timeout
	g := git.New(false, cwd, defaultTimeout)

	paths := config.BootstrapConfigPaths(cwd, homeDir)
	if root, gitErr := g.GetWorktreeRoot(); gitErr == nil && root != "" {
		if mainPath, gitErr := g.GetMainWorktreePath(); gitErr == nil {
			paths = config.ConfigPaths(cwd, root, mainPath, homeDir)
		} else {
			log.Debug("logs: failed to get main worktree path, using worktree root only", "err", gitErr)
			paths = config.ConfigPaths(cwd, root, root, homeDir)
		}
	} else {
		paths = resolveLogConfigPaths(cwd, homeDir, paths, defaultTimeout)
	}

	result, err := config.NewDefaultLoader().Load(paths)
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to load config: %w", err)
	}

	return result.Config, nil
}

func resolveLogConfigPaths(cwd, homeDir string, fallback []string, timeout time.Duration) []string {
	bootstrapResult, err := config.NewDefaultLoader().Load(fallback)
	if err != nil {
		log.Debug("logs: failed to load bootstrap config", "err", err)
		return fallback
	}

	wsRoot, err := resolveWorkspaceRoot(cwd, bootstrapResult.Config.Workspace.PrimaryBranches, timeout)
	if err != nil {
		log.Debug("logs: workspace root detection failed, using bootstrap config", "err", err)
		return fallback
	}

	wsGit := git.New(false, wsRoot, timeout)
	mainPath, err := wsGit.GetMainWorktreePath()
	if err != nil {
		log.Debug("logs: failed to get main worktree path from workspace root", "err", err)
		return fallback
	}

	return config.ConfigPaths(cwd, wsRoot, mainPath, homeDir)
}

func resolveLogPath() (string, error) {
	cfg, err := loadLogConfig()
	if err != nil {
		return "", err
	}

	p := cfg.Log.File
	if p == "" {
		return "", errors.New("file logging is disabled (log.file is empty)")
	}
	if !filepath.IsAbs(p) {
		p, err = filepath.Abs(p)
		if err != nil {
			return "", fmt.Errorf("failed to resolve relative log path: %w", err)
		}
	}
	return p, nil
}
