package github

type GitHub interface {
	// GetPullRequest returns a single pull request by number.
	GetPullRequest(prNum int) (PullRequest, error)

	// GetPullRequestByBranch returns the pull request for the given branch name.
	// Returns nil if no pull request exists for the branch.
	GetPullRequestByBranch(branchName string) (*PullRequest, error)

	// GetPullRequestFiles returns the list of files changed in a pull request.
	GetPullRequestFiles(prNum int) ([]PullRequestFile, error)

	// GetPullRequestReviewThreads returns review threads aggregated per file path.
	GetPullRequestReviewThreads(prNum int) ([]ReviewThread, error)

	// GetPullRequestTimeline returns normalized timeline events for a pull request.
	GetPullRequestTimeline(prNum int) ([]TimelineEvent, error)

	// ListPullRequests returns a list of pull requests matching the given query.
	// Use DefaultPRLimit for the limit parameter to get the standard number of results.
	ListPullRequests(query PRQuery, limit int) ([]PullRequest, error)

	// Validate checks if gh CLI is available and authenticated.
	Validate() error
}
