package github

import (
	"io"
	"testing"
	"time"

	clog "github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseActivityResponse(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		wantEvents  []TimelineEvent
		wantThreads []ReviewThread
	}{
		{
			name: "combined response with threads and timeline",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[
					{"isResolved":false,"path":"main.go","comments":{"totalCount":3}}
				]},
				"timelineItems":{"nodes":[
					{"__typename":"PullRequestReview","author":{"login":"alice"},"createdAt":"2024-06-01T10:00:00Z","state":"APPROVED"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{
				{CommentCount: 3, IsResolved: false, Path: "main.go"},
			},
			wantEvents: []TimelineEvent{
				{Actor: "alice", CreatedAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), Details: "approved", Type: TimelineEventReviewed},
			},
		},
		{
			name:    "invalid JSON",
			input:   `{invalid`,
			wantErr: true,
		},
		{
			name: "changes requested review",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"PullRequestReview","author":{"login":"bob"},"createdAt":"2024-06-01T11:00:00Z","state":"CHANGES_REQUESTED"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "bob", CreatedAt: time.Date(2024, 6, 1, 11, 0, 0, 0, time.UTC), Details: "changes requested", Type: TimelineEventReviewed},
			},
		},
		{
			name: "comment event",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"IssueComment","author":{"login":"carol"},"createdAt":"2024-06-01T12:00:00Z"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "carol", CreatedAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), Type: TimelineEventCommented},
			},
		},
		{
			name: "force push event",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"HeadRefForcePushedEvent","actor":{"login":"dave"},"createdAt":"2024-06-01T13:00:00Z"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "dave", CreatedAt: time.Date(2024, 6, 1, 13, 0, 0, 0, time.UTC), Type: TimelineEventForcePushed},
			},
		},
		{
			name: "commit event",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"PullRequestCommit","commit":{"author":{"user":{"login":"eve"}},"committedDate":"2024-06-01T14:00:00Z","messageHeadline":"fix: resolve null pointer"}}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "eve", CreatedAt: time.Date(2024, 6, 1, 14, 0, 0, 0, time.UTC), Details: "fix: resolve null pointer", Type: TimelineEventCommitted},
			},
		},
		{
			name: "labeled event",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"LabeledEvent","actor":{"login":"frank"},"createdAt":"2024-06-01T15:00:00Z","label":{"name":"bug"}}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "frank", CreatedAt: time.Date(2024, 6, 1, 15, 0, 0, 0, time.UTC), Details: "bug", Type: TimelineEventLabeled},
			},
		},
		{
			name: "merged event",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"MergedEvent","actor":{"login":"grace"},"createdAt":"2024-06-01T16:00:00Z"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "grace", CreatedAt: time.Date(2024, 6, 1, 16, 0, 0, 0, time.UTC), Type: TimelineEventMerged},
			},
		},
		{
			name: "closed event",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"ClosedEvent","actor":{"login":"hank"},"createdAt":"2024-06-01T17:00:00Z"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "hank", CreatedAt: time.Date(2024, 6, 1, 17, 0, 0, 0, time.UTC), Type: TimelineEventClosed},
			},
		},
		{
			name: "reopened event",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"ReopenedEvent","actor":{"login":"iris"},"createdAt":"2024-06-01T18:00:00Z"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "iris", CreatedAt: time.Date(2024, 6, 1, 18, 0, 0, 0, time.UTC), Type: TimelineEventReopened},
			},
		},
		{
			name: "ready for review event",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"ReadyForReviewEvent","actor":{"login":"jack"},"createdAt":"2024-06-01T19:00:00Z"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "jack", CreatedAt: time.Date(2024, 6, 1, 19, 0, 0, 0, time.UTC), Type: TimelineEventReadyForReview},
			},
		},
		{
			name: "convert to draft event",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"ConvertToDraftEvent","actor":{"login":"kate"},"createdAt":"2024-06-01T20:00:00Z"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "kate", CreatedAt: time.Date(2024, 6, 1, 20, 0, 0, 0, time.UTC), Type: TimelineEventConvertToDraft},
			},
		},
		{
			name: "review requested event",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"ReviewRequestedEvent","actor":{"login":"leo"},"createdAt":"2024-06-01T21:00:00Z","requestedReviewer":{"login":"mike"}}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "leo", CreatedAt: time.Date(2024, 6, 1, 21, 0, 0, 0, time.UTC), Details: "mike", Type: TimelineEventReviewRequested},
			},
		},
		{
			name: "multiple mixed events",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"PullRequestCommit","commit":{"author":{"user":{"login":"dev"}},"committedDate":"2024-06-01T10:00:00Z","messageHeadline":"initial commit"}},
					{"__typename":"PullRequestReview","author":{"login":"reviewer"},"createdAt":"2024-06-01T11:00:00Z","state":"APPROVED"},
					{"__typename":"MergedEvent","actor":{"login":"merger"},"createdAt":"2024-06-01T12:00:00Z"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents: []TimelineEvent{
				{Actor: "dev", CreatedAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), Details: "initial commit", Type: TimelineEventCommitted},
				{Actor: "reviewer", CreatedAt: time.Date(2024, 6, 1, 11, 0, 0, 0, time.UTC), Details: "approved", Type: TimelineEventReviewed},
				{Actor: "merger", CreatedAt: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), Type: TimelineEventMerged},
			},
		},
		{
			name: "empty nodes",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents:  nil,
		},
		{
			name: "unknown typename is skipped",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[]},
				"timelineItems":{"nodes":[
					{"__typename":"UnknownEvent","actor":{"login":"x"},"createdAt":"2024-06-01T10:00:00Z"}
				]}
			}}}}`,
			wantThreads: []ReviewThread{},
			wantEvents:  nil,
		},
		{
			name: "thread aggregation: multiple threads same path",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[
					{"isResolved":true,"path":"auth.go","comments":{"totalCount":2}},
					{"isResolved":false,"path":"auth.go","comments":{"totalCount":1}}
				]},
				"timelineItems":{"nodes":[]}
			}}}}`,
			wantThreads: []ReviewThread{
				{CommentCount: 3, IsResolved: false, Path: "auth.go"},
			},
			wantEvents: nil,
		},
		{
			name: "thread aggregation: multiple paths sorted",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[
					{"isResolved":true,"path":"util.go","comments":{"totalCount":1}},
					{"isResolved":false,"path":"main.go","comments":{"totalCount":2}}
				]},
				"timelineItems":{"nodes":[]}
			}}}}`,
			wantThreads: []ReviewThread{
				{CommentCount: 2, IsResolved: false, Path: "main.go"},
				{CommentCount: 1, IsResolved: true, Path: "util.go"},
			},
			wantEvents: nil,
		},
		{
			name: "thread aggregation: all resolved stay resolved",
			input: `{"data":{"repository":{"pullRequest":{
				"reviewThreads":{"nodes":[
					{"isResolved":true,"path":"foo.go","comments":{"totalCount":1}},
					{"isResolved":true,"path":"foo.go","comments":{"totalCount":2}}
				]},
				"timelineItems":{"nodes":[]}
			}}}}`,
			wantThreads: []ReviewThread{
				{CommentCount: 3, IsResolved: true, Path: "foo.go"},
			},
			wantEvents: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			threads, events, err := parseActivityResponse(clog.New(io.Discard), tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantThreads, threads)
			assert.Equal(t, tt.wantEvents, events)
		})
	}
}

func TestParseTimelineNode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *TimelineEvent
		wantErr bool
	}{
		{
			name:  "review event",
			input: `{"__typename":"PullRequestReview","author":{"login":"alice"},"createdAt":"2024-06-01T10:00:00Z","state":"APPROVED"}`,
			want:  &TimelineEvent{Actor: "alice", CreatedAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), Details: "approved", Type: TimelineEventReviewed},
		},
		{
			name:  "unknown typename returns nil",
			input: `{"__typename":"UnknownEvent"}`,
			want:  nil,
		},
		{
			name:    "invalid JSON",
			input:   `{invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimelineNode([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
