package github

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTimelineEvents(t *testing.T) {
	tests := []struct {
		input   string
		name    string
		want    []TimelineEvent
		wantErr bool
	}{
		{
			name: "review event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"PullRequestReview","author":{"login":"alice"},"createdAt":"2024-06-01T10:00:00Z","state":"APPROVED"}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "alice", CreatedAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), Details: "approved", Type: TimelineEventReviewed},
			},
		},
		{
			name: "changes requested review",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"PullRequestReview","author":{"login":"bob"},"createdAt":"2024-06-01T11:00:00Z","state":"CHANGES_REQUESTED"}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "bob", CreatedAt: time.Date(2024, 6, 1, 11, 0, 0, 0, time.UTC), Details: "changes requested", Type: TimelineEventReviewed},
			},
		},
		{
			name: "comment event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"IssueComment","author":{"login":"carol"},"createdAt":"2024-06-01T12:00:00Z"}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "carol", CreatedAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), Type: TimelineEventCommented},
			},
		},
		{
			name: "force push event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"HeadRefForcePushedEvent","actor":{"login":"dave"},"createdAt":"2024-06-01T13:00:00Z"}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "dave", CreatedAt: time.Date(2024, 6, 1, 13, 0, 0, 0, time.UTC), Type: TimelineEventForcePushed},
			},
		},
		{
			name: "commit event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"PullRequestCommit","commit":{"author":{"user":{"login":"eve"}},"committedDate":"2024-06-01T14:00:00Z","messageHeadline":"fix: resolve null pointer"}}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "eve", CreatedAt: time.Date(2024, 6, 1, 14, 0, 0, 0, time.UTC), Details: "fix: resolve null pointer", Type: TimelineEventCommitted},
			},
		},
		{
			name: "labeled event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"LabeledEvent","actor":{"login":"frank"},"createdAt":"2024-06-01T15:00:00Z","label":{"name":"bug"}}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "frank", CreatedAt: time.Date(2024, 6, 1, 15, 0, 0, 0, time.UTC), Details: "bug", Type: TimelineEventLabeled},
			},
		},
		{
			name: "merged event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"MergedEvent","actor":{"login":"grace"},"createdAt":"2024-06-01T16:00:00Z"}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "grace", CreatedAt: time.Date(2024, 6, 1, 16, 0, 0, 0, time.UTC), Type: TimelineEventMerged},
			},
		},
		{
			name: "closed event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"ClosedEvent","actor":{"login":"hank"},"createdAt":"2024-06-01T17:00:00Z"}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "hank", CreatedAt: time.Date(2024, 6, 1, 17, 0, 0, 0, time.UTC), Type: TimelineEventClosed},
			},
		},
		{
			name: "reopened event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"ReopenedEvent","actor":{"login":"iris"},"createdAt":"2024-06-01T18:00:00Z"}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "iris", CreatedAt: time.Date(2024, 6, 1, 18, 0, 0, 0, time.UTC), Type: TimelineEventReopened},
			},
		},
		{
			name: "ready for review event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"ReadyForReviewEvent","actor":{"login":"jack"},"createdAt":"2024-06-01T19:00:00Z"}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "jack", CreatedAt: time.Date(2024, 6, 1, 19, 0, 0, 0, time.UTC), Type: TimelineEventReadyForReview},
			},
		},
		{
			name: "convert to draft event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"ConvertToDraftEvent","actor":{"login":"kate"},"createdAt":"2024-06-01T20:00:00Z"}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "kate", CreatedAt: time.Date(2024, 6, 1, 20, 0, 0, 0, time.UTC), Type: TimelineEventConvertToDraft},
			},
		},
		{
			name: "review requested event",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"ReviewRequestedEvent","actor":{"login":"leo"},"createdAt":"2024-06-01T21:00:00Z","requestedReviewer":{"login":"mike"}}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "leo", CreatedAt: time.Date(2024, 6, 1, 21, 0, 0, 0, time.UTC), Details: "mike", Type: TimelineEventReviewRequested},
			},
		},
		{
			name: "multiple mixed events",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"PullRequestCommit","commit":{"author":{"user":{"login":"dev"}},"committedDate":"2024-06-01T10:00:00Z","messageHeadline":"initial commit"}},
				{"__typename":"PullRequestReview","author":{"login":"reviewer"},"createdAt":"2024-06-01T11:00:00Z","state":"APPROVED"},
				{"__typename":"MergedEvent","actor":{"login":"merger"},"createdAt":"2024-06-01T12:00:00Z"}
			]}}}}}`,
			want: []TimelineEvent{
				{Actor: "dev", CreatedAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), Details: "initial commit", Type: TimelineEventCommitted},
				{Actor: "reviewer", CreatedAt: time.Date(2024, 6, 1, 11, 0, 0, 0, time.UTC), Details: "approved", Type: TimelineEventReviewed},
				{Actor: "merger", CreatedAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), Type: TimelineEventMerged},
			},
		},
		{
			name:  "empty nodes",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[]}}}}}`,
			want:  nil,
		},
		{
			name: "unknown typename is skipped",
			input: `{"data":{"repository":{"pullRequest":{"timelineItems":{"nodes":[
				{"__typename":"UnknownEvent","actor":{"login":"x"},"createdAt":"2024-06-01T10:00:00Z"}
			]}}}}}`,
			want: nil,
		},
		{
			name:    "invalid JSON",
			input:   `{invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimelineEvents(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseReviewThreads(t *testing.T) {
	tests := []struct {
		input string
		name  string
		want  []ReviewThread
	}{
		{
			name: "single thread",
			input: `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[
				{"isResolved":false,"path":"main.go","comments":{"totalCount":3}}
			]}}}}}`,
			want: []ReviewThread{
				{CommentCount: 3, IsResolved: false, Path: "main.go"},
			},
		},
		{
			name: "multiple threads same path aggregated",
			input: `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[
				{"isResolved":true,"path":"auth.go","comments":{"totalCount":2}},
				{"isResolved":false,"path":"auth.go","comments":{"totalCount":1}}
			]}}}}}`,
			want: []ReviewThread{
				{CommentCount: 3, IsResolved: false, Path: "auth.go"},
			},
		},
		{
			name: "multiple threads different paths sorted by path",
			input: `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[
				{"isResolved":true,"path":"util.go","comments":{"totalCount":1}},
				{"isResolved":false,"path":"main.go","comments":{"totalCount":2}}
			]}}}}}`,
			want: []ReviewThread{
				{CommentCount: 2, IsResolved: false, Path: "main.go"},
				{CommentCount: 1, IsResolved: true, Path: "util.go"},
			},
		},
		{
			name: "all resolved threads stay resolved",
			input: `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[
				{"isResolved":true,"path":"foo.go","comments":{"totalCount":1}},
				{"isResolved":true,"path":"foo.go","comments":{"totalCount":2}}
			]}}}}}`,
			want: []ReviewThread{
				{CommentCount: 3, IsResolved: true, Path: "foo.go"},
			},
		},
		{
			name:  "empty nodes",
			input: `{"data":{"repository":{"pullRequest":{"reviewThreads":{"nodes":[]}}}}}`,
			want:  []ReviewThread{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseReviewThreadsFromJSON(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
