// Package repository holds the per-command runtime: the loaded configuration
// and the git and GitHub clients bound to one repository.
package repository

import (
	"context"

	"charm.land/log/v2"
	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/grove/internal/cache"
	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
)

// Runtime holds configuration and clients for one repository command execution.
type Runtime struct {
	Config           config.Config
	ConfigReport     configloader.LoadReport
	Context          context.Context
	Directory        string
	GitClient        git.Git
	Logger           *log.Logger
	MainWorktreePath string
}

// NewCachedGitHubClient creates a GitHub client using the configured preview cache.
func (rt *Runtime) NewCachedGitHubClient() (github.GitHub, error) {
	dir, err := cache.DefaultDir()
	if err != nil {
		return nil, err
	}
	c := cache.New(dir, rt.Config.GitHub.PreviewCacheTTL, rt.Logger)
	return github.New(rt.Context, rt.Directory, rt.Config.Git.Timeout, c, rt.Logger), nil
}

// NewUncachedGitHubClient creates a GitHub client without a cache.
func (rt *Runtime) NewUncachedGitHubClient() github.GitHub {
	return github.New(rt.Context, rt.Directory, rt.Config.Git.Timeout, nil, rt.Logger)
}
