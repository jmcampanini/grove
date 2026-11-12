// Package issue matches GitHub issues to the worktrees that work on them.
package issue

import (
	clog "charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/naming"
)

// Matcher matches issues to existing worktrees.
type Matcher struct {
	log   *clog.Logger
	namer *naming.IssueNamer
}

// NewMatcher creates a new Matcher with the given IssueNamer.
// A nil logger falls back to the default logger.
func NewMatcher(namer *naming.IssueNamer, logger *clog.Logger) *Matcher {
	if logger == nil {
		logger = clog.Default()
	}
	return &Matcher{
		log:   logger.WithPrefix("issue"),
		namer: namer,
	}
}

// FindWorktreeForIssue searches worktrees for one whose branch belongs to the
// given issue. Matching is anchored on the issue number so it survives issue
// title edits (see naming.IssueNamer.MatchesIssueNumber).
// Returns nil if no match is found.
func (m *Matcher) FindWorktreeForIssue(iss github.Issue, worktrees []git.Worktree) *git.Worktree {
	for i := range worktrees {
		if worktrees[i].Ref == nil {
			continue
		}
		if branch, ok := worktrees[i].Ref.FullBranch(); ok {
			if m.namer.MatchesIssueNumber(branch.Name, iss.Number, iss.Title) {
				return &worktrees[i]
			}
		}
	}
	return nil
}
