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

type commandRuntime struct {
	cfg              config.Config
	cwd              string
	gitClient        git.Git
	mainWorktreePath string
}

func (rt *commandRuntime) newGitHubClient() github.GitHub {
	return github.New(rt.cwd, rt.cfg.Git.Timeout)
}

func loadCommandRuntime() (*commandRuntime, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	initGit := git.New(false, cwd, config.DefaultConfig().Git.Timeout)

	worktreeRoot, err := initGit.GetWorktreeRoot()
	if err != nil {
		return nil, fmt.Errorf("git error: %w", err)
	}
	if worktreeRoot == "" {
		return nil, errNotGitRepo
	}

	mainWorktreePath, err := initGit.GetMainWorktreePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get main worktree path: %w", err)
	}

	configPaths := config.ConfigPaths(cwd, worktreeRoot, mainWorktreePath, homeDir)
	loadResult, err := config.NewDefaultLoader().Load(configPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &commandRuntime{
		cfg:              loadResult.Config,
		cwd:              cwd,
		gitClient:        git.New(false, cwd, loadResult.Config.Git.Timeout),
		mainWorktreePath: mainWorktreePath,
	}, nil
}
