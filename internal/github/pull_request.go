package github

import (
	"encoding/json"
	"fmt"
)

// PRState is the state of a pull request. PRStateDraft is grove's own:
// GitHub reports a draft as OPEN with isDraft set.
type PRState string

// Pull request states.
const (
	PRStateOpen   PRState = "OPEN"
	PRStateClosed PRState = "CLOSED"
	PRStateMerged PRState = "MERGED"
	PRStateDraft  PRState = "DRAFT" // Virtual state: GitHub returns OPEN + isDraft=true
)

func (s PRState) String() string {
	return string(s)
}

// IsValid reports whether s is one of the known states, PRStateDraft included.
func (s PRState) IsValid() bool {
	switch s {
	case PRStateOpen, PRStateClosed, PRStateMerged, PRStateDraft:
		return true
	}
	return false
}

// PullRequest is a GitHub pull request as gh returns it.
type PullRequest struct {
	BranchName string
	Number     int
	State      PRState
	Title      string
}

const prJSONFields = "headRefName,isDraft,number,state,title"

// UnmarshalJSON decodes the gh pr JSON shape, reporting PRStateDraft for an
// open draft.
func (pr *PullRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		HeadRefName string `json:"headRefName"`
		IsDraft     bool   `json:"isDraft"`
		Number      int    `json:"number"`
		State       string `json:"state"`
		Title       string `json:"title"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	pr.BranchName = raw.HeadRefName
	pr.Number = raw.Number
	pr.Title = raw.Title

	state, err := parsePRState(raw.State, raw.IsDraft)
	if err != nil {
		return err
	}
	pr.State = state

	return nil
}

// parsePRState maps the state GitHub reports to a PRState, reporting
// PRStateDraft for an open draft. An unknown state is an error.
func parsePRState(state string, isDraft bool) (PRState, error) {
	switch state {
	case "OPEN":
		if isDraft {
			return PRStateDraft, nil
		}
		return PRStateOpen, nil
	case "CLOSED":
		return PRStateClosed, nil
	case "MERGED":
		return PRStateMerged, nil
	default:
		return "", fmt.Errorf("unknown PR state: %s", state)
	}
}
