package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jmcampanini/grove/internal/git"
)

// removeWorktreeAndBranch force-removes a worktree and deletes its branch if present.
func removeWorktreeAndBranch(gitClient git.Git, absPath, branchName string) error {
	if err := gitClient.RemoveWorktree(absPath, true); err != nil {
		return fmt.Errorf("failed to remove worktree %q: %w", filepath.Base(absPath), err)
	}
	return deleteBranchIfExists(gitClient, branchName)
}

// deleteBranchIfExists force-deletes a named branch and ignores a missing branch.
func deleteBranchIfExists(gitClient git.Git, branchName string) error {
	if branchName == "" {
		return nil
	}
	if err := gitClient.DeleteBranch(branchName, true); err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete branch %q: %w", branchName, err)
	}
	return nil
}
