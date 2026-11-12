package github

// GitHub is the set of GitHub operations grove needs. In production the gh
// executable serves them; tests substitute fakes.
type GitHub interface {
	// GetIssue returns a single issue by number.
	GetIssue(issueNum int) (Issue, error)

	// GetPullRequest returns a single pull request by number.
	GetPullRequest(prNum int) (PullRequest, error)

	// GetPullRequestByBranch returns the pull request for the given branch name.
	// Returns nil if no pull request exists for the branch.
	GetPullRequestByBranch(branchName string) (*PullRequest, error)
}
