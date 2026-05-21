package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	configloader "github.com/jmcampanini/go-config-loader"

	"github.com/jmcampanini/grove-cli/internal/cache"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/logging"
)

var errNotGitRepo = errors.New("grove must be run inside a git repository")

type commandRuntime struct {
	cfg              config.Config
	configReport     configloader.LoadReport
	cwd              string
	gitClient        git.Git
	mainWorktreePath string
}

func (rt *commandRuntime) newUncachedGitHubClient() github.GitHub {
	return github.New(rt.cwd, rt.cfg.Git.Timeout, nil)
}

func (rt *commandRuntime) newCachedGitHubClient() (github.GitHub, error) {
	dir, err := cache.DefaultDir()
	if err != nil {
		return nil, err
	}
	c := cache.New(dir, rt.cfg.GitHub.PreviewCacheTTL)
	return github.New(rt.cwd, rt.cfg.Git.Timeout, c), nil
}

func loadCommandRuntime() (*commandRuntime, error) {
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
	initGit := git.New(false, gitDir, defaultTimeout)

	worktreeRoot, err := initGit.GetWorktreeRoot()
	if err != nil {
		return nil, fmt.Errorf("git error: %w", err)
	}

	if worktreeRoot == "" {
		log.Debug("not in a git repository, attempting workspace root detection", "cwd", originalCwd)

		bootstrapPaths := config.BootstrapConfigPaths(originalCwd, homeDir)
		bootstrapCfg, bootstrapReport, err := config.LoadFiles(bootstrapPaths)
		if err != nil {
			return nil, fmt.Errorf("failed to load bootstrap config: %w", err)
		}
		log.Debug("bootstrap config loaded", "paths", bootstrapPaths, "sources", bootstrapReport.LoadedFiles)

		worktreeRoot, err = resolveWorkspaceRoot(originalCwd, bootstrapCfg.Workspace.PrimaryBranches, defaultTimeout)
		if err != nil {
			log.Debug("workspace root detection failed", "err", err)
			return nil, errNotGitRepo
		}

		gitDir = worktreeRoot
		initGit = git.New(false, gitDir, defaultTimeout)
		log.Debug("anchored to worktree from workspace root", "anchor", gitDir, "originalCwd", originalCwd)
	}

	mainWorktreePath, err := initGit.GetMainWorktreePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get main worktree path: %w", err)
	}

	configPaths := config.ConfigPaths(originalCwd, worktreeRoot, mainWorktreePath, homeDir)
	cfg, report, err := config.LoadFiles(configPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	log.Debug("config loaded", "paths", configPaths, "sources", report.LoadedFiles)

	if err := logging.Setup(cfg.Log.File); err != nil {
		log.Warn("failed to set up file logging", "path", cfg.Log.File, "error", err)
	}

	return &commandRuntime{
		cfg:              cfg,
		configReport:     report,
		cwd:              gitDir,
		gitClient:        git.New(false, gitDir, cfg.Git.Timeout),
		mainWorktreePath: mainWorktreePath,
	}, nil
}

// resolveWorkspaceRoot probes immediate children of cwd for a valid git worktree.
// Returns the worktree root of the first child directory that has a .git marker and is a valid git repo.
func resolveWorkspaceRoot(cwd string, primaryBranches []string, timeout time.Duration) (string, error) {
	if len(primaryBranches) == 0 {
		return "", errors.New("no primary branches configured")
	}

	log.Debug("probing for primary worktree", "candidates", primaryBranches)

	for _, name := range primaryBranches {
		candidate := filepath.Join(cwd, name)
		if !hasGitMarker(candidate) {
			continue
		}

		testGit := git.New(false, candidate, timeout)
		testRoot, err := testGit.GetWorktreeRoot()
		if err != nil {
			log.Debug("candidate git error", "dir", candidate, "err", err)
			continue
		}
		if testRoot == "" {
			log.Debug("candidate has .git marker but is not a valid repo", "dir", candidate)
			continue
		}

		log.Debug("found valid worktree", "dir", candidate)
		return testRoot, nil
	}

	return "", errors.New("no valid worktree found in workspace root")
}

// hasGitMarker checks if a directory contains a .git file or directory.
func hasGitMarker(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
