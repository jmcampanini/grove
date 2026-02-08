package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	clog "github.com/charmbracelet/log"
)

// DefaultPRLimit is the maximum number of pull requests returned by ListPullRequests.
const DefaultPRLimit = 20

// GitHubCli provides GitHub operations by executing the gh CLI.
type GitHubCli struct {
	log        *clog.Logger
	timeout    time.Duration
	workingDir string
}

var _ GitHub = &GitHubCli{}

// New creates a new GitHubCli instance that executes gh commands
// in the specified working directory.
func New(workingDir string, timeout time.Duration) GitHub {
	return &GitHubCli{
		log:        clog.Default().WithPrefix("github"),
		timeout:    timeout,
		workingDir: workingDir,
	}
}

func (g *GitHubCli) executeGhCommand(args ...string) (string, error) {
	g.log.Debug("Executing gh command", "cmd", "gh", "args", args, "workingDir", g.workingDir)

	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = g.workingDir
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			g.log.Warn("gh command timed out", "args", args, "timeout", g.timeout, "error", err)
			return "", fmt.Errorf("gh %s timed out after %s", strings.Join(args, " "), g.timeout)
		}
		g.log.Warn("gh command failed", "args", args, "stderr", stderr.String(), "error", err)
		return "", fmt.Errorf("gh %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	g.log.Debug("gh command succeeded", "args", args, "outputLen", len(output))
	return output, nil
}

func (g *GitHubCli) repoOwnerName() (owner, name string, err error) {
	output, err := g.executeGhCommand("repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	if err != nil {
		return "", "", fmt.Errorf("failed to get repo name: %w", err)
	}
	parts := strings.SplitN(output, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected repo format: %s", output)
	}
	return parts[0], parts[1], nil
}

func (g *GitHubCli) executeGraphQL(query string) (string, error) {
	return g.executeGhCommand("api", "graphql", "-f", "query="+query)
}

func (g *GitHubCli) GetPullRequest(prNum int) (PullRequest, error) {
	args := []string{
		"pr", "view", fmt.Sprintf("%d", prNum),
		"--json", prJsonFields,
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

func (g *GitHubCli) GetPullRequestFiles(prNum int) ([]PullRequestFile, error) {
	args := []string{
		"pr", "view", fmt.Sprintf("%d", prNum),
		"--json", "files",
	}

	output, err := g.executeGhCommand(args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get files for pull request #%d: %w", prNum, err)
	}

	// gh returns: {"files": [{"path": "...", "additions": N, "deletions": N}, ...]}
	var result struct {
		Files []PullRequestFile `json:"files"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("failed to parse files for pull request #%d: %w", prNum, err)
	}

	return result.Files, nil
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

func (g *GitHubCli) GetPullRequestReviewThreads(prNum int) ([]ReviewThread, error) {
	owner, name, err := g.repoOwnerName()
	if err != nil {
		return nil, err
	}

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
    }
  }
}`, owner, name, prNum)

	output, err := g.executeGraphQL(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get review threads for PR #%d: %w", prNum, err)
	}

	return parseReviewThreadsFromJSON(output)
}

func (g *GitHubCli) GetPullRequestTimeline(prNum int) ([]TimelineEvent, error) {
	owner, name, err := g.repoOwnerName()
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`{
  repository(owner: %q, name: %q) {
    pullRequest(number: %d) {
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
}`, owner, name, prNum)

	output, err := g.executeGraphQL(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get timeline for PR #%d: %w", prNum, err)
	}

	return parseTimelineEvents(output)
}

func parseTimelineEvents(data string) ([]TimelineEvent, error) {
	var result struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					TimelineItems struct {
						Nodes []json.RawMessage `json:"nodes"`
					} `json:"timelineItems"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to parse timeline: %w", err)
	}

	var events []TimelineEvent
	for _, raw := range result.Data.Repository.PullRequest.TimelineItems.Nodes {
		event, _ := parseTimelineNode(raw)
		if event != nil {
			events = append(events, *event)
		}
	}
	return events, nil
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
		return parseActorEvent(raw, "force_pushed", "")
	case "PullRequestCommit":
		return parseCommitEvent(raw)
	case "LabeledEvent":
		return parseLabeledEvent(raw)
	case "MergedEvent":
		return parseActorEvent(raw, "merged", "")
	case "ClosedEvent":
		return parseActorEvent(raw, "closed", "")
	case "ReopenedEvent":
		return parseActorEvent(raw, "reopened", "")
	case "ReadyForReviewEvent":
		return parseActorEvent(raw, "ready_for_review", "")
	case "ConvertToDraftEvent":
		return parseActorEvent(raw, "convert_to_draft", "")
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
		Type:      "reviewed",
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
		Type:      "commented",
	}, nil
}

func parseActorEvent(raw json.RawMessage, eventType, details string) (*TimelineEvent, error) {
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
		Details:   details,
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
		Type:      "committed",
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
		Type:      "labeled",
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
		Type:      "review_requested",
	}, nil
}

func parseReviewThreadsFromJSON(data string) ([]ReviewThread, error) {
	var result struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewThreads struct {
						Nodes []struct {
							Comments struct {
								TotalCount int `json:"totalCount"`
							} `json:"comments"`
							IsResolved bool   `json:"isResolved"`
							Path       string `json:"path"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to parse review threads: %w", err)
	}

	threadsByPath := make(map[string]*ReviewThread)
	for _, node := range result.Data.Repository.PullRequest.ReviewThreads.Nodes {
		if rt, ok := threadsByPath[node.Path]; ok {
			rt.CommentCount += node.Comments.TotalCount
			rt.IsResolved = rt.IsResolved && node.IsResolved
		} else {
			threadsByPath[node.Path] = &ReviewThread{
				CommentCount: node.Comments.TotalCount,
				IsResolved:   node.IsResolved,
				Path:         node.Path,
			}
		}
	}

	threads := make([]ReviewThread, 0, len(threadsByPath))
	for _, rt := range threadsByPath {
		threads = append(threads, *rt)
	}
	sort.Slice(threads, func(i, j int) bool {
		return threads[i].Path < threads[j].Path
	})
	return threads, nil
}

func (g *GitHubCli) Validate() error {
	// Check if gh is installed
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found: install from https://cli.github.com")
	}

	// Check auth status - gh auth status exits non-zero if not authenticated
	if _, err := g.executeGhCommand("auth", "status"); err != nil {
		return fmt.Errorf("gh CLI not authenticated: run 'gh auth login'")
	}

	return nil
}
