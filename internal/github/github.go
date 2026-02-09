package github

type GitHub interface {
	// GetPullRequest returns a single pull request by number.
	GetPullRequest(prNum int) (PullRequest, error)

	// GetPullRequestByBranch returns the pull request for the given branch name.
	// Returns nil if no pull request exists for the branch.
	GetPullRequestByBranch(branchName string) (*PullRequest, error)

	// GetPullRequestActivity returns review threads and timeline events for a pull request
	// in a single GraphQL call.
	GetPullRequestActivity(prNum int) ([]ReviewThread, []TimelineEvent, error)

	// ListPullRequests returns a list of pull requests matching the given query.
	// Use DefaultPRLimit for the limit parameter to get the standard number of results.
	ListPullRequests(query PRQuery, limit int) ([]PullRequest, error)

	// Validate checks if gh CLI is available and authenticated.
	Validate() error
}
