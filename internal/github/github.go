package github

// GitHub is the set of GitHub operations grove needs. In production the gh
// executable serves them; tests substitute fakes.
type GitHub interface {
	// GetIssue returns a single issue by number.
	GetIssue(issueNum int) (Issue, error)

	// GetPullRequest returns a single pull request by number.
	GetPullRequest(prNum int) (PullRequest, error)

	// GetPullRequestActivity returns review threads and timeline events for a pull request
	// in a single GraphQL call.
	GetPullRequestActivity(owner, repo string, prNum int) ([]ReviewThread, []TimelineEvent, error)

	// GetPullRequestByBranch returns the pull request for the given branch name.
	// Returns nil if no pull request exists for the branch.
	GetPullRequestByBranch(branchName string) (*PullRequest, error)

	// ListOpenIssues returns the open issues of the repository, newest
	// first, in the range the request selects.
	ListOpenIssues(req ListRequest) (IssueList, error)

	// ListOpenPullRequests returns the open pull requests of the repository
	// that are not drafts, newest first, in the range the request selects.
	ListOpenPullRequests(req ListRequest) (PullRequestList, error)
}
