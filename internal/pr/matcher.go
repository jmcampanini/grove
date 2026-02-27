package pr

import (
	"fmt"
	"os"

	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/naming"
)

// Match represents a PR with its worktree matching status.
type Match struct {
	PR           github.PullRequest
	WorktreePath string
}

func (m Match) HasWorktree() bool {
	return m.WorktreePath != ""
}

// Matcher matches pull requests to existing worktrees.
type Matcher struct {
	namer *naming.PullRequestNamer
}

// NewMatcher creates a new Matcher with the given PullRequestNamer.
func NewMatcher(namer *naming.PullRequestNamer) *Matcher {
	return &Matcher{
		namer: namer,
	}
}

// MatchAll returns a Match for each PR, indicating whether a worktree exists.
func (m *Matcher) MatchAll(prs []github.PullRequest, worktrees []git.Worktree) []Match {
	result := make([]Match, len(prs))
	for i, p := range prs {
		result[i] = Match{PR: p}
		if wt := m.FindWorktreeForPR(p, worktrees); wt != nil {
			result[i].WorktreePath = wt.AbsolutePath
		}
	}
	return result
}

// FindWorktreeForPR searches worktrees for one that matches the given PR.
// It checks three match strategies in order:
// 1. Template-generated branch name (for worktrees created via grove pr checkout)
// 2. PR's remote branch name directly (for manually created worktrees)
// 3. Recreated branch name (for branches reconstructed from merge commits)
// Returns nil if no match is found.
func (m *Matcher) FindWorktreeForPR(pr github.PullRequest, worktrees []git.Worktree) *git.Worktree {
	// Apply template to get expected local branch name
	prData := naming.PullRequestTemplateData{
		BranchName: pr.BranchName,
		Number:     pr.Number,
	}
	expectedBranch, err := m.namer.GenerateBranchName(prData)
	if err != nil {
		// TODO: replace with structured debug logging when a logging framework is added
		fmt.Fprintf(os.Stderr, "warning: branch name template failed, using direct match only: %v\n", err)
		expectedBranch = ""
	}

	recreatedBranch, err := m.namer.GenerateRecreatedBranchName(prData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: recreated branch name template failed, skipping recreated match: %v\n", err)
		recreatedBranch = ""
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
			// Match 3: Recreated branch name (reconstructed from merge commit)
			if recreatedBranch != "" && branch.Name == recreatedBranch {
				return &worktrees[i]
			}
		}
	}
	return nil
}
