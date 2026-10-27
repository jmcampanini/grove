package github

import (
	"encoding/json"
	"fmt"
	"time"
)

// IssueState is the open or closed state GitHub reports for an issue.
type IssueState string

// Issue states as GitHub reports them.
const (
	IssueStateOpen   IssueState = "OPEN"
	IssueStateClosed IssueState = "CLOSED"
)

func (s IssueState) String() string {
	return string(s)
}

// IsValid reports whether s is a state GitHub can report.
func (s IssueState) IsValid() bool {
	switch s {
	case IssueStateOpen, IssueStateClosed:
		return true
	}
	return false
}

// IssueStateReason describes why a closed issue was closed.
// GitHub returns an empty reason for open issues.
type IssueStateReason string

// Reasons GitHub attaches to a closed issue.
const (
	IssueStateReasonCompleted  IssueStateReason = "COMPLETED"
	IssueStateReasonNotPlanned IssueStateReason = "NOT_PLANNED"
	IssueStateReasonReopened   IssueStateReason = "REOPENED"
)

// IssueComment represents a single comment on an issue.
type IssueComment struct {
	AuthorLogin string // May be empty if author's account was deleted
	Body        string
	CreatedAt   time.Time
}

// Issue is a GitHub issue as gh returns it. Assignees, Body, Comments,
// Milestone, and StateReason are populated only by GetIssue.
type Issue struct {
	Assignees   []string // Assignee logins
	AuthorLogin string   // May be empty if author's account was deleted
	Body        string
	Comments    []IssueComment // Only populated by GetIssue, not ListOpenIssues
	CreatedAt   time.Time
	Labels      []Label
	Milestone   string // Milestone title, empty if none
	Number      int
	State       IssueState
	StateReason IssueStateReason // Empty for open issues
	Title       string
	UpdatedAt   time.Time
	URL         string
}

// issueViewJSONFields are the gh issue view fields fetched for a single issue.
const issueViewJSONFields = "assignees,author,body,comments,createdAt,labels,milestone,number,state,stateReason,title,updatedAt,url"

// UnmarshalJSON decodes the gh issue JSON shape, flattening the author,
// assignee, and milestone objects to their names. An unknown state is an
// error.
func (i *Issue) UnmarshalJSON(data []byte) error {
	type rawComment struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"createdAt"`
	}
	type rawIssue struct {
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Body      string       `json:"body"`
		Comments  []rawComment `json:"comments"`
		CreatedAt time.Time    `json:"createdAt"`
		Labels    []Label      `json:"labels"`
		Milestone struct {
			Title string `json:"title"`
		} `json:"milestone"`
		Number      int       `json:"number"`
		State       string    `json:"state"`
		StateReason string    `json:"stateReason"`
		Title       string    `json:"title"`
		UpdatedAt   time.Time `json:"updatedAt"`
		URL         string    `json:"url"`
	}
	var raw rawIssue
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	i.AuthorLogin = raw.Author.Login
	i.Body = raw.Body
	i.CreatedAt = raw.CreatedAt
	i.Labels = raw.Labels
	i.Milestone = raw.Milestone.Title
	i.Number = raw.Number
	i.StateReason = IssueStateReason(raw.StateReason)
	i.Title = raw.Title
	i.UpdatedAt = raw.UpdatedAt
	i.URL = raw.URL

	for _, a := range raw.Assignees {
		i.Assignees = append(i.Assignees, a.Login)
	}

	for _, c := range raw.Comments {
		i.Comments = append(i.Comments, IssueComment{
			AuthorLogin: c.Author.Login,
			Body:        c.Body,
			CreatedAt:   c.CreatedAt,
		})
	}

	switch raw.State {
	case "OPEN":
		i.State = IssueStateOpen
	case "CLOSED":
		i.State = IssueStateClosed
	default:
		return fmt.Errorf("unknown issue state: %s", raw.State)
	}

	return nil
}
