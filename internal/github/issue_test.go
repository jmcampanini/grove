package github

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Issue
		wantErr string
	}{
		{
			name: "full view payload",
			input: `{
				"assignees": [{"login": "bob"}, {"login": "carol"}],
				"author": {"login": "alice", "name": "Alice A"},
				"body": "Steps to reproduce",
				"comments": [
					{"author": {"login": "bob"}, "body": "Reproduced.", "createdAt": "2026-07-10T10:00:00Z"}
				],
				"createdAt": "2026-07-01T09:00:00Z",
				"labels": [{"name": "bug", "color": "d73a4a"}],
				"milestone": {"title": "v1.4"},
				"number": 123,
				"state": "OPEN",
				"stateReason": "",
				"title": "Fix login crash",
				"updatedAt": "2026-07-11T12:00:00Z",
				"url": "https://github.com/owner/repo/issues/123"
			}`,
			want: Issue{
				Assignees:   []string{"bob", "carol"},
				AuthorLogin: "alice",
				Body:        "Steps to reproduce",
				Comments: []IssueComment{
					{
						AuthorLogin: "bob",
						Body:        "Reproduced.",
						CreatedAt:   time.Date(2026, 7, 10, 10, 0, 0, 0, time.UTC),
					},
				},
				CreatedAt:   time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
				Labels:      []Label{{Color: "d73a4a", Name: "bug"}},
				Milestone:   "v1.4",
				Number:      123,
				State:       IssueStateOpen,
				StateReason: "",
				Title:       "Fix login crash",
				UpdatedAt:   time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC),
				URL:         "https://github.com/owner/repo/issues/123",
			},
		},
		{
			name: "closed as not planned",
			input: `{
				"author": {"login": "alice"},
				"number": 7,
				"state": "CLOSED",
				"stateReason": "NOT_PLANNED",
				"title": "Old idea"
			}`,
			want: Issue{
				AuthorLogin: "alice",
				Number:      7,
				State:       IssueStateClosed,
				StateReason: IssueStateReasonNotPlanned,
				Title:       "Old idea",
			},
		},
		{
			name: "null milestone and missing detail fields (list payload)",
			input: `{
				"author": {"login": "alice"},
				"milestone": null,
				"number": 42,
				"state": "OPEN",
				"title": "List item"
			}`,
			want: Issue{
				AuthorLogin: "alice",
				Number:      42,
				State:       IssueStateOpen,
				Title:       "List item",
			},
		},
		{
			name:    "unknown state",
			input:   `{"number": 1, "state": "BANANAS", "title": "x"}`,
			wantErr: "unknown issue state: BANANAS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Issue
			err := json.Unmarshal([]byte(tt.input), &got)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssueQueryToSearchQuery(t *testing.T) {
	tests := []struct {
		name       string
		query      IssueQuery
		want       string
		wantPrefix string
	}{
		{
			name:  "default is open",
			query: IssueQuery{},
			want:  "is:issue is:open",
		},
		{
			name:  "explicit open",
			query: IssueQuery{State: IssueStateOpen},
			want:  "is:issue is:open",
		},
		{
			name:  "closed",
			query: IssueQuery{State: IssueStateClosed},
			want:  "is:issue is:closed",
		},
		{
			name:       "updated within days",
			query:      IssueQuery{State: IssueStateOpen, UpdatedWithinDays: 7},
			wantPrefix: "is:issue is:open updated:>=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.query.ToSearchQuery()
			if tt.wantPrefix != "" {
				assert.True(t, strings.HasPrefix(got, tt.wantPrefix), "got %q, want prefix %q", got, tt.wantPrefix)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssueStateIsValid(t *testing.T) {
	assert.True(t, IssueStateOpen.IsValid())
	assert.True(t, IssueStateClosed.IsValid())
	assert.False(t, IssueState("MERGED").IsValid())
}
