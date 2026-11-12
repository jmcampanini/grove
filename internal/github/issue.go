package github

import (
	"encoding/json"
	"fmt"
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

// IssueStateReason describes why a closed issue was closed.
// GitHub returns an empty reason for open issues.
type IssueStateReason string

// Reasons GitHub attaches to a closed issue.
const (
	IssueStateReasonCompleted  IssueStateReason = "COMPLETED"
	IssueStateReasonNotPlanned IssueStateReason = "NOT_PLANNED"
	IssueStateReasonReopened   IssueStateReason = "REOPENED"
)

// Issue is a GitHub issue as gh returns it.
type Issue struct {
	Number      int
	State       IssueState
	StateReason IssueStateReason // Empty for open issues
	Title       string
}

// issueViewJSONFields are the gh issue view fields fetched for a single issue.
const issueViewJSONFields = "number,state,stateReason,title"

// UnmarshalJSON decodes the gh issue JSON shape. An unknown state is an
// error.
func (i *Issue) UnmarshalJSON(data []byte) error {
	var raw struct {
		Number      int    `json:"number"`
		State       string `json:"state"`
		StateReason string `json:"stateReason"`
		Title       string `json:"title"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	i.Number = raw.Number
	i.StateReason = IssueStateReason(raw.StateReason)
	i.Title = raw.Title

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
