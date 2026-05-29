package cmd

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
	"github.com/charmbracelet/colorprofile"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pinTestColorProfile(t *testing.T) {
	t.Helper()
	origProfile := lipgloss.Writer.Profile
	origCompatProfile := compat.Profile
	origDark := compat.HasDarkBackground
	setPreviewColorProfile(colorprofile.ASCII)
	compat.HasDarkBackground = true
	t.Cleanup(func() {
		lipgloss.Writer.Profile = origProfile
		compat.Profile = origCompatProfile
		compat.HasDarkBackground = origDark
	})
}

func TestHandlePreviewError(t *testing.T) {
	tests := []struct {
		err        error
		fzfMode    bool
		name       string
		wantErr    bool
		wantOutput string
	}{
		{
			err:        assert.AnError,
			fzfMode:    true,
			name:       "fzf mode prints error to stdout and returns nil",
			wantErr:    false,
			wantOutput: "Error: assert.AnError general error for testing\n",
		},
		{
			err:        assert.AnError,
			fzfMode:    false,
			name:       "normal mode returns error",
			wantErr:    true,
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldFlag := prPreviewFzfFlag
			prPreviewFzfFlag = tt.fzfMode
			defer func() { prPreviewFzfFlag = oldFlag }()

			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)

			resultErr := handlePreviewError(cmd, tt.err)

			if tt.wantErr {
				assert.Error(t, resultErr)
				assert.Equal(t, tt.err, resultErr)
			} else {
				assert.NoError(t, resultErr)
			}

			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

func TestDetectPreviewWidth(t *testing.T) {
	t.Run("FZF_PREVIEW_COLUMNS takes precedence", func(t *testing.T) {
		t.Setenv("FZF_PREVIEW_COLUMNS", "120")
		assert.Equal(t, 120, detectPreviewWidth())
	})

	t.Run("invalid env falls through to terminal or default", func(t *testing.T) {
		t.Setenv("FZF_PREVIEW_COLUMNS", "abc")
		w := detectPreviewWidth()
		assert.Greater(t, w, 0)
	})

	t.Run("zero env falls through to terminal or default", func(t *testing.T) {
		t.Setenv("FZF_PREVIEW_COLUMNS", "0")
		w := detectPreviewWidth()
		assert.Greater(t, w, 0)
	})

	t.Run("no env falls through to terminal or default", func(t *testing.T) {
		w := detectPreviewWidth()
		assert.Greater(t, w, 0)
	})
}

func testPRWithExpanded() github.PullRequest {
	return github.PullRequest{
		AuthorLogin:  "dev",
		BaseRefName:  "main",
		Body:         "Fixes the login bug.",
		BranchName:   "fix/login",
		CreatedAt:    time.Now().Add(-2 * time.Hour),
		FilesChanged: 3,
		Labels:       []github.Label{{Color: "0e8a16", Name: "bug"}, {Color: "1d76db", Name: "priority"}},
		LinesAdded:   25,
		LinesDeleted: 10,
		Number:       42,
		Reviews: []github.Review{
			{AuthorLogin: "reviewer1", State: github.ReviewStateApproved, SubmittedAt: time.Now().Add(-1 * time.Hour)},
			{AuthorLogin: "reviewer2", State: github.ReviewStateChangesRequested, SubmittedAt: time.Now().Add(-30 * time.Minute)},
		},
		State: github.PRStateOpen,
		StatusChecks: []github.StatusCheck{
			{Conclusion: github.CheckConclusionSuccess, DetailURL: "https://github.com/owner/repo/actions/runs/1", Name: "ci/test", Status: github.CheckStatusCompleted},
			{Conclusion: github.CheckConclusionFailure, DetailURL: "https://github.com/owner/repo/actions/runs/2", Name: "ci/lint", Status: github.CheckStatusCompleted},
			{Conclusion: "", Name: "ci/deploy", Status: github.CheckStatusPending},
		},
		Title: "Fix login flow",
		URL:   "https://github.com/owner/repo/pull/42",
	}
}

func testFiles() []github.PullRequestFile {
	return []github.PullRequestFile{
		{Path: "auth.go", Additions: 15, Deletions: 5},
		{Path: "auth_test.go", Additions: 10, Deletions: 5},
	}
}

func TestCheckIcon(t *testing.T) {
	tests := []struct {
		conclusion github.CheckConclusion
		name       string
		wantChar   string
	}{
		{name: "success", conclusion: github.CheckConclusionSuccess, wantChar: iconCheck},
		{name: "failure", conclusion: github.CheckConclusionFailure, wantChar: iconCross},
		{name: "cancelled", conclusion: github.CheckConclusionCancelled, wantChar: iconCross},
		{name: "pending empty", conclusion: "", wantChar: iconPending},
		{name: "timed_out", conclusion: github.CheckConclusionTimedOut, wantChar: iconCross},
		{name: "skipped", conclusion: github.CheckConclusionSkipped, wantChar: iconCross},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkIcon(tt.conclusion)
			assert.Contains(t, result, tt.wantChar)
		})
	}
}

func TestDeduplicateReviews(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		reviews     []github.Review
		wantAuthors []string
		wantStates  []github.ReviewState
	}{
		{
			name:        "empty",
			reviews:     nil,
			wantAuthors: nil,
			wantStates:  nil,
		},
		{
			name: "single review unchanged",
			reviews: []github.Review{
				{AuthorLogin: "alice", State: github.ReviewStateApproved, SubmittedAt: now},
			},
			wantAuthors: []string{"alice"},
			wantStates:  []github.ReviewState{github.ReviewStateApproved},
		},
		{
			name: "same author keeps latest",
			reviews: []github.Review{
				{AuthorLogin: "alice", State: github.ReviewStateCommented, SubmittedAt: now.Add(-2 * time.Hour)},
				{AuthorLogin: "alice", State: github.ReviewStateCommented, SubmittedAt: now.Add(-1 * time.Hour)},
				{AuthorLogin: "alice", State: github.ReviewStateApproved, SubmittedAt: now},
			},
			wantAuthors: []string{"alice"},
			wantStates:  []github.ReviewState{github.ReviewStateApproved},
		},
		{
			name: "multiple authors keep each latest",
			reviews: []github.Review{
				{AuthorLogin: "alice", State: github.ReviewStateCommented, SubmittedAt: now.Add(-3 * time.Hour)},
				{AuthorLogin: "bob", State: github.ReviewStateChangesRequested, SubmittedAt: now.Add(-2 * time.Hour)},
				{AuthorLogin: "alice", State: github.ReviewStateApproved, SubmittedAt: now.Add(-1 * time.Hour)},
			},
			wantAuthors: []string{"bob", "alice"},
			wantStates:  []github.ReviewState{github.ReviewStateChangesRequested, github.ReviewStateApproved},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateReviews(tt.reviews)
			var gotAuthors []string
			var gotStates []github.ReviewState
			for _, r := range result {
				gotAuthors = append(gotAuthors, r.AuthorLogin)
				gotStates = append(gotStates, r.State)
			}
			assert.Equal(t, tt.wantAuthors, gotAuthors)
			assert.Equal(t, tt.wantStates, gotStates)
		})
	}
}

func TestReviewIcon(t *testing.T) {
	tests := []struct {
		name     string
		state    github.ReviewState
		wantChar string
	}{
		{name: "approved", state: github.ReviewStateApproved, wantChar: iconCheck},
		{name: "changes requested", state: github.ReviewStateChangesRequested, wantChar: iconCross},
		{name: "commented", state: github.ReviewStateCommented, wantChar: iconCircle},
		{name: "dismissed", state: github.ReviewStateDismissed, wantChar: iconCircle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reviewIcon(tt.state)
			assert.Contains(t, result, tt.wantChar)
		})
	}
}

func TestLabelColor(t *testing.T) {
	tests := []struct {
		hex  string
		name string
		want color.Color
	}{
		{name: "valid hex", hex: "0e8a16", want: lipgloss.Color("#0e8a16")},
		{name: "with hash", hex: "#1d76db", want: lipgloss.Color("#1d76db")},
		{name: "empty", hex: "", want: colorGray},
		{name: "invalid", hex: "xyz", want: colorGray},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, labelColor(tt.hex))
		})
	}
}

func TestRenderLabels(t *testing.T) {
	pinTestColorProfile(t)
	tests := []struct {
		labels       []github.Label
		name         string
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:      "empty labels",
			labels:    nil,
			wantEmpty: true,
		},
		{
			name:         "single label",
			labels:       []github.Label{{Color: "0e8a16", Name: "bug"}},
			wantContains: []string{"bug"},
		},
		{
			name:         "multiple labels",
			labels:       []github.Label{{Color: "0e8a16", Name: "bug"}, {Color: "1d76db", Name: "feature"}},
			wantContains: []string{"bug", "feature", "·"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderLabels(tt.labels)
			if tt.wantEmpty {
				assert.Empty(t, result)
			} else {
				for _, want := range tt.wantContains {
					assert.Contains(t, result, want)
				}
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{name: "just now", time: time.Now().Add(-1 * time.Second), want: "just now"},
		{name: "minutes ago", time: time.Now().Add(-5*time.Minute - 5*time.Second), want: "5 mins ago"},
		{name: "1 min ago", time: time.Now().Add(-1*time.Minute - 5*time.Second), want: "1 min ago"},
		{name: "hours ago", time: time.Now().Add(-3*time.Hour - 5*time.Second), want: "3 hours ago"},
		{name: "1 hour ago", time: time.Now().Add(-1*time.Hour - 5*time.Second), want: "1 hour ago"},
		{name: "days ago", time: time.Now().Add(-48*time.Hour - 5*time.Second), want: "2 days ago"},
		{name: "1 day ago", time: time.Now().Add(-25*time.Hour - 5*time.Second), want: "1 day ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, relativeTime(tt.time))
		})
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "Go test file", path: "foo_test.go", want: true},
		{name: "Go test file in dir", path: "pkg/foo_test.go", want: true},
		{name: "Java test file", path: "FooTest.java", want: true},
		{name: "Java tests file", path: "FooTests.java", want: true},
		{name: "Java IT file", path: "FooIT.java", want: true},
		{name: "path with /test/", path: "src/test/Bar.java", want: true},
		{name: "path with /tests/", path: "src/tests/helper.go", want: true},
		{name: "path with /testdata/", path: "pkg/testdata/fixture.json", want: true},
		{name: "top-level test dir", path: "test/Bar.java", want: true},
		{name: "top-level tests dir", path: "tests/helper.go", want: true},
		{name: "top-level testdata dir", path: "testdata/fixture.json", want: true},
		{name: "regular Go file", path: "main.go", want: false},
		{name: "TestHelper not a test", path: "TestHelper.go", want: false},
		{name: "Java non-test", path: "Testing.java", want: false},
		{name: "README", path: "README.md", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTestFile(tt.path))
		})
	}
}

func testReviewFiles() []github.PullRequestFile {
	return []github.PullRequestFile{
		{Path: "cmd/server.go", Additions: 150, Deletions: 20},
		{Path: "cmd/server_test.go", Additions: 80, Deletions: 10},
		{Path: "internal/auth.go", Additions: 50, Deletions: 5},
		{Path: "internal/auth_test.go", Additions: 30, Deletions: 3},
		{Path: "config.go", Additions: 10, Deletions: 2},
	}
}

func testFileComments() map[string]int {
	return map[string]int{
		"cmd/server.go":    3,
		"internal/auth.go": 1,
	}
}

func testTimelineEvents() []github.TimelineEvent {
	return []github.TimelineEvent{
		{Actor: "dev", CreatedAt: time.Now().Add(-90 * time.Minute), Details: "initial commit", Type: github.TimelineEventCommitted},
		{Actor: "reviewer1", CreatedAt: time.Now().Add(-1 * time.Hour), Details: "approved", Type: github.TimelineEventReviewed},
		{Actor: "merger", CreatedAt: time.Now().Add(-30 * time.Minute), Type: github.TimelineEventMerged},
	}
}

func TestRenderPreview(t *testing.T) {
	pinTestColorProfile(t)
	pr := testPRWithExpanded()
	pr.Files = testReviewFiles()
	comments := testFileComments()
	timeline := testTimelineEvents()

	tests := []struct {
		comments     map[string]int
		name         string
		pr           github.PullRequest
		timeline     []github.TimelineEvent
		wantContains []string
	}{
		{
			name:     "full PR with all data",
			pr:       pr,
			comments: comments,
			timeline: timeline,
			wantContains: []string{
				"#42",
				"Fix login flow",
				"dev",
				"fix/login",
				"main",
				"OPEN",
				"bug",
				"priority",
				"ci/test",
				"ci/lint",
				"reviewer1",
				"reviewer2",
				"server.go",
				"auth.go",
				"High Activity Files",
				"Activity",
			},
		},
		{
			name: "no labels reviews or checks",
			pr: func() github.PullRequest {
				p := pr
				p.Labels = nil
				p.Reviews = nil
				p.StatusChecks = nil
				return p
			}(),
			comments: comments,
			timeline: timeline,
			wantContains: []string{
				"#42", "Fix login flow", "server.go",
			},
		},
		{
			name: "empty body",
			pr: func() github.PullRequest {
				p := pr
				p.Body = ""
				return p
			}(),
			comments:     comments,
			timeline:     timeline,
			wantContains: []string{"#42", "Fix login flow"},
		},
		{
			name: "merged state",
			pr: func() github.PullRequest {
				p := pr
				p.State = github.PRStateMerged
				return p
			}(),
			comments:     comments,
			timeline:     timeline,
			wantContains: []string{"MERGED"},
		},
		{
			name: "empty timeline and comments",
			pr: func() github.PullRequest {
				p := pr
				p.Files = testFiles()
				return p
			}(),
			comments:     map[string]int{},
			timeline:     nil,
			wantContains: []string{"#42", "auth.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := renderPreview(&buf, tt.pr, tt.comments, tt.timeline, 60, "auto")
			require.NoError(t, err)

			output := buf.String()
			assert.NotEmpty(t, output)
			for _, want := range tt.wantContains {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestRenderHighActivity(t *testing.T) {
	pinTestColorProfile(t)
	files := testReviewFiles()
	comments := testFileComments()

	scored := scoreFiles(files, comments)
	output, shown := renderHighActivity(scored, "", 56)

	assert.Contains(t, output, "High Activity Files")
	assert.Contains(t, output, "server.go", "highest churn non-test file should appear")
	assert.Contains(t, output, "auth.go", "second highest non-test file should appear")
	assert.NotContains(t, output, "server_test.go", "test files should be excluded")
	assert.NotContains(t, output, "auth_test.go", "test files should be excluded")
	assert.NotEmpty(t, shown)

	testOnlyFiles := []github.PullRequestFile{
		{Path: "foo_test.go", Additions: 100, Deletions: 50},
	}
	output, shown = renderHighActivity(scoreFiles(testOnlyFiles, map[string]int{}), "", 56)
	assert.Empty(t, output)
	assert.Nil(t, shown)
}

func TestRenderFileComments(t *testing.T) {
	pinTestColorProfile(t)
	files := []github.PullRequestFile{
		{Path: "with_comments.go", Additions: 10, Deletions: 5},
		{Path: "no_comments.go", Additions: 3, Deletions: 1},
	}
	comments := map[string]int{
		"with_comments.go": 3,
	}
	cw := computeFileColumnWidths(files, comments)

	output := formatFileEntry(files[0], comments["with_comments.go"], "", 56, cw)
	assert.Contains(t, output, iconComment)
	assert.Contains(t, output, "3")

	output = formatFileEntry(files[1], comments["no_comments.go"], "", 56, cw)
	assert.NotContains(t, output, iconComment)
}

func TestFormatFileEntryAlignment(t *testing.T) {
	files := []github.PullRequestFile{
		{Path: "small.go", Additions: 3, Deletions: 1},
		{Path: "large.go", Additions: 181, Deletions: 42},
	}
	cw := computeFileColumnWidths(files, nil)

	out1 := formatFileEntry(files[0], 0, "", 80, cw)
	out2 := formatFileEntry(files[1], 0, "", 80, cw)

	assert.Contains(t, out1, "  +3")
	assert.Contains(t, out2, "+181")
	assert.Contains(t, out1, " -1")
	assert.Contains(t, out2, "-42")
}

func TestRenderBody(t *testing.T) {
	pinTestColorProfile(t)
	tests := []struct {
		body         string
		name         string
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:      "empty body returns empty",
			body:      "",
			wantEmpty: true,
		},
		{
			name:         "markdown is rendered with ANSI",
			body:         "## Heading\n\nSome **bold** text.",
			wantContains: []string{"Heading", "bold"},
		},
		{
			name:         "plain text passes through",
			body:         "Just some plain text.",
			wantContains: []string{"plain text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := renderBody(tt.body, 60, "auto")
			if tt.wantEmpty {
				assert.Empty(t, output)
			} else {
				for _, want := range tt.wantContains {
					assert.Contains(t, output, want)
				}
			}
		})
	}
}

func TestRenderTimeline(t *testing.T) {
	pinTestColorProfile(t)
	pr := testPRWithExpanded()

	t.Run("events are ordered chronologically", func(t *testing.T) {
		timeline := []github.TimelineEvent{
			{Actor: "dev", CreatedAt: time.Now().Add(-90 * time.Minute), Details: "initial commit", Type: github.TimelineEventCommitted},
			{Actor: "reviewer1", CreatedAt: time.Now().Add(-1 * time.Hour), Details: "approved", Type: github.TimelineEventReviewed},
			{Actor: "merger", CreatedAt: time.Now().Add(-30 * time.Minute), Type: github.TimelineEventMerged},
		}

		output := renderTimeline(pr, timeline)

		assert.Contains(t, output, "Activity")
		assert.Contains(t, output, "opened this PR")
		assert.Contains(t, output, "initial commit")
		assert.Contains(t, output, "@reviewer1 approved")
		assert.Contains(t, output, "@merger merged")

		openedIdx := strings.Index(output, "opened this PR")
		commitIdx := strings.Index(output, "initial commit")
		approvedIdx := strings.Index(output, "@reviewer1 approved")
		mergedIdx := strings.Index(output, "@merger merged")
		assert.Less(t, openedIdx, commitIdx)
		assert.Less(t, commitIdx, approvedIdx)
		assert.Less(t, approvedIdx, mergedIdx)
	})

	t.Run("empty PR and timeline returns empty", func(t *testing.T) {
		output := renderTimeline(github.PullRequest{}, nil)
		assert.Empty(t, output)
	})

	t.Run("consecutive commits are collapsed", func(t *testing.T) {
		timeline := []github.TimelineEvent{
			{Actor: "dev", CreatedAt: time.Now().Add(-90 * time.Minute), Details: "first", Type: github.TimelineEventCommitted},
			{Actor: "dev", CreatedAt: time.Now().Add(-85 * time.Minute), Details: "second", Type: github.TimelineEventCommitted},
			{Actor: "dev", CreatedAt: time.Now().Add(-80 * time.Minute), Details: "third", Type: github.TimelineEventCommitted},
			{Actor: "reviewer1", CreatedAt: time.Now().Add(-1 * time.Hour), Details: "approved", Type: github.TimelineEventReviewed},
		}
		output := renderTimeline(pr, timeline)
		assert.Contains(t, output, "pushed 3 commits")
		assert.NotContains(t, output, "first")
		assert.NotContains(t, output, "second")
	})
}

func TestHighActivityIncludesCommentedFiles(t *testing.T) {
	files := []github.PullRequestFile{
		{Path: "high_churn1.go", Additions: 200, Deletions: 100},
		{Path: "high_churn2.go", Additions: 150, Deletions: 50},
		{Path: "high_churn3.go", Additions: 100, Deletions: 50},
		{Path: "low_churn_commented.go", Additions: 2, Deletions: 1},
	}
	comments := map[string]int{
		"low_churn_commented.go": 5,
	}

	scored := scoreFiles(files, comments)
	output, shown := renderHighActivity(scored, "", 80)

	assert.Contains(t, output, "low_churn_commented.go", "low-churn file with comments should appear")
	assert.Contains(t, output, "High Activity Files")

	var shownPaths []string
	for _, sf := range shown {
		shownPaths = append(shownPaths, sf.file.Path)
	}
	assert.Contains(t, shownPaths, "low_churn_commented.go")
}

func TestRenderStateBadge(t *testing.T) {
	pinTestColorProfile(t)
	pr := testPRWithExpanded()

	output := renderHeader(pr)
	assert.Contains(t, output, "OPEN")
}

func TestDetectDarkBackgroundFromEnv(t *testing.T) {
	tests := []struct {
		env      string
		name     string
		wantDark bool
		wantOK   bool
	}{
		{name: "unset", env: "", wantDark: false, wantOK: false},
		{name: "no semicolon", env: "15", wantDark: false, wantOK: false},
		{name: "black bg is dark", env: "15;0", wantDark: true, wantOK: true},
		{name: "white bg is light", env: "0;15", wantDark: false, wantOK: true},
		{name: "color 7 is light", env: "0;7", wantDark: false, wantOK: true},
		{name: "color 8 is dark", env: "0;8", wantDark: true, wantOK: true},
		{name: "color 6 is dark", env: "0;6", wantDark: true, wantOK: true},
		{name: "color 9 is light", env: "0;9", wantDark: false, wantOK: true},
		{name: "three parts uses last", env: "37;0;15", wantDark: false, wantOK: true},
		{name: "non-numeric bg", env: "0;abc", wantDark: false, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COLORFGBG", tt.env)
			dark, ok := detectDarkBackgroundFromEnv()
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantDark, dark)
			}
		})
	}
}

func TestFileDiffURL(t *testing.T) {
	got := fileDiffURL("https://github.com/owner/repo/pull/42", "README.md")
	assert.Equal(t, "https://github.com/owner/repo/pull/42/files#diff-b335630551682c19a781afebcf4d07bf978fb1f8ac04c6bf87428ed5106870f5", got)
}

func TestHighActivityCountInHeader(t *testing.T) {
	pinTestColorProfile(t)
	files := testReviewFiles()
	comments := testFileComments()

	scored := scoreFiles(files, comments)
	output, shown := renderHighActivity(scored, "", 56)

	assert.Contains(t, output, fmt.Sprintf("High Activity Files (%d)", len(shown)))
}
