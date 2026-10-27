package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// maxPageSize is the most nodes GitHub's GraphQL API returns for one page of
// a connection.
const maxPageSize = 100

// ListRequest selects the records one list call returns: the records after
// After, or from the newest when After is empty, up to Limit of them, or
// every remaining record when All is set. Limit must be positive unless All
// is set.
type ListRequest struct {
	After string
	All   bool
	Limit int
}

// IssueList is one run of open issues. When HasMore is set, the request
// stopped at its limit while GitHub has more records, and NextCursor resumes
// after the last issue in Issues.
type IssueList struct {
	HasMore    bool
	Issues     []Issue
	NextCursor string
}

// PullRequestList is one run of open pull requests. When HasMore is set, the
// request stopped at its limit while GitHub has more records, and NextCursor
// resumes after the last pull request in PullRequests.
type PullRequestList struct {
	HasMore      bool
	NextCursor   string
	PullRequests []PullRequest
}

// pageInfo is the GraphQL pageInfo of one connection page.
type pageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

// fetchPage requests up to first nodes after the cursor, or from the start
// when after is empty.
type fetchPage[T any] func(first int, after string) ([]T, pageInfo, error)

// collectPages pages through a connection until it has req.Limit records, or
// until the connection ends when req.All is set. keep selects the nodes that
// count as records. Every page is requested with at most as many nodes as
// the limit still allows, so the limit is always reached at the end of a
// page and the page's endCursor resumes after the last record returned.
func collectPages[T any](req ListRequest, fetch fetchPage[T], keep func(T) bool) (records []T, hasMore bool, nextCursor string, err error) {
	records = []T{}
	after := req.After
	for {
		first := maxPageSize
		if !req.All {
			first = min(req.Limit-len(records), maxPageSize)
		}
		nodes, page, err := fetch(first, after)
		if err != nil {
			return nil, false, "", err
		}
		for _, node := range nodes {
			if keep(node) {
				records = append(records, node)
			}
		}
		if !page.HasNextPage {
			return records, false, "", nil
		}
		if len(nodes) == 0 || page.EndCursor == "" {
			return nil, false, "", errors.New("GitHub reported another page without returning a cursor to it")
		}
		after = page.EndCursor
		if !req.All && len(records) >= req.Limit {
			return records, true, after, nil
		}
	}
}

// openIssuesQuery reads one page of the open issues of a repository, newest
// first.
const openIssuesQuery = `query($owner: String!, $name: String!, $first: Int!, $after: String) {
  repository(owner: $owner, name: $name) {
    issues(first: $first, after: $after, states: [OPEN], orderBy: {field: CREATED_AT, direction: DESC}) {
      pageInfo { endCursor hasNextPage }
      nodes {
        author { login }
        createdAt
        labels(first: 100) { nodes { color name } }
        number
        state
        title
        updatedAt
        url
      }
    }
  }
}`

// openPullRequestsQuery reads one page of the open pull requests of a
// repository, newest first. The connection cannot exclude drafts, so the
// caller drops them.
const openPullRequestsQuery = `query($owner: String!, $name: String!, $first: Int!, $after: String) {
  repository(owner: $owner, name: $name) {
    pullRequests(first: $first, after: $after, states: [OPEN], orderBy: {field: CREATED_AT, direction: DESC}) {
      pageInfo { endCursor hasNextPage }
      nodes {
        author { login }
        baseRefName
        createdAt
        headRefName
        isCrossRepository
        isDraft
        labels(first: 100) { nodes { color name } }
        number
        state
        title
        updatedAt
        url
      }
    }
  }
}`

// ListOpenIssues pages through the open issues of the repository with gh api
// graphql. Each page is one gh command and is read from and stored in the
// cache like any other.
func (g *CLI) ListOpenIssues(req ListRequest) (IssueList, error) {
	repo, err := g.listRepository()
	if err != nil {
		return IssueList{}, fmt.Errorf("failed to list issues: %w", err)
	}
	if !repo.HasIssuesEnabled {
		return IssueList{}, fmt.Errorf("failed to list issues: the %s/%s repository has disabled issues", repo.Owner.Login, repo.Name)
	}

	fetch := func(first int, after string) ([]Issue, pageInfo, error) {
		var connection struct {
			Issues struct {
				Nodes    []graphQLIssueNode `json:"nodes"`
				PageInfo pageInfo           `json:"pageInfo"`
			} `json:"issues"`
		}
		if err := g.queryRepositoryPage(repo, openIssuesQuery, first, after, &connection); err != nil {
			return nil, pageInfo{}, err
		}
		issues := make([]Issue, 0, len(connection.Issues.Nodes))
		for _, node := range connection.Issues.Nodes {
			issue, err := node.issue()
			if err != nil {
				return nil, pageInfo{}, err
			}
			issues = append(issues, issue)
		}
		return issues, connection.Issues.PageInfo, nil
	}

	issues, hasMore, nextCursor, err := collectPages(req, fetch, func(Issue) bool { return true })
	if err != nil {
		return IssueList{}, fmt.Errorf("failed to list issues: %w", err)
	}
	return IssueList{HasMore: hasMore, Issues: issues, NextCursor: nextCursor}, nil
}

// ListOpenPullRequests pages through the open pull requests of the
// repository with gh api graphql, dropping drafts. Each page is one gh
// command and is read from and stored in the cache like any other.
func (g *CLI) ListOpenPullRequests(req ListRequest) (PullRequestList, error) {
	repo, err := g.listRepository()
	if err != nil {
		return PullRequestList{}, fmt.Errorf("failed to list pull requests: %w", err)
	}

	fetch := func(first int, after string) ([]PullRequest, pageInfo, error) {
		var connection struct {
			PullRequests struct {
				Nodes    []graphQLPullRequestNode `json:"nodes"`
				PageInfo pageInfo                 `json:"pageInfo"`
			} `json:"pullRequests"`
		}
		if err := g.queryRepositoryPage(repo, openPullRequestsQuery, first, after, &connection); err != nil {
			return nil, pageInfo{}, err
		}
		prs := make([]PullRequest, 0, len(connection.PullRequests.Nodes))
		for _, node := range connection.PullRequests.Nodes {
			pr, err := node.pullRequest()
			if err != nil {
				return nil, pageInfo{}, err
			}
			prs = append(prs, pr)
		}
		return prs, connection.PullRequests.PageInfo, nil
	}

	prs, hasMore, nextCursor, err := collectPages(req, fetch, func(pr PullRequest) bool {
		return pr.State != PRStateDraft
	})
	if err != nil {
		return PullRequestList{}, fmt.Errorf("failed to list pull requests: %w", err)
	}
	return PullRequestList{HasMore: hasMore, NextCursor: nextCursor, PullRequests: prs}, nil
}

// listRepository is the repository a list command reads: the one gh
// resolves for the working directory, the same one gh issue list and gh pr
// list use.
type listRepository struct {
	HasIssuesEnabled bool   `json:"hasIssuesEnabled"`
	Name             string `json:"name"`
	Owner            struct {
		Login string `json:"login"`
	} `json:"owner"`
	URL string `json:"url"`
}

// listRepository resolves the repository of the working directory with gh
// repo view. gh api sends a query to gh's default host rather than the
// host of the repository, so each query names the host of the repository
// URL.
func (g *CLI) listRepository() (listRepository, error) {
	output, err := g.executeGhCommand("repo", "view", "--json", "hasIssuesEnabled,name,owner,url")
	if err != nil {
		return listRepository{}, err
	}
	var repo listRepository
	if err := json.Unmarshal([]byte(output), &repo); err != nil {
		return listRepository{}, fmt.Errorf("failed to parse repository: %w", err)
	}
	if repo.Name == "" || repo.Owner.Login == "" {
		return listRepository{}, errors.New("gh repo view reported no repository name or owner")
	}
	return repo, nil
}

// host returns the host name of the repository URL.
func (r listRepository) host() (string, error) {
	u, err := url.Parse(r.URL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("repository URL %q has no host", r.URL)
	}
	return u.Host, nil
}

// queryRepositoryPage runs a query against repo for one page and decodes
// the repository object into repository.
func (g *CLI) queryRepositoryPage(repo listRepository, query string, first int, after string, repository any) error {
	host, err := repo.host()
	if err != nil {
		return err
	}
	args := []string{
		"api", "graphql",
		"--hostname", host,
		"-f", "owner=" + repo.Owner.Login,
		"-f", "name=" + repo.Name,
		"-F", "first=" + strconv.Itoa(first),
	}
	if after != "" {
		args = append(args, "-f", "after="+after)
	}
	args = append(args, "-f", "query="+query)

	output, err := g.executeGhCommand(args...)
	if err != nil {
		return err
	}

	var response struct {
		Data struct {
			Repository json.RawMessage `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return fmt.Errorf("failed to parse GraphQL response: %w", err)
	}
	if len(response.Errors) > 0 {
		messages := make([]string, 0, len(response.Errors))
		for _, e := range response.Errors {
			messages = append(messages, e.Message)
		}
		return fmt.Errorf("GraphQL query failed: %s", strings.Join(messages, "; "))
	}
	if len(response.Data.Repository) == 0 || string(response.Data.Repository) == "null" {
		return errors.New("GraphQL response has no repository")
	}
	if err := json.Unmarshal(response.Data.Repository, repository); err != nil {
		return fmt.Errorf("failed to parse GraphQL response: %w", err)
	}
	return nil
}

// graphQLLabels is a GraphQL label connection.
type graphQLLabels struct {
	Nodes []Label `json:"nodes"`
}

// labels returns the label nodes, never nil.
func (l graphQLLabels) labels() []Label {
	if l.Nodes == nil {
		return []Label{}
	}
	return l.Nodes
}

// graphQLIssueNode is one issue node of openIssuesQuery.
type graphQLIssueNode struct {
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	CreatedAt time.Time     `json:"createdAt"`
	Labels    graphQLLabels `json:"labels"`
	Number    int           `json:"number"`
	State     string        `json:"state"`
	Title     string        `json:"title"`
	UpdatedAt time.Time     `json:"updatedAt"`
	URL       string        `json:"url"`
}

func (n graphQLIssueNode) issue() (Issue, error) {
	state := IssueState(n.State)
	if !state.IsValid() {
		return Issue{}, fmt.Errorf("unknown issue state: %s", n.State)
	}
	issue := Issue{
		CreatedAt: n.CreatedAt,
		Labels:    n.Labels.labels(),
		Number:    n.Number,
		State:     state,
		Title:     n.Title,
		UpdatedAt: n.UpdatedAt,
		URL:       n.URL,
	}
	if n.Author != nil {
		issue.AuthorLogin = n.Author.Login
	}
	return issue, nil
}

// graphQLPullRequestNode is one pull request node of openPullRequestsQuery.
type graphQLPullRequestNode struct {
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
	BaseRefName       string        `json:"baseRefName"`
	CreatedAt         time.Time     `json:"createdAt"`
	HeadRefName       string        `json:"headRefName"`
	IsCrossRepository bool          `json:"isCrossRepository"`
	IsDraft           bool          `json:"isDraft"`
	Labels            graphQLLabels `json:"labels"`
	Number            int           `json:"number"`
	State             string        `json:"state"`
	Title             string        `json:"title"`
	UpdatedAt         time.Time     `json:"updatedAt"`
	URL               string        `json:"url"`
}

func (n graphQLPullRequestNode) pullRequest() (PullRequest, error) {
	state, err := parsePRState(n.State, n.IsDraft)
	if err != nil {
		return PullRequest{}, err
	}
	pr := PullRequest{
		BaseRefName:       n.BaseRefName,
		BranchName:        n.HeadRefName,
		CreatedAt:         n.CreatedAt,
		IsCrossRepository: n.IsCrossRepository,
		Labels:            n.Labels.labels(),
		Number:            n.Number,
		State:             state,
		Title:             n.Title,
		UpdatedAt:         n.UpdatedAt,
		URL:               n.URL,
	}
	if n.Author != nil {
		pr.AuthorLogin = n.Author.Login
	}
	return pr, nil
}
