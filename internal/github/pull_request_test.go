package github

import (
	"encoding/json"
	"testing"

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
				"headRefName": "feature-branch",
				"isDraft": false,
				"number": 123,
				"state": "OPEN",
				"title": "Add new feature"
			}`,
			want: PullRequest{
				BranchName: "feature-branch",
				Number:     123,
				State:      PRStateOpen,
				Title:      "Add new feature",
			},
		},
		{
			name: "draft PR becomes PRStateDraft",
			input: `{
				"headRefName": "draft-feature",
				"isDraft": true,
				"number": 456,
				"state": "OPEN",
				"title": "WIP: Draft Feature"
			}`,
			want: PullRequest{
				BranchName: "draft-feature",
				Number:     456,
				State:      PRStateDraft,
				Title:      "WIP: Draft Feature",
			},
		},
		{
			name: "merged PR",
			input: `{
				"headRefName": "merged-feature",
				"isDraft": false,
				"number": 789,
				"state": "MERGED",
				"title": "Merged Feature"
			}`,
			want: PullRequest{
				BranchName: "merged-feature",
				Number:     789,
				State:      PRStateMerged,
				Title:      "Merged Feature",
			},
		},
		{
			name: "closed unmerged PR",
			input: `{
				"headRefName": "closed-branch",
				"isDraft": false,
				"number": 101,
				"state": "CLOSED",
				"title": "Closed PR"
			}`,
			want: PullRequest{
				BranchName: "closed-branch",
				Number:     101,
				State:      PRStateClosed,
				Title:      "Closed PR",
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
