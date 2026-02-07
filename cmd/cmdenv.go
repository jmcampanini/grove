package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
)

var errNotGitRepo = errors.New("grove must be run inside a git repository")

type cmdenv struct {
	cfg              config.Config
	ghClient         github.GitHub
	gitClient        git.Git
	mainWorktreePath string
}

func initFromEnv() (*cmdenv, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	gitClient := git.New(false, cwd, config.DefaultConfig().Git.Timeout)

	worktreeRoot, err := gitClient.GetWorktreeRoot()
	if err != nil {
		return nil, fmt.Errorf("git error: %w", err)
	}
	if worktreeRoot == "" {
		return nil, errNotGitRepo
	}

	mainWorktreePath, err := gitClient.GetMainWorktreePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get main worktree path: %w", err)
	}

	configPaths := config.ConfigPaths(cwd, worktreeRoot, mainWorktreePath, homeDir)
	loadResult, err := config.NewDefaultLoader().Load(configPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	cfg := loadResult.Config
	return &cmdenv{
		cfg:              cfg,
		ghClient:         github.New(cwd, cfg.Git.Timeout),
		gitClient:        git.New(false, cwd, cfg.Git.Timeout),
		mainWorktreePath: mainWorktreePath,
	}, nil
}
