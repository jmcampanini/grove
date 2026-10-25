package worktree

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jmcampanini/grove/internal/git"
)

// ExtractBranchName returns the local branch name, or an empty string for a detached worktree.
func ExtractBranchName(wt *git.Worktree) string {
	if wt.Ref == nil {
		return ""
	}
	if branch, ok := wt.Ref.FullBranch(); ok {
		return branch.Name
	}
	return ""
}

// RemoveWorktreeAndBranch force-removes a worktree and deletes its branch if present.
func RemoveWorktreeAndBranch(gitClient git.Git, absPath, branchName string) error {
	if err := gitClient.RemoveWorktree(absPath, true); err != nil {
		return fmt.Errorf("failed to remove worktree %q: %w", filepath.Base(absPath), err)
	}
	return DeleteBranchIfExists(gitClient, branchName)
}

// DeleteBranchIfExists force-deletes a named branch and ignores a missing branch.
func DeleteBranchIfExists(gitClient git.Git, branchName string) error {
	if branchName == "" {
		return nil
	}
	if err := gitClient.DeleteBranch(branchName, true); err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete branch %q: %w", branchName, err)
	}
	return nil
}
