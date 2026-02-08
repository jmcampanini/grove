package github

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRState_String(t *testing.T) {
	tests := []struct {
		name  string
		state PRState
		want  string
	}{
		{name: "open", state: PRStateOpen, want: "OPEN"},
		{name: "closed", state: PRStateClosed, want: "CLOSED"},
		{name: "merged", state: PRStateMerged, want: "MERGED"},
		{name: "draft", state: PRStateDraft, want: "DRAFT"},
		{name: "empty", state: PRState(""), want: ""},
		{name: "unknown", state: PRState("UNKNOWN"), want: "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.String())
		})
	}
}

func TestPRState_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		state PRState
		want  bool
	}{
		{name: "open is valid", state: PRStateOpen, want: true},
		{name: "closed is valid", state: PRStateClosed, want: true},
		{name: "merged is valid", state: PRStateMerged, want: true},
		{name: "draft is valid", state: PRStateDraft, want: true},
		{name: "empty is invalid", state: PRState(""), want: false},
		{name: "unknown is invalid", state: PRState("UNKNOWN"), want: false},
		{name: "lowercase open is invalid", state: PRState("open"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.state.IsValid())
		})
	}
}

func TestPRQuery_ToSearchQuery(t *testing.T) {
	// Helper to get date string for N days ago
	daysAgo := func(n int) string {
		return time.Now().AddDate(0, 0, -n).Format("2006-01-02")
	}

	tests := []struct {
		name           string
		query          PRQuery
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:         "empty query defaults to open non-draft PRs",
			query:        PRQuery{},
			wantContains: []string{"is:pr", "is:open", "draft:false"},
		},
		{
			name:         "explicit open state",
			query:        PRQuery{State: PRStateOpen},
			wantContains: []string{"is:pr", "is:open", "draft:false"},
		},
		{
			name:         "draft state",
			query:        PRQuery{State: PRStateDraft},
			wantContains: []string{"is:pr", "is:open", "draft:true"},
		},
		{
			name:         "closed state without date filter",
			query:        PRQuery{State: PRStateClosed},
			wantContains: []string{"is:pr", "is:closed", "is:unmerged"},
		},
		{
			name:         "closed state with date filter",
			query:        PRQuery{State: PRStateClosed, ClosedWithinDays: 7},
			wantContains: []string{"is:pr", "is:closed", "is:unmerged", "closed:>=" + daysAgo(7)},
		},
		{
			name:         "merged state without date filter",
			query:        PRQuery{State: PRStateMerged},
			wantContains: []string{"is:pr", "is:merged"},
		},
		{
			name:         "merged state with date filter",
			query:        PRQuery{State: PRStateMerged, MergedWithinDays: 30},
			wantContains: []string{"is:pr", "is:merged", "merged:>=" + daysAgo(30)},
		},
		{
			name:         "open state with updated filter",
			query:        PRQuery{State: PRStateOpen, UpdatedWithinDays: 14},
			wantContains: []string{"is:pr", "is:open", "draft:false", "updated:>=" + daysAgo(14)},
		},
		{
			name:           "closed date filter ignored for open state",
			query:          PRQuery{State: PRStateOpen, ClosedWithinDays: 7},
			wantContains:   []string{"is:pr", "is:open"},
			wantNotContain: []string{"closed:>="},
		},
		{
			name:           "merged date filter ignored for closed state",
			query:          PRQuery{State: PRStateClosed, MergedWithinDays: 7},
			wantContains:   []string{"is:pr", "is:closed"},
			wantNotContain: []string{"merged:>="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.query.ToSearchQuery()

			for _, want := range tt.wantContains {
				assert.True(t, strings.Contains(got, want),
					"expected query to contain %q, got: %q", want, got)
			}

			for _, notWant := range tt.wantNotContain {
				assert.False(t, strings.Contains(got, notWant),
					"expected query to NOT contain %q, got: %q", notWant, got)
			}
		})
	}
}

func TestPullRequest_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        PullRequest
		wantErr     bool
		errContains string
	}{
		{
			name: "open non-draft PR",
			input: `{
				"additions": 10,
				"author": {"login": "testuser", "name": "Test User"},
				"body": "This is the PR body",
				"changedFiles": 3,
				"createdAt": "2024-01-15T10:30:00Z",
				"deletions": 5,
				"headRefName": "feature-branch",
				"isDraft": false,
				"number": 123,
				"state": "OPEN",
				"title": "Add new feature",
				"updatedAt": "2024-01-16T11:00:00Z",
				"url": "https://github.com/owner/repo/pull/123"
			}`,
			want: PullRequest{
				AuthorLogin:  "testuser",
				AuthorName:   "Test User",
				Body:         "This is the PR body",
				BranchName:   "feature-branch",
				CreatedAt:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				FilesChanged: 3,
				LinesAdded:   10,
				LinesDeleted: 5,
				Number:       123,
				State:        PRStateOpen,
				Title:        "Add new feature",
				UpdatedAt:    time.Date(2024, 1, 16, 11, 0, 0, 0, time.UTC),
				URL:          "https://github.com/owner/repo/pull/123",
			},
		},
		{
			name: "draft PR becomes PRStateDraft",
			input: `{
				"additions": 0,
				"author": {"login": "dev"},
				"body": "",
				"changedFiles": 1,
				"createdAt": "2024-02-01T09:00:00Z",
				"deletions": 0,
				"headRefName": "wip-branch",
				"isDraft": true,
				"number": 456,
				"state": "OPEN",
				"title": "WIP: Draft PR",
				"updatedAt": "2024-02-01T09:00:00Z",
				"url": "https://github.com/owner/repo/pull/456"
			}`,
			want: PullRequest{
				AuthorLogin:  "dev",
				BranchName:   "wip-branch",
				CreatedAt:    time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC),
				FilesChanged: 1,
				Number:       456,
				State:        PRStateDraft,
				Title:        "WIP: Draft PR",
				UpdatedAt:    time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC),
				URL:          "https://github.com/owner/repo/pull/456",
			},
		},
		{
			name: "merged PR",
			input: `{
				"additions": 100,
				"author": {"login": "contributor", "name": "A Contributor"},
				"body": "Big feature",
				"changedFiles": 10,
				"createdAt": "2024-01-01T00:00:00Z",
				"deletions": 50,
				"headRefName": "merged-feature",
				"isDraft": false,
				"number": 789,
				"state": "MERGED",
				"title": "Merged Feature",
				"updatedAt": "2024-01-10T00:00:00Z",
				"url": "https://github.com/owner/repo/pull/789"
			}`,
			want: PullRequest{
				AuthorLogin:  "contributor",
				AuthorName:   "A Contributor",
				Body:         "Big feature",
				BranchName:   "merged-feature",
				CreatedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				FilesChanged: 10,
				LinesAdded:   100,
				LinesDeleted: 50,
				Number:       789,
				State:        PRStateMerged,
				Title:        "Merged Feature",
				UpdatedAt:    time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
				URL:          "https://github.com/owner/repo/pull/789",
			},
		},
		{
			name: "closed unmerged PR",
			input: `{
				"additions": 5,
				"author": {"login": "someone"},
				"body": "Abandoned",
				"changedFiles": 2,
				"createdAt": "2024-03-01T00:00:00Z",
				"deletions": 3,
				"headRefName": "closed-branch",
				"isDraft": false,
				"number": 101,
				"state": "CLOSED",
				"title": "Closed PR",
				"updatedAt": "2024-03-05T00:00:00Z",
				"url": "https://github.com/owner/repo/pull/101"
			}`,
			want: PullRequest{
				AuthorLogin:  "someone",
				Body:         "Abandoned",
				BranchName:   "closed-branch",
				CreatedAt:    time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
				FilesChanged: 2,
				LinesAdded:   5,
				LinesDeleted: 3,
				Number:       101,
				State:        PRStateClosed,
				Title:        "Closed PR",
				UpdatedAt:    time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC),
				URL:          "https://github.com/owner/repo/pull/101",
			},
		},
		{
			name: "deleted author account (empty author)",
			input: `{
				"additions": 1,
				"author": {},
				"body": "Orphan PR",
				"changedFiles": 1,
				"createdAt": "2024-01-01T00:00:00Z",
				"deletions": 0,
				"headRefName": "orphan-branch",
				"isDraft": false,
				"number": 999,
				"state": "OPEN",
				"title": "Orphan PR",
				"updatedAt": "2024-01-01T00:00:00Z",
				"url": "https://github.com/owner/repo/pull/999"
			}`,
			want: PullRequest{
				AuthorLogin:  "",
				AuthorName:   "",
				Body:         "Orphan PR",
				BranchName:   "orphan-branch",
				CreatedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				FilesChanged: 1,
				LinesAdded:   1,
				Number:       999,
				State:        PRStateOpen,
				Title:        "Orphan PR",
				UpdatedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				URL:          "https://github.com/owner/repo/pull/999",
			},
		},
		{
			name: "null author",
			input: `{
				"additions": 0,
				"author": null,
				"body": "",
				"changedFiles": 0,
				"createdAt": "2024-01-01T00:00:00Z",
				"deletions": 0,
				"headRefName": "null-author",
				"isDraft": false,
				"number": 888,
				"state": "OPEN",
				"title": "Null Author PR",
				"updatedAt": "2024-01-01T00:00:00Z",
				"url": "https://github.com/owner/repo/pull/888"
			}`,
			want: PullRequest{
				AuthorLogin: "",
				AuthorName:  "",
				BranchName:  "null-author",
				CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Number:      888,
				State:       PRStateOpen,
				Title:       "Null Author PR",
				UpdatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				URL:         "https://github.com/owner/repo/pull/888",
			},
		},
		{
			name:        "unknown state returns error",
			input:       `{"state": "UNKNOWN", "number": 1, "headRefName": "test"}`,
			wantErr:     true,
			errContains: "unknown PR state",
		},
		{
			name:    "invalid JSON returns error",
			input:   `{invalid json`,
			wantErr: true,
		},
		{
			name: "minimal valid PR",
			input: `{
				"headRefName": "minimal",
				"number": 1,
				"state": "OPEN",
				"isDraft": false
			}`,
			want: PullRequest{
				BranchName: "minimal",
				Number:     1,
				State:      PRStateOpen,
			},
		},
		{
			name: "PR with labels reviews and status checks",
			input: `{
				"additions": 10,
				"author": {"login": "dev"},
				"baseRefName": "main",
				"body": "Full PR",
				"changedFiles": 2,
				"comments": [{"body": "lgtm"}, {"body": "nit"}],
				"createdAt": "2024-06-01T10:00:00Z",
				"deletions": 3,
				"headRefName": "feature/full",
				"isDraft": false,
				"labels": [
					{"name": "bug", "color": "d73a4a"},
					{"name": "enhancement", "color": "a2eeef"}
				],
				"number": 500,
				"reviews": [
					{"author": {"login": "reviewer1"}, "state": "APPROVED", "submittedAt": "2024-06-02T08:00:00Z"},
					{"author": {"login": "reviewer2"}, "state": "CHANGES_REQUESTED", "submittedAt": "2024-06-02T09:00:00Z"}
				],
				"state": "OPEN",
				"statusCheckRollup": [
					{"name": "ci/test", "status": "COMPLETED", "conclusion": "success"},
					{"name": "ci/lint", "status": "IN_PROGRESS", "conclusion": ""}
				],
				"title": "Full featured PR",
				"updatedAt": "2024-06-02T12:00:00Z",
				"url": "https://github.com/owner/repo/pull/500"
			}`,
			want: PullRequest{
				AuthorLogin:  "dev",
				BaseRefName:  "main",
				Body:         "Full PR",
				BranchName:   "feature/full",
				Comments:     2,
				CreatedAt:    time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
				FilesChanged: 2,
				Labels: []Label{
					{Name: "bug", Color: "d73a4a"},
					{Name: "enhancement", Color: "a2eeef"},
				},
				LinesAdded:   10,
				LinesDeleted: 3,
				Number:       500,
				Reviews: []Review{
					{AuthorLogin: "reviewer1", State: "APPROVED", SubmittedAt: time.Date(2024, 6, 2, 8, 0, 0, 0, time.UTC)},
					{AuthorLogin: "reviewer2", State: "CHANGES_REQUESTED", SubmittedAt: time.Date(2024, 6, 2, 9, 0, 0, 0, time.UTC)},
				},
				State: PRStateOpen,
				StatusChecks: []StatusCheck{
					{Conclusion: "success", Name: "ci/test", Status: "COMPLETED"},
					{Name: "ci/lint", Status: "IN_PROGRESS"},
				},
				Title:     "Full featured PR",
				UpdatedAt: time.Date(2024, 6, 2, 12, 0, 0, 0, time.UTC),
				URL:       "https://github.com/owner/repo/pull/500",
			},
		},
		{
			name: "PR with empty labels reviews and status checks",
			input: `{
				"additions": 0,
				"author": {"login": "dev"},
				"baseRefName": "develop",
				"body": "",
				"changedFiles": 0,
				"comments": [],
				"createdAt": "2024-07-01T00:00:00Z",
				"deletions": 0,
				"headRefName": "empty-extras",
				"isDraft": false,
				"labels": [],
				"number": 600,
				"reviews": [],
				"state": "OPEN",
				"statusCheckRollup": [],
				"title": "Empty extras",
				"updatedAt": "2024-07-01T00:00:00Z",
				"url": "https://github.com/owner/repo/pull/600"
			}`,
			want: PullRequest{
				AuthorLogin: "dev",
				BaseRefName: "develop",
				BranchName:  "empty-extras",
				CreatedAt:   time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
				Labels:      []Label{},
				Number:      600,
				State:       PRStateOpen,
				Title:       "Empty extras",
				UpdatedAt:   time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
				URL:         "https://github.com/owner/repo/pull/600",
			},
		},
		{
			name: "PR with missing optional new fields",
			input: `{
				"additions": 1,
				"author": {"login": "dev"},
				"body": "",
				"changedFiles": 1,
				"createdAt": "2024-08-01T00:00:00Z",
				"deletions": 0,
				"headRefName": "no-extras",
				"isDraft": false,
				"number": 700,
				"state": "OPEN",
				"title": "No extras",
				"updatedAt": "2024-08-01T00:00:00Z",
				"url": "https://github.com/owner/repo/pull/700"
			}`,
			want: PullRequest{
				AuthorLogin:  "dev",
				BranchName:   "no-extras",
				CreatedAt:    time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC),
				FilesChanged: 1,
				LinesAdded:   1,
				Number:       700,
				State:        PRStateOpen,
				Title:        "No extras",
				UpdatedAt:    time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC),
				URL:          "https://github.com/owner/repo/pull/700",
			},
		},
		{
			name: "mixed statusCheckRollup with CheckRun and StatusContext",
			input: `{
				"additions": 0,
				"author": {"login": "dev"},
				"body": "",
				"changedFiles": 0,
				"createdAt": "2024-09-01T00:00:00Z",
				"deletions": 0,
				"headRefName": "mixed-checks",
				"isDraft": false,
				"number": 800,
				"state": "OPEN",
				"statusCheckRollup": [
					{"__typename": "CheckRun", "name": "ci/test", "status": "COMPLETED", "conclusion": "success", "detailsUrl": "https://github.com/owner/repo/actions/runs/123"},
					{"__typename": "StatusContext", "context": "deploy/staging", "state": "SUCCESS", "targetUrl": "https://deploy.example.com/staging"},
					{"__typename": "StatusContext", "context": "deploy/prod", "state": "PENDING"}
				],
				"title": "Mixed checks",
				"updatedAt": "2024-09-01T00:00:00Z",
				"url": "https://github.com/owner/repo/pull/800"
			}`,
			want: PullRequest{
				AuthorLogin: "dev",
				BranchName:  "mixed-checks",
				CreatedAt:   time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
				Number:      800,
				State:       PRStateOpen,
				StatusChecks: []StatusCheck{
					{Conclusion: "success", DetailURL: "https://github.com/owner/repo/actions/runs/123", Name: "ci/test", Status: "COMPLETED"},
					{Conclusion: "success", DetailURL: "https://deploy.example.com/staging", Name: "deploy/staging", Status: "COMPLETED"},
					{Conclusion: "", Name: "deploy/prod", Status: "PENDING"},
				},
				Title:     "Mixed checks",
				UpdatedAt: time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
				URL:       "https://github.com/owner/repo/pull/800",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got PullRequest
			err := json.Unmarshal([]byte(tt.input), &got)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
