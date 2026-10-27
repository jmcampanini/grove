package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
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

// Label is a GitHub label with its hex color.
type Label struct {
	Color string `json:"color"`
	Name  string `json:"name"`
}

// ReviewState is the state of one pull request review.
type ReviewState string

// Review states as GitHub reports them.
const (
	ReviewStateApproved         ReviewState = "APPROVED"
	ReviewStateChangesRequested ReviewState = "CHANGES_REQUESTED"
	ReviewStateCommented        ReviewState = "COMMENTED"
	ReviewStateDismissed        ReviewState = "DISMISSED"
	ReviewStatePending          ReviewState = "PENDING"
)

// Review is one review submitted on a pull request.
type Review struct {
	AuthorLogin string
	State       ReviewState
	SubmittedAt time.Time
}

// CheckConclusion is the outcome of a completed status check, in the
// lowercase form GitHub uses for check runs; commit statuses are mapped
// onto the same values.
type CheckConclusion string

// Check conclusions.
const (
	CheckConclusionActionRequired CheckConclusion = "action_required"
	CheckConclusionCancelled      CheckConclusion = "cancelled" //nolint:misspell // Match GitHub's check conclusion value.
	CheckConclusionFailure        CheckConclusion = "failure"
	CheckConclusionNeutral        CheckConclusion = "neutral"
	CheckConclusionSkipped        CheckConclusion = "skipped"
	CheckConclusionSuccess        CheckConclusion = "success"
	CheckConclusionTimedOut       CheckConclusion = "timed_out"
)

// CheckStatus is the progress state of a status check.
type CheckStatus string

// Check statuses as GitHub reports them.
const (
	CheckStatusCompleted  CheckStatus = "COMPLETED"
	CheckStatusInProgress CheckStatus = "IN_PROGRESS"
	CheckStatusPending    CheckStatus = "PENDING"
	CheckStatusQueued     CheckStatus = "QUEUED"
	CheckStatusRequested  CheckStatus = "REQUESTED"
	CheckStatusWaiting    CheckStatus = "WAITING"
)

// StatusCheck is one entry of a pull request's status check rollup. Check
// runs and commit statuses are both normalized to this shape.
type StatusCheck struct {
	Conclusion CheckConclusion
	DetailURL  string
	Name       string
	Status     CheckStatus
}

// TimelineEventType classifies a pull request timeline entry.
type TimelineEventType string

// Timeline event types grove renders.
const (
	TimelineEventClosed          TimelineEventType = "closed"
	TimelineEventCommented       TimelineEventType = "commented"
	TimelineEventCommitted       TimelineEventType = "committed"
	TimelineEventConvertToDraft  TimelineEventType = "convert_to_draft"
	TimelineEventForcePushed     TimelineEventType = "force_pushed"
	TimelineEventLabeled         TimelineEventType = "labeled"
	TimelineEventMerged          TimelineEventType = "merged"
	TimelineEventOpened          TimelineEventType = "opened" // synthetic: injected by renderTimeline
	TimelineEventReadyForReview  TimelineEventType = "ready_for_review"
	TimelineEventReopened        TimelineEventType = "reopened"
	TimelineEventReviewRequested TimelineEventType = "review_requested"
	TimelineEventReviewed        TimelineEventType = "reviewed"
)

// PullRequest is a GitHub pull request as gh returns it. Files is populated
// only by GetPullRequest. ListOpenPullRequests populates only AuthorLogin,
// BaseRefName, BranchName, CreatedAt, IsCrossRepository, Labels, Number,
// State, Title, UpdatedAt, and URL.
type PullRequest struct {
	AuthorLogin       string // May be empty if author's account was deleted
	AuthorName        string // May be empty if author's account was deleted
	BaseRefName       string
	Body              string
	BranchName        string
	Comments          int
	CreatedAt         time.Time
	Files             []PullRequestFile
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
	Details   string
	Type      TimelineEventType
}

// ParseRepoFromURL extracts owner and repo from a GitHub pull request URL.
// Works for both github.com and GitHub Enterprise URLs (only the path is parsed).
func ParseRepoFromURL(prURL string) (owner, repo string, err error) {
	u, err := url.Parse(prURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid PR URL %q: %w", prURL, err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "pull" {
		return "", "", fmt.Errorf("unexpected PR URL format %q: expected /owner/repo/pull/number", prURL)
	}
	return parts[0], parts[1], nil
}

const prJSONFields = "additions,author,baseRefName,body,changedFiles,comments,createdAt,deletions,headRefName,isCrossRepository,isDraft,labels,number,reviews,state,statusCheckRollup,title,updatedAt,url"

// UnmarshalJSON decodes the gh pr JSON shape, reporting PRStateDraft for an
// open draft and normalizing the status check rollup to StatusCheck values.
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
		Conclusion string `json:"conclusion"`
		Context    string `json:"context"`
		DetailsURL string `json:"detailsUrl"`
		Name       string `json:"name"`
		State      string `json:"state"`
		Status     string `json:"status"`
		TargetURL  string `json:"targetUrl"`
		TypeName   string `json:"__typename"`
	}
	type rawPR struct {
		Additions int `json:"additions"`
		Author    struct {
			Login string `json:"login"`
			Name  string `json:"name"`
		} `json:"author"`
		BaseRefName       string                      `json:"baseRefName"`
		Body              string                      `json:"body"`
		ChangedFiles      int                         `json:"changedFiles"`
		Comments          []json.RawMessage           `json:"comments"`
		CreatedAt         time.Time                   `json:"createdAt"`
		Deletions         int                         `json:"deletions"`
		Files             []PullRequestFile           `json:"files"`
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
	pr.Files = raw.Files
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
			State:       ReviewState(r.State),
			SubmittedAt: r.SubmittedAt,
		})
	}

	for _, sc := range raw.StatusCheckRollup {
		if sc.TypeName == "StatusContext" {
			status := CheckStatusCompleted
			conclusion := CheckConclusion(strings.ToLower(sc.State))
			switch sc.State {
			case "PENDING", "EXPECTED":
				status = CheckStatusPending
				conclusion = ""
			case "ERROR":
				conclusion = CheckConclusionFailure
			}
			pr.StatusChecks = append(pr.StatusChecks, StatusCheck{
				Conclusion: conclusion,
				DetailURL:  sc.TargetURL,
				Name:       sc.Context,
				Status:     status,
			})
		} else {
			pr.StatusChecks = append(pr.StatusChecks, StatusCheck{
				Conclusion: CheckConclusion(strings.ToLower(sc.Conclusion)),
				DetailURL:  sc.DetailsURL,
				Name:       sc.Name,
				Status:     CheckStatus(sc.Status),
			})
		}
	}

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
