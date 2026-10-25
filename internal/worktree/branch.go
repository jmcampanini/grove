package worktree

import (
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
