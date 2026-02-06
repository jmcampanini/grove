package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type PRState string

const (
	PRStateOpen   PRState = "OPEN"
	PRStateClosed PRState = "CLOSED"
	PRStateMerged PRState = "MERGED"
	PRStateDraft  PRState = "DRAFT" // Virtual state: GitHub returns OPEN + isDraft=true
)

func (s PRState) String() string {
	return string(s)
}

func (s PRState) IsValid() bool {
	switch s {
	case PRStateOpen, PRStateClosed, PRStateMerged, PRStateDraft:
		return true
	}
	return false
}

// PRQuery specifies filters for listing pull requests.
// TODO: add a ignore-users field, and thread it through from config
// TODO: add default updated within days from config
type PRQuery struct {
	ClosedWithinDays  int     // 0 = no filter, uses closed:>= in search
	MergedWithinDays  int     // 0 = no filter, uses merged:>= in search
	State             PRState // Defaults to PRStateOpen if empty
	UpdatedWithinDays int     // 0 = no filter, uses updated:>= in search
}

// ToSearchQuery converts the query to a GitHub search string for use with `gh pr list --search`.
// Date filters are only applied when semantically valid for the given state.
func (q PRQuery) ToSearchQuery() string {
	state := q.State
	if state == "" {
		state = PRStateOpen
	}

	var parts []string

	switch state {
	case PRStateOpen:
		parts = append(parts, "is:pr", "is:open", "draft:false")
	case PRStateDraft:
		parts = append(parts, "is:pr", "is:open", "draft:true")
	case PRStateClosed:
		parts = append(parts, "is:pr", "is:closed", "is:unmerged")
		if q.ClosedWithinDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -q.ClosedWithinDays)
			parts = append(parts, fmt.Sprintf("closed:>=%s", cutoff.Format("2006-01-02")))
		}
	case PRStateMerged:
		parts = append(parts, "is:pr", "is:merged")
		if q.MergedWithinDays > 0 {
			cutoff := time.Now().AddDate(0, 0, -q.MergedWithinDays)
			parts = append(parts, fmt.Sprintf("merged:>=%s", cutoff.Format("2006-01-02")))
		}
	}

	if q.UpdatedWithinDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -q.UpdatedWithinDays)
		parts = append(parts, fmt.Sprintf("updated:>=%s", cutoff.Format("2006-01-02")))
	}

	return strings.Join(parts, " ")
}

type PullRequest struct {
	AuthorLogin       string // May be empty if author's account was deleted
	AuthorName        string // May be empty if author's account was deleted
	Body              string
	BranchName        string
	CreatedAt         time.Time
	FilesChanged      int
	IsCrossRepository bool // True if PR is from a fork
	LinesAdded        int
	LinesDeleted      int
	Number            int
	State             PRState
	Title             string
	UpdatedAt         time.Time
	URL               string
}

// PullRequestFile represents a file changed in a pull request.
type PullRequestFile struct {
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Path      string `json:"path"`
}

const prJsonFields = "additions,author,body,changedFiles,createdAt,deletions,headRefName,isCrossRepository,isDraft,number,state,title,updatedAt,url"

func (pr *PullRequest) UnmarshalJSON(data []byte) error {
	type rawPR struct {
		Additions         int       `json:"additions"`
		Body              string    `json:"body"`
		ChangedFiles      int       `json:"changedFiles"`
		CreatedAt         time.Time `json:"createdAt"`
		Deletions         int       `json:"deletions"`
		HeadRefName       string    `json:"headRefName"`
		IsCrossRepository bool      `json:"isCrossRepository"`
		IsDraft           bool      `json:"isDraft"`
		Number            int       `json:"number"`
		State             string    `json:"state"`
		Title             string    `json:"title"`
		UpdatedAt         time.Time `json:"updatedAt"`
		URL               string    `json:"url"`
		Author            struct {
			Login string `json:"login"`
			Name  string `json:"name"`
		} `json:"author"`
	}
	var raw rawPR
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	pr.AuthorLogin = raw.Author.Login
	pr.AuthorName = raw.Author.Name
	pr.Body = raw.Body
	pr.BranchName = raw.HeadRefName
	pr.CreatedAt = raw.CreatedAt
	pr.FilesChanged = raw.ChangedFiles
	pr.IsCrossRepository = raw.IsCrossRepository
	pr.LinesAdded = raw.Additions
	pr.LinesDeleted = raw.Deletions
	pr.Number = raw.Number
	pr.Title = raw.Title
	pr.UpdatedAt = raw.UpdatedAt
	pr.URL = raw.URL

	if raw.IsDraft && raw.State == "OPEN" {
		pr.State = PRStateDraft
	} else {
		switch raw.State {
		case "OPEN":
			pr.State = PRStateOpen
		case "CLOSED":
			pr.State = PRStateClosed
		case "MERGED":
			pr.State = PRStateMerged
		default:
			return fmt.Errorf("unknown PR state: %s", raw.State)
		}
	}

	return nil
}
