// Package repository holds the per-command runtime: the loaded configuration
// and the git client bound to one repository, plus the constructor for a
// GitHub client that shares its context, directory, and timeout.
package repository

import (
	"context"

	"charm.land/log/v2"
	"github.com/jmcampanini/go-config-loader/configloader"
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

// NewGitHubClient creates a GitHub client that shares the runtime's context,
// directory, and timeout.
func (rt *Runtime) NewGitHubClient() github.GitHub {
	return github.New(rt.Context, rt.Directory, rt.Config.Git.Timeout, rt.Logger)
}
