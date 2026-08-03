package issue

import (
	clog "charm.land/log/v2"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/naming"
)

// Match represents an issue with its worktree matching status.
type Match struct {
	Issue        github.Issue
	WorktreePath string
}

func (m Match) HasWorktree() bool {
	return m.WorktreePath != ""
}

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

// MatchAll returns a Match for each issue, indicating whether a worktree exists.
func (m *Matcher) MatchAll(issues []github.Issue, worktrees []git.Worktree) []Match {
	result := make([]Match, len(issues))
	for i, iss := range issues {
		result[i] = Match{Issue: iss}
		if wt := m.FindWorktreeForIssue(iss, worktrees); wt != nil {
			result[i].WorktreePath = wt.AbsolutePath
		}
	}
	return result
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
