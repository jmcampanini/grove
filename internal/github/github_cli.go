package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	clog "charm.land/log/v2"
	"github.com/jmcampanini/grove-cli/internal/cache"
	"github.com/jmcampanini/grove-cli/internal/process"
)

// DefaultPRLimit is the maximum number of pull requests returned by ListPullRequests.
const DefaultPRLimit = 20

// GitHubCli provides GitHub operations by executing the gh CLI.
type GitHubCli struct {
	cache      *cache.Cache
	ctx        context.Context
	log        *clog.Logger
	timeout    time.Duration
	workingDir string
}

var _ GitHub = &GitHubCli{}

func New(ctx context.Context, workingDir string, timeout time.Duration, c *cache.Cache) GitHub {
	if ctx == nil {
		ctx = context.Background()
	}
	return &GitHubCli{
		cache:      c,
		ctx:        ctx,
		log:        clog.Default().WithPrefix("github"),
		timeout:    timeout,
		workingDir: workingDir,
	}
}

// executeGhCommand runs a read-only gh command, returning cached results when available.
func (g *GitHubCli) executeGhCommand(args ...string) (string, error) {
	if g.cache == nil {
		return g.runGhProcess(args...)
	}

	cacheKey := cache.BuildKey(g.workingDir, args)
	if payload, ok := g.cache.Get(cacheKey); ok {
		g.log.Debug("gh command cache hit", "args", args)
		return payload, nil
	}

	output, err := g.runGhProcess(args...)
	if err != nil {
		return "", err
	}

	g.cache.Set(cacheKey, output)
	return output, nil
}

func (g *GitHubCli) runGhProcess(args ...string) (string, error) {
	g.log.Debug("Executing gh command", "cmd", "gh", "args", args, "workingDir", g.workingDir)

	result, err := process.Run(g.ctx, process.Spec{
		Args: args,
		Dir:  g.workingDir,
		Env:  append(os.Environ(), "GH_PROMPT_DISABLED=1"),
		Name: "gh",
	}, g.timeout)
	if err != nil {
		command := strings.Join(args, " ")
		switch {
		case errors.Is(err, process.ErrTimedOut):
			g.log.Warn("gh command timed out", "args", args, "timeout", g.timeout, "error", err)
			return "", fmt.Errorf("gh %s timed out after %s: %w", command, g.timeout, err)
		case errors.Is(err, process.ErrCanceled):
			g.log.Warn("gh command canceled", "args", args, "error", err)
			return "", fmt.Errorf("gh %s canceled: %w", command, err)
		case errors.Is(err, process.ErrOutputLimitExceeded):
			var limitErr *process.OutputLimitError
			if errors.As(err, &limitErr) {
				g.log.Warn("gh command output limit exceeded", "args", args, "stream", limitErr.Stream, "limit", limitErr.Limit)
				return "", fmt.Errorf("gh %s %s exceeded %d MiB output limit: %w", command, limitErr.Stream, limitErr.Limit/(1024*1024), err)
			}
		}
		g.log.Warn("gh command failed", "args", args, "error", err)
		if stderr := strings.TrimSpace(string(result.Stderr)); stderr != "" {
			return "", fmt.Errorf("gh %s failed: %w: %s", command, err, stderr)
		}
		return "", fmt.Errorf("gh %s failed: %w", command, err)
	}

	output := strings.TrimSpace(string(result.Stdout))
	g.log.Debug("gh command succeeded", "args", args, "outputLen", len(output))
	return output, nil
}

func (g *GitHubCli) executeGraphQL(query string) (string, error) {
	return g.executeGhCommand("api", "graphql", "-f", "query="+query)
}

func (g *GitHubCli) GetPullRequest(prNum int) (PullRequest, error) {
	args := []string{
		"pr", "view", fmt.Sprintf("%d", prNum),
		"--json", prJsonFields + ",files",
	}

	output, err := g.executeGhCommand(args...)
	if err != nil {
		return PullRequest{}, fmt.Errorf("failed to get pull request #%d: %w", prNum, err)
	}

	var pr PullRequest
	if err := json.Unmarshal([]byte(output), &pr); err != nil {
		return PullRequest{}, fmt.Errorf("failed to parse pull request #%d: %w", prNum, err)
	}

	return pr, nil
}

func (g *GitHubCli) GetPullRequestActivity(owner, repo string, prNum int) ([]ReviewThread, []TimelineEvent, error) {
	query := fmt.Sprintf(`{
  repository(owner: %q, name: %q) {
    pullRequest(number: %d) {
      reviewThreads(first: 100) {
        nodes {
          isResolved
          path
          comments {
            totalCount
          }
        }
      }
      timelineItems(first: 100, itemTypes: [
        PULL_REQUEST_REVIEW,
        ISSUE_COMMENT,
        HEAD_REF_FORCE_PUSHED_EVENT,
        PULL_REQUEST_COMMIT,
        LABELED_EVENT,
        MERGED_EVENT,
        CLOSED_EVENT,
        REOPENED_EVENT,
        READY_FOR_REVIEW_EVENT,
        CONVERT_TO_DRAFT_EVENT,
        REVIEW_REQUESTED_EVENT
      ]) {
        nodes {
          __typename
          ... on PullRequestReview {
            author { login }
            createdAt
            state
          }
          ... on IssueComment {
            author { login }
            createdAt
          }
          ... on HeadRefForcePushedEvent {
            actor { login }
            createdAt
          }
          ... on PullRequestCommit {
            commit {
              author { user { login } }
              committedDate
              messageHeadline
            }
          }
          ... on LabeledEvent {
            actor { login }
            createdAt
            label { name }
          }
          ... on MergedEvent {
            actor { login }
            createdAt
          }
          ... on ClosedEvent {
            actor { login }
            createdAt
          }
          ... on ReopenedEvent {
            actor { login }
            createdAt
          }
          ... on ReadyForReviewEvent {
            actor { login }
            createdAt
          }
          ... on ConvertToDraftEvent {
            actor { login }
            createdAt
          }
          ... on ReviewRequestedEvent {
            actor { login }
            createdAt
            requestedReviewer {
              ... on User { login }
            }
          }
        }
      }
    }
  }
}`, owner, repo, prNum)

	output, err := g.executeGraphQL(query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get activity for PR #%d: %w", prNum, err)
	}

	return parseActivityResponse(g.log, output)
}

func (g *GitHubCli) GetPullRequestByBranch(branchName string) (*PullRequest, error) {
	args := []string{
		"pr", "list",
		"--head", branchName,
		"--state", "all",
		"--json", prJsonFields,
		"--limit", "1",
	}

	output, err := g.executeGhCommand(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request for branch %s: %w", branchName, err)
	}

	var prs []PullRequest
	if err := json.Unmarshal([]byte(output), &prs); err != nil {
		return nil, fmt.Errorf("failed to parse pull requests for branch %s: %w", branchName, err)
	}

	if len(prs) == 0 {
		return nil, nil
	}

	return &prs[0], nil
}

func (g *GitHubCli) ListPullRequests(query PRQuery, limit int) ([]PullRequest, error) {
	searchQuery := query.ToSearchQuery()

	args := []string{
		"pr", "list",
		"--search", searchQuery,
		"--json", prJsonFields,
		"--limit", fmt.Sprintf("%d", limit),
	}

	output, err := g.executeGhCommand(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests: %w", err)
	}

	var prs []PullRequest
	if err := json.Unmarshal([]byte(output), &prs); err != nil {
		return nil, fmt.Errorf("failed to parse pull requests: %w", err)
	}

	return prs, nil
}

type graphQLThreadNode struct {
	Comments struct {
		TotalCount int `json:"totalCount"`
	} `json:"comments"`
	IsResolved bool   `json:"isResolved"`
	Path       string `json:"path"`
}

func parseActivityResponse(log *clog.Logger, data string) ([]ReviewThread, []TimelineEvent, error) {
	var result struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						Nodes []graphQLThreadNode `json:"nodes"`
					} `json:"reviewThreads"`
					TimelineItems struct {
						Nodes []json.RawMessage `json:"nodes"`
					} `json:"timelineItems"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse activity response: %w", err)
	}

	threads := aggregateThreadsByPath(result.Data.Repository.PullRequest.ReviewThreads.Nodes)

	var events []TimelineEvent
	for _, raw := range result.Data.Repository.PullRequest.TimelineItems.Nodes {
		event, err := parseTimelineNode(raw)
		if err != nil {
			log.Debug("skipping timeline event", "error", err)
			continue
		}
		if event != nil {
			events = append(events, *event)
		}
	}

	return threads, events, nil
}

func aggregateThreadsByPath(nodes []graphQLThreadNode) []ReviewThread {
	byPath := make(map[string]*ReviewThread, len(nodes))
	for _, node := range nodes {
		if rt, ok := byPath[node.Path]; ok {
			rt.CommentCount += node.Comments.TotalCount
			rt.IsResolved = rt.IsResolved && node.IsResolved
		} else {
			byPath[node.Path] = &ReviewThread{
				CommentCount: node.Comments.TotalCount,
				IsResolved:   node.IsResolved,
				Path:         node.Path,
			}
		}
	}
	threads := make([]ReviewThread, 0, len(byPath))
	for _, rt := range byPath {
		threads = append(threads, *rt)
	}
	slices.SortFunc(threads, func(a, b ReviewThread) int {
		return strings.Compare(a.Path, b.Path)
	})
	return threads
}

func parseTimelineNode(raw json.RawMessage) (*TimelineEvent, error) {
	var base struct {
		TypeName string `json:"__typename"`
	}
	if err := json.Unmarshal(raw, &base); err != nil {
		return nil, err
	}

	switch base.TypeName {
	case "PullRequestReview":
		return parseReviewEvent(raw)
	case "IssueComment":
		return parseCommentEvent(raw)
	case "HeadRefForcePushedEvent":
		return parseActorEvent(raw, TimelineEventForcePushed)
	case "PullRequestCommit":
		return parseCommitEvent(raw)
	case "LabeledEvent":
		return parseLabeledEvent(raw)
	case "MergedEvent":
		return parseActorEvent(raw, TimelineEventMerged)
	case "ClosedEvent":
		return parseActorEvent(raw, TimelineEventClosed)
	case "ReopenedEvent":
		return parseActorEvent(raw, TimelineEventReopened)
	case "ReadyForReviewEvent":
		return parseActorEvent(raw, TimelineEventReadyForReview)
	case "ConvertToDraftEvent":
		return parseActorEvent(raw, TimelineEventConvertToDraft)
	case "ReviewRequestedEvent":
		return parseReviewRequestedEvent(raw)
	default:
		return nil, nil
	}
}

func parseReviewEvent(raw json.RawMessage) (*TimelineEvent, error) {
	var v struct {
		Author    struct{ Login string } `json:"author"`
		CreatedAt time.Time              `json:"createdAt"`
		State     string                 `json:"state"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &TimelineEvent{
		Actor:     v.Author.Login,
		CreatedAt: v.CreatedAt,
		Details:   strings.ToLower(strings.ReplaceAll(v.State, "_", " ")),
		Type:      TimelineEventReviewed,
	}, nil
}

func parseCommentEvent(raw json.RawMessage) (*TimelineEvent, error) {
	var v struct {
		Author    struct{ Login string } `json:"author"`
		CreatedAt time.Time              `json:"createdAt"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &TimelineEvent{
		Actor:     v.Author.Login,
		CreatedAt: v.CreatedAt,
		Type:      TimelineEventCommented,
	}, nil
}

func parseActorEvent(raw json.RawMessage, eventType TimelineEventType) (*TimelineEvent, error) {
	var v struct {
		Actor     struct{ Login string } `json:"actor"`
		CreatedAt time.Time              `json:"createdAt"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &TimelineEvent{
		Actor:     v.Actor.Login,
		CreatedAt: v.CreatedAt,
		Type:      eventType,
	}, nil
}

func parseCommitEvent(raw json.RawMessage) (*TimelineEvent, error) {
	var v struct {
		Commit struct {
			Author struct {
				User struct{ Login string } `json:"user"`
			} `json:"author"`
			CommittedDate   time.Time `json:"committedDate"`
			MessageHeadline string    `json:"messageHeadline"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &TimelineEvent{
		Actor:     v.Commit.Author.User.Login,
		CreatedAt: v.Commit.CommittedDate,
		Details:   v.Commit.MessageHeadline,
		Type:      TimelineEventCommitted,
	}, nil
}

func parseLabeledEvent(raw json.RawMessage) (*TimelineEvent, error) {
	var v struct {
		Actor     struct{ Login string } `json:"actor"`
		CreatedAt time.Time              `json:"createdAt"`
		Label     struct{ Name string }  `json:"label"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &TimelineEvent{
		Actor:     v.Actor.Login,
		CreatedAt: v.CreatedAt,
		Details:   v.Label.Name,
		Type:      TimelineEventLabeled,
	}, nil
}

func parseReviewRequestedEvent(raw json.RawMessage) (*TimelineEvent, error) {
	var v struct {
		Actor             struct{ Login string } `json:"actor"`
		CreatedAt         time.Time              `json:"createdAt"`
		RequestedReviewer struct{ Login string } `json:"requestedReviewer"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return &TimelineEvent{
		Actor:     v.Actor.Login,
		CreatedAt: v.CreatedAt,
		Details:   v.RequestedReviewer.Login,
		Type:      TimelineEventReviewRequested,
	}, nil
}
