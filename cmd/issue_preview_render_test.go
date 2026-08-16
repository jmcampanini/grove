package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/grove/internal/github"
	"github.com/stretchr/testify/assert"
)

func TestIssueStateText(t *testing.T) {
	tests := []struct {
		name  string
		issue github.Issue
		want  string
	}{
		{
			name:  "open",
			issue: github.Issue{State: github.IssueStateOpen},
			want:  "OPEN",
		},
		{
			name:  "closed as completed",
			issue: github.Issue{State: github.IssueStateClosed, StateReason: github.IssueStateReasonCompleted},
			want:  "CLOSED",
		},
		{
			name:  "closed as not planned",
			issue: github.Issue{State: github.IssueStateClosed, StateReason: github.IssueStateReasonNotPlanned},
			want:  "NOT PLANNED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, issueStateText(tt.issue))
		})
	}
}

func TestIssueStateColor(t *testing.T) {
	assert.Equal(t, colorGreen, issueStateColor(github.Issue{State: github.IssueStateOpen}))
	assert.Equal(t, colorPurple, issueStateColor(github.Issue{State: github.IssueStateClosed, StateReason: github.IssueStateReasonCompleted}))
	assert.Equal(t, colorGray, issueStateColor(github.Issue{State: github.IssueStateClosed, StateReason: github.IssueStateReasonNotPlanned}))
}

func TestRenderIssueComments(t *testing.T) {
	comment := func(author, body string, minsAgo int) github.IssueComment {
		return github.IssueComment{
			AuthorLogin: author,
			Body:        body,
			CreatedAt:   time.Now().Add(-time.Duration(minsAgo) * time.Minute),
		}
	}

	t.Run("no comments renders nothing", func(t *testing.T) {
		assert.Empty(t, noTTYPreviewRenderer().renderIssueComments(nil, 80))
	})

	t.Run("renders author and body", func(t *testing.T) {
		got := noTTYPreviewRenderer().renderIssueComments([]github.IssueComment{
			comment("bob", "Reproduced on macOS.", 10),
		}, 80)
		assert.Contains(t, got, "Comments (1)")
		assert.Contains(t, got, "@bob")
		assert.Contains(t, got, "Reproduced on macOS.")
	})

	t.Run("only the most recent comments are shown", func(t *testing.T) {
		comments := []github.IssueComment{
			comment("a", "first comment", 70),
			comment("b", "second comment", 60),
			comment("c", "third comment", 50),
			comment("d", "fourth comment", 40),
			comment("e", "fifth comment", 30),
			comment("f", "sixth comment", 20),
			comment("g", "seventh comment", 10),
		}
		got := noTTYPreviewRenderer().renderIssueComments(comments, 80)
		assert.Contains(t, got, "Comments (7)")
		assert.Contains(t, got, "2 earlier comments not shown")
		assert.NotContains(t, got, "first comment")
		assert.NotContains(t, got, "second comment")
		assert.Contains(t, got, "third comment")
		assert.Contains(t, got, "seventh comment")
		assert.Equal(t, 1, strings.Count(got, "earlier comments not shown"))
	})
}
