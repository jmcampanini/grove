// Package pr matches GitHub pull requests to the worktrees that check them out.
package pr

import (
	clog "charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/naming"
)

// Matcher matches pull requests to existing worktrees.
type Matcher struct {
	log   *clog.Logger
	namer *naming.PullRequestNamer
}

// NewMatcher creates a new Matcher with the given PullRequestNamer.
// A nil logger falls back to the default logger.
func NewMatcher(namer *naming.PullRequestNamer, logger *clog.Logger) *Matcher {
	if logger == nil {
		logger = clog.Default()
	}
	return &Matcher{
		log:   logger.WithPrefix("pr"),
		namer: namer,
	}
}

// FindWorktreeForPR searches worktrees for one that matches the given PR.
// It uses a dual-match strategy:
// 1. Template-generated branch name (for worktrees created via grove pr checkout)
// 2. PR's remote branch name directly (for manually created worktrees)
// Returns nil if no match is found.
func (m *Matcher) FindWorktreeForPR(pr github.PullRequest, worktrees []git.Worktree) *git.Worktree {
	// Apply template to get expected local branch name
	prData := naming.PullRequestTemplateData{
		Branch: pr.BranchName,
		Number: pr.Number,
	}
	expectedBranch, err := m.namer.GenerateBranchName(prData)
	if err != nil {
		m.log.Debug("branch name template failed, using direct match only", "error", err, "number", pr.Number, "branch", pr.BranchName)
		expectedBranch = ""
	}

	// Search worktrees for matching branch
	for i := range worktrees {
		if branch, ok := worktrees[i].Ref.FullBranch(); ok {
			// Match 1: Template-generated branch name (grove pr checkout)
			if expectedBranch != "" && branch.Name == expectedBranch {
				return &worktrees[i]
			}
			// Match 2: PR's remote branch name directly (manual worktrees)
			if branch.Name == pr.BranchName {
				return &worktrees[i]
			}
		}
	}
	return nil
}
