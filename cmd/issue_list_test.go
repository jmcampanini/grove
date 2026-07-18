package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/issue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatIssueLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels []github.Label
		want   string
	}{
		{
			name:   "no labels",
			labels: nil,
			want:   "",
		},
		{
			name:   "one label",
			labels: []github.Label{{Name: "bug"}},
			want:   "bug",
		},
		{
			name:   "two labels",
			labels: []github.Label{{Name: "bug"}, {Name: "ui"}},
			want:   "bug, ui",
		},
		{
			name: "overflow collapses into count",
			labels: []github.Label{
				{Name: "bug"}, {Name: "ui"}, {Name: "a11y"}, {Name: "dark-mode"},
			},
			want: "bug, ui +2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatIssueLabels(tt.labels))
		})
	}
}

func issueMatch(number int, title, author string, labels []github.Label, worktreePath string) issue.Match {
	return issue.Match{
		Issue: github.Issue{
			AuthorLogin: author,
			Labels:      labels,
			Number:      number,
			State:       github.IssueStateOpen,
			Title:       title,
			UpdatedAt:   time.Now().Add(-2 * time.Hour),
		},
		WorktreePath: worktreePath,
	}
}

func TestOutputIssueListFzf(t *testing.T) {
	tests := []struct {
		name      string
		matches   []issue.Match
		wantLines []string
	}{
		{
			name:      "empty list produces no output",
			matches:   nil,
			wantLines: nil,
		},
		{
			name: "issue without worktree",
			matches: []issue.Match{
				issueMatch(123, "Fix login crash", "alice", []github.Label{{Name: "bug"}}, ""),
			},
			wantLines: []string{
				"123\t123 Fix login crash bug alice open\t#123 Fix login crash [alice] bug",
			},
		},
		{
			name: "issue with worktree gets checkmark prefix",
			matches: []issue.Match{
				issueMatch(7, "Add dark mode", "bob", nil, "/workspace/is-7-add-dark-mode"),
			},
			wantLines: []string{
				"7\t7 Add dark mode  bob open\t✓ #7 Add dark mode [bob] ",
			},
		},
		{
			name: "tabs and newlines in title are sanitized",
			matches: []issue.Match{
				issueMatch(9, "Fix\tweird\ntitle", "carol", nil, ""),
			},
			wantLines: []string{
				"9\t9 Fix weird title  carol open\t#9 Fix weird title [carol] ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			require.NoError(t, outputIssueListFzf(&buf, tt.matches))

			if tt.wantLines == nil {
				assert.Empty(t, buf.String())
				return
			}

			gotLines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
			require.Len(t, gotLines, len(tt.wantLines))
			for i, want := range tt.wantLines {
				assert.Equal(t, want, gotLines[i])
			}
		})
	}
}

func TestOutputIssueListTable(t *testing.T) {
	t.Run("empty list prints message", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, outputIssueListTable(&buf, nil))
		assert.Contains(t, buf.String(), "No open issues found.")
	})

	t.Run("table contains issue fields", func(t *testing.T) {
		var buf bytes.Buffer
		matches := []issue.Match{
			issueMatch(123, "Fix login crash", "alice", []github.Label{{Name: "bug"}, {Name: "ui"}, {Name: "a11y"}}, "/workspace/is-123"),
			issueMatch(456, "Add dark mode", "bob", nil, ""),
		}
		require.NoError(t, outputIssueListTable(&buf, matches))

		out := buf.String()
		assert.Contains(t, out, "123")
		assert.Contains(t, out, "Fix login crash")
		assert.Contains(t, out, "alice")
		assert.Contains(t, out, "bug, ui +1")
		assert.Contains(t, out, "✓")
		assert.Contains(t, out, "Labels")
	})
}
