package worktree

import (
	"fmt"

	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
)

// StatusContext provides the clients and main worktree identity used to inspect worktrees.
type StatusContext struct {
	GitHubClient     github.GitHub
	GitClient        git.Git
	MainWorktreePath string
}

// TrackingInfo records the upstream and commit counts for a local branch.
type TrackingInfo struct {
	Ahead    int
	Behind   int
	Upstream string
}

// Status records a worktree identity and its observed branch state.
type Status struct {
	AbsPath    string
	BranchName string
	Dirty      bool
	HeadSHA    string
	IsMain     bool
	Kind       string
	Locked     bool
	PR         *github.PullRequest
	Tracking   TrackingInfo
}

// LoadWorktreesAndBranches lists worktrees and indexes local branches by name.
func LoadWorktreesAndBranches(gitClient git.Git) ([]git.Worktree, map[string]git.LocalBranch, error) {
	worktrees, err := gitClient.ListWorktrees()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	branches, err := gitClient.ListLocalBranches()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list branches: %w", err)
	}

	branchMap := make(map[string]git.LocalBranch, len(branches))
	for _, branch := range branches {
		branchMap[branch.Name] = branch
	}
	return worktrees, branchMap, nil
}

// NewWorktreeStatus records identity and tracking without querying dirty or pull request state.
func NewWorktreeStatus(ctx *StatusContext, wt git.Worktree, branchMap map[string]git.LocalBranch) Status {
	ws := Status{
		AbsPath: wt.AbsolutePath,
		IsMain:  wt.AbsolutePath == ctx.MainWorktreePath,
		Locked:  wt.Locked,
	}
	ws.BranchName = ExtractBranchName(&wt)
	ws.HeadSHA = wt.CommitSHA()
	if b, ok := branchMap[ws.BranchName]; ok {
		ws.Tracking = TrackingInfo{
			Ahead:    b.Ahead,
			Behind:   b.Behind,
			Upstream: b.UpstreamName,
		}
	}
	return ws
}
