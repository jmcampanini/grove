package worktree

import (
	"fmt"

	"github.com/jmcampanini/grove/internal/git"
)

// ResolveRemotePrimaryBaseRef fetches the default remote branch and returns its remote-qualified name.
func ResolveRemotePrimaryBaseRef(gitClient git.Git) (string, error) {
	remoteName, err := gitClient.GetDefaultRemote("origin")
	if err != nil {
		return "", fmt.Errorf("failed to determine default remote: %w", err)
	}

	branchName, err := gitClient.GetRemoteDefaultBranch(remoteName)
	if err != nil {
		return "", fmt.Errorf("failed to determine default branch for remote %q: %w", remoteName, err)
	}
	if branchName == "" {
		return "", fmt.Errorf("could not determine default branch for remote %q; ensure the remote HEAD/default branch is configured", remoteName)
	}

	if err := gitClient.FetchRemoteTrackingBranch(remoteName, branchName); err != nil {
		return "", fmt.Errorf("failed to fetch default branch %q from remote %q: %w", branchName, remoteName, err)
	}

	return remoteName + "/" + branchName, nil
}
