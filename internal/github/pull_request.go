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

type Label struct {
	Color string `json:"color"`
	Name  string `json:"name"`
}

type Review struct {
	AuthorLogin string
	State       string // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED, PENDING
	SubmittedAt time.Time
}

type StatusCheck struct {
	Conclusion string // success, failure, neutral, cancelled, timed_out, action_required, skipped
	Name       string
	Status     string // COMPLETED, IN_PROGRESS, QUEUED, REQUESTED, WAITING, PENDING
}

type PullRequest struct {
	AuthorLogin       string // May be empty if author's account was deleted
	AuthorName        string // May be empty if author's account was deleted
	BaseRefName       string
	Body              string
	BranchName        string
	Comments          int
	CreatedAt         time.Time
	FilesChanged      int
	IsCrossRepository bool // True if PR is from a fork
	Labels            []Label
	LinesAdded        int
	LinesDeleted      int
	Number            int
	Reviews           []Review
	State             PRState
	StatusChecks      []StatusCheck
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

// ReviewThread represents a review thread on a specific file in a pull request.
type ReviewThread struct {
	CommentCount int
	IsResolved   bool
	Path         string
}

// TimelineEvent represents a normalized event from a pull request's timeline.
type TimelineEvent struct {
	Actor     string
	CreatedAt time.Time
	Details   string // commit headline, label name, reviewer login, etc.
	Type      string // commented, force_pushed, committed, reviewed, labeled, merged, closed, reopened, ready_for_review, convert_to_draft, review_requested
}

const prJsonFields = "additions,author,baseRefName,body,changedFiles,comments,createdAt,deletions,headRefName,isCrossRepository,isDraft,labels,number,reviews,state,statusCheckRollup,title,updatedAt,url"

func (pr *PullRequest) UnmarshalJSON(data []byte) error {
	type rawReview struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		State       string    `json:"state"`
		SubmittedAt time.Time `json:"submittedAt"`
	}
	// gh CLI returns two shapes in statusCheckRollup:
	//   CheckRun:      {"__typename":"CheckRun", "name":"ci/test", "status":"COMPLETED", "conclusion":"success"}
	//   StatusContext:  {"__typename":"StatusContext", "context":"ci/deploy", "state":"SUCCESS"}
	type rawStatusCheckRollupEntry struct {
		TypeName   string `json:"__typename"`
		Conclusion string `json:"conclusion"` // CheckRun
		Context    string `json:"context"`    // StatusContext
		Name       string `json:"name"`       // CheckRun
		State      string `json:"state"`      // StatusContext
		Status     string `json:"status"`     // CheckRun
	}
	type rawPR struct {
		Additions         int                         `json:"additions"`
		BaseRefName       string                      `json:"baseRefName"`
		Body              string                      `json:"body"`
		ChangedFiles      int                         `json:"changedFiles"`
		Comments          []json.RawMessage           `json:"comments"`
		CreatedAt         time.Time                   `json:"createdAt"`
		Deletions         int                         `json:"deletions"`
		HeadRefName       string                      `json:"headRefName"`
		IsCrossRepository bool                        `json:"isCrossRepository"`
		IsDraft           bool                        `json:"isDraft"`
		Labels            []Label                     `json:"labels"`
		Number            int                         `json:"number"`
		Reviews           []rawReview                 `json:"reviews"`
		State             string                      `json:"state"`
		StatusCheckRollup []rawStatusCheckRollupEntry `json:"statusCheckRollup"`
		Title             string                      `json:"title"`
		UpdatedAt         time.Time                   `json:"updatedAt"`
		URL               string                      `json:"url"`
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
	pr.BaseRefName = raw.BaseRefName
	pr.Body = raw.Body
	pr.BranchName = raw.HeadRefName
	pr.Comments = len(raw.Comments)
	pr.CreatedAt = raw.CreatedAt
	pr.FilesChanged = raw.ChangedFiles
	pr.IsCrossRepository = raw.IsCrossRepository
	pr.Labels = raw.Labels
	pr.LinesAdded = raw.Additions
	pr.LinesDeleted = raw.Deletions
	pr.Number = raw.Number
	pr.Title = raw.Title
	pr.UpdatedAt = raw.UpdatedAt
	pr.URL = raw.URL

	for _, r := range raw.Reviews {
		pr.Reviews = append(pr.Reviews, Review{
			AuthorLogin: r.Author.Login,
			State:       r.State,
			SubmittedAt: r.SubmittedAt,
		})
	}

	for _, sc := range raw.StatusCheckRollup {
		if sc.TypeName == "StatusContext" {
			status := "COMPLETED"
			conclusion := strings.ToLower(sc.State)
			if sc.State == "PENDING" || sc.State == "EXPECTED" {
				status = "PENDING"
				conclusion = ""
			}
			pr.StatusChecks = append(pr.StatusChecks, StatusCheck{
				Conclusion: conclusion,
				Name:       sc.Context,
				Status:     status,
			})
		} else {
			pr.StatusChecks = append(pr.StatusChecks, StatusCheck{
				Conclusion: sc.Conclusion,
				Name:       sc.Name,
				Status:     sc.Status,
			})
		}
	}

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
