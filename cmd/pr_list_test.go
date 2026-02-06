package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/pr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeFzfField(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no special characters",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "tabs replaced with spaces",
			input: "hello\tworld",
			want:  "hello world",
		},
		{
			name:  "newlines replaced with spaces",
			input: "hello\nworld",
			want:  "hello world",
		},
		{
			name:  "carriage returns replaced with spaces",
			input: "hello\rworld",
			want:  "hello world",
		},
		{
			name:  "multiple special characters",
			input: "hello\t\n\rworld",
			want:  "hello   world",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFzfField(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "string shorter than maxLen",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "string equal to maxLen",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "string longer than maxLen",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "maxLen less than 3",
			input:  "hello",
			maxLen: 2,
			want:   "he",
		},
		{
			name:   "maxLen exactly 3",
			input:  "hello",
			maxLen: 3,
			want:   "hel",
		},
		{
			name:   "maxLen of 4",
			input:  "hello world",
			maxLen: 4,
			want:   "h...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOutputPRListFzf(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		matches      []pr.WorktreeMatch
		wantContains []string
		wantLines    int
	}{
		{
			name:         "empty list",
			matches:      []pr.WorktreeMatch{},
			wantContains: []string{},
			wantLines:    0,
		},
		{
			name: "single PR without worktree",
			matches: []pr.WorktreeMatch{
				{
					PR: github.PullRequest{
						AuthorLogin: "jsmith",
						BranchName:  "feature/add-auth",
						Number:      123,
						State:       github.PRStateOpen,
						Title:       "Add authentication",
						UpdatedAt:   now,
					},
					HasWorktree: false,
				},
			},
			wantContains: []string{
				"123\t",                   // Column 1: PR number
				"123 Add authentication",  // Part of searchable column
				"#123 Add authentication", // Part of display column
				"[jsmith]",                // Author in display
				"feature/add-auth",        // Branch in both columns
				"open",                    // State in searchable
			},
			wantLines: 1,
		},
		{
			name: "single PR with worktree",
			matches: []pr.WorktreeMatch{
				{
					PR: github.PullRequest{
						AuthorLogin: "jsmith",
						BranchName:  "feature/add-auth",
						Number:      123,
						State:       github.PRStateOpen,
						Title:       "Add authentication",
						UpdatedAt:   now,
					},
					HasWorktree:  true,
					WorktreePath: "/path/to/worktree",
				},
			},
			wantContains: []string{
				"\u2713 #123", // Checkmark before PR number in display
			},
			wantLines: 1,
		},
		{
			name: "multiple PRs",
			matches: []pr.WorktreeMatch{
				{
					PR: github.PullRequest{
						AuthorLogin: "user1",
						BranchName:  "fix/bug",
						Number:      100,
						State:       github.PRStateDraft,
						Title:       "Fix bug",
						UpdatedAt:   now,
					},
					HasWorktree: false,
				},
				{
					PR: github.PullRequest{
						AuthorLogin: "user2",
						BranchName:  "feature/new",
						Number:      101,
						State:       github.PRStateOpen,
						Title:       "New feature",
						UpdatedAt:   now,
					},
					HasWorktree: true,
				},
			},
			wantContains: []string{
				"100\t",
				"101\t",
				"draft",
				"open",
			},
			wantLines: 2,
		},
		{
			name: "PR with special characters in title",
			matches: []pr.WorktreeMatch{
				{
					PR: github.PullRequest{
						AuthorLogin: "dev",
						BranchName:  "fix/issue",
						Number:      200,
						State:       github.PRStateOpen,
						Title:       "Fix\ttab\nand\rnewline",
						UpdatedAt:   now,
					},
					HasWorktree: false,
				},
			},
			wantContains: []string{
				"Fix tab and newline", // Special chars replaced with spaces
			},
			wantLines: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			err := outputPRListFzf(&buf, tt.matches)
			require.NoError(t, err)

			output := buf.String()

			// Check line count
			if tt.wantLines == 0 {
				assert.Empty(t, output)
			} else {
				lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
				assert.Len(t, lines, tt.wantLines)
			}

			// Check expected content
			for _, want := range tt.wantContains {
				assert.Contains(t, output, want, "output should contain %q", want)
			}

			// Verify TSV format (3 columns per line)
			if tt.wantLines > 0 {
				lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
				for i, line := range lines {
					cols := strings.Split(line, "\t")
					assert.Len(t, cols, 3, "line %d should have 3 tab-separated columns", i)
				}
			}
		})
	}
}

func TestOutputPRListTable(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		matches      []pr.WorktreeMatch
		wantContains []string
	}{
		{
			name:    "empty list shows message",
			matches: []pr.WorktreeMatch{},
			wantContains: []string{
				"No open pull requests found.",
			},
		},
		{
			name: "single PR renders table",
			matches: []pr.WorktreeMatch{
				{
					PR: github.PullRequest{
						AuthorLogin: "jsmith",
						BranchName:  "feature/add-auth",
						Number:      123,
						State:       github.PRStateOpen,
						Title:       "Add authentication",
						UpdatedAt:   now,
					},
					HasWorktree: true,
				},
			},
			wantContains: []string{
				"#",       // Header
				"Title",   // Header
				"Author",  // Header
				"Branch",  // Header
				"State",   // Header
				"Local",   // Header
				"Updated", // Header
				"123",
				"Add authentication",
				"jsmith",
				"feature/add-auth",
				"open",
				"\u2713", // Checkmark for local worktree
			},
		},
		{
			name: "draft state shows lowercase",
			matches: []pr.WorktreeMatch{
				{
					PR: github.PullRequest{
						AuthorLogin: "dev",
						BranchName:  "fix/issue",
						Number:      456,
						State:       github.PRStateDraft,
						Title:       "Draft PR",
						UpdatedAt:   now,
					},
					HasWorktree: false,
				},
			},
			wantContains: []string{
				"draft",
			},
		},
		{
			name: "long title truncated",
			matches: []pr.WorktreeMatch{
				{
					PR: github.PullRequest{
						AuthorLogin: "dev",
						BranchName:  "feature/x",
						Number:      789,
						State:       github.PRStateOpen,
						Title:       "This is a very long title that exceeds the maximum display width",
						UpdatedAt:   now,
					},
					HasWorktree: false,
				},
			},
			wantContains: []string{
				"...", // Truncation indicator
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			err := outputPRListTable(&buf, tt.matches)
			require.NoError(t, err)

			output := buf.String()
			for _, want := range tt.wantContains {
				assert.Contains(t, output, want, "output should contain %q", want)
			}
		})
	}
}

func TestOutputPRListFzfStateFormats(t *testing.T) {
	now := time.Now()

	// Test that all PR states are formatted as lowercase
	states := []struct {
		state    github.PRState
		wantText string
	}{
		{github.PRStateOpen, "open"},
		{github.PRStateDraft, "draft"},
		{github.PRStateClosed, "closed"},
		{github.PRStateMerged, "merged"},
	}

	for _, st := range states {
		t.Run(string(st.state), func(t *testing.T) {
			matches := []pr.WorktreeMatch{
				{
					PR: github.PullRequest{
						AuthorLogin: "test",
						BranchName:  "test-branch",
						Number:      1,
						State:       st.state,
						Title:       "Test",
						UpdatedAt:   now,
					},
					HasWorktree: false,
				},
			}

			var buf bytes.Buffer

			err := outputPRListFzf(&buf, matches)
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, st.wantText, "searchable column should contain lowercase state %q", st.wantText)
		})
	}
}
