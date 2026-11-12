package github

import (
	"encoding/json"
	"testing"

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
			name: "open",
			input: `{
				"number": 123,
				"state": "OPEN",
				"stateReason": "",
				"title": "Fix login crash"
			}`,
			want: Issue{
				Number: 123,
				State:  IssueStateOpen,
				Title:  "Fix login crash",
			},
		},
		{
			name: "closed as not planned",
			input: `{
				"number": 7,
				"state": "CLOSED",
				"stateReason": "NOT_PLANNED",
				"title": "Old idea"
			}`,
			want: Issue{
				Number:      7,
				State:       IssueStateClosed,
				StateReason: IssueStateReasonNotPlanned,
				Title:       "Old idea",
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
