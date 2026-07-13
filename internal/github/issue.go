package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type IssueState string

const (
	IssueStateOpen   IssueState = "OPEN"
	IssueStateClosed IssueState = "CLOSED"
)

func (s IssueState) String() string {
	return string(s)
}

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

const (
	IssueStateReasonCompleted  IssueStateReason = "COMPLETED"
	IssueStateReasonNotPlanned IssueStateReason = "NOT_PLANNED"
	IssueStateReasonReopened   IssueStateReason = "REOPENED"
)

// IssueQuery specifies filters for listing issues.
type IssueQuery struct {
	State             IssueState // Defaults to IssueStateOpen if empty
	UpdatedWithinDays int        // 0 = no filter, uses updated:>= in search
}

// ToSearchQuery converts the query to a GitHub search string for use with `gh issue list --search`.
func (q IssueQuery) ToSearchQuery() string {
	state := q.State
	if state == "" {
		state = IssueStateOpen
	}

	parts := []string{"is:issue"}
	switch state {
	case IssueStateOpen:
		parts = append(parts, "is:open")
	case IssueStateClosed:
		parts = append(parts, "is:closed")
	}

	if q.UpdatedWithinDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -q.UpdatedWithinDays)
		parts = append(parts, fmt.Sprintf("updated:>=%s", cutoff.Format("2006-01-02")))
	}

	return strings.Join(parts, " ")
}

// IssueComment represents a single comment on an issue.
type IssueComment struct {
	AuthorLogin string // May be empty if author's account was deleted
	Body        string
	CreatedAt   time.Time
}

type Issue struct {
	Assignees   []string // Assignee logins
	AuthorLogin string   // May be empty if author's account was deleted
	Body        string
	Comments    []IssueComment // Only populated by GetIssue, not ListIssues
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

// issueListJsonFields are the fields fetched for issue lists; issueViewJsonFields
// adds the detail-only fields fetched for a single issue.
const (
	issueListJsonFields = "author,createdAt,labels,number,state,title,updatedAt,url"
	issueViewJsonFields = issueListJsonFields + ",assignees,body,comments,milestone,stateReason"
)

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
