package cmd

import (
	"fmt"
	"os"

	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/layout"
)

// resolveWorktreePath renders the absolute path for a new worktree named
// name: worktree.root joined with worktree.layout rendered from the
// configured space and, when the layout uses them, the host, owner, and
// repository of the default remote.
func resolveWorktreePath(cfg config.Config, gitClient git.Git, name string) (string, error) {
	// An unknown home directory is "" here; layout.New reports it only when
	// worktree.root starts with "~".
	homeDir, _ := os.UserHomeDir()
	resolver, err := layout.New(cfg.Worktree, os.LookupEnv, homeDir)
	if err != nil {
		return "", err
	}

	var remote layout.Remote
	if resolver.NeedsRemote() {
		remoteName, err := gitClient.GetDefaultRemote("origin")
		if err != nil {
			return "", fmt.Errorf("failed to determine remote: %w", err)
		}
		remoteURL, err := gitClient.GetRemoteURL(remoteName)
		if err != nil {
			return "", fmt.Errorf("worktree.layout needs the remote's host, owner, and repository: %w", err)
		}
		remote, err = layout.ParseRemoteURL(remoteURL)
		if err != nil {
			return "", fmt.Errorf("worktree.layout needs the remote's host, owner, and repository: %w", err)
		}
	}

	path, err := resolver.Path(remote, cfg.Worktree.Space, name)
	if err != nil {
		return "", fmt.Errorf("failed to resolve worktree path: %w", err)
	}
	return path, nil
}
