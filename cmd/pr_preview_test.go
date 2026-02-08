package cmd

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputPRPreview(t *testing.T) {
	now := time.Now()

	tests := []struct {
		files           []github.PullRequestFile
		name            string
		pr              github.PullRequest
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "basic PR with no files",
			pr: github.PullRequest{
				AuthorLogin:  "jsmith",
				Body:         "This is the PR description.",
				BranchName:   "feature/add-auth",
				FilesChanged: 0,
				Number:       123,
				State:        github.PRStateOpen,
				Title:        "Add authentication",
				UpdatedAt:    now,
			},
			files: []github.PullRequestFile{},
			wantContains: []string{
				"PR #123",
				"Title:  Add authentication",
				"Author: jsmith",
				"Branch: feature/add-auth",
				"State:  open",
				"Files changed (0):",
				"This is the PR description.",
			},
		},
		{
			name: "PR with files",
			pr: github.PullRequest{
				AuthorLogin:  "developer",
				Body:         "Fixed the bug.",
				BranchName:   "fix/bug",
				FilesChanged: 3,
				Number:       456,
				State:        github.PRStateDraft,
				Title:        "Fix critical bug",
				UpdatedAt:    now,
			},
			files: []github.PullRequestFile{
				{Path: "main.go", Additions: 10, Deletions: 5},
				{Path: "utils/helper.go", Additions: 20, Deletions: 0},
				{Path: "README.md", Additions: 3, Deletions: 1},
			},
			wantContains: []string{
				"PR #456",
				"Title:  Fix critical bug",
				"Author: developer",
				"Branch: fix/bug",
				"State:  draft",
				"Files changed (3):",
				"main.go (+10, -5)",
				"utils/helper.go (+20, -0)",
				"README.md (+3, -1)",
				"Fixed the bug.",
			},
		},
		{
			name: "PR states formatted lowercase",
			pr: github.PullRequest{
				AuthorLogin:  "user",
				Body:         "",
				BranchName:   "branch",
				FilesChanged: 0,
				Number:       1,
				State:        github.PRStateMerged,
				Title:        "Merged PR",
				UpdatedAt:    now,
			},
			files: []github.PullRequestFile{},
			wantContains: []string{
				"State:  merged",
			},
		},
		{
			name: "closed state formatted lowercase",
			pr: github.PullRequest{
				AuthorLogin:  "user",
				Body:         "",
				BranchName:   "branch",
				FilesChanged: 0,
				Number:       2,
				State:        github.PRStateClosed,
				Title:        "Closed PR",
				UpdatedAt:    now,
			},
			files: []github.PullRequestFile{},
			wantContains: []string{
				"State:  closed",
			},
		},
		{
			name: "horizontal line separator",
			pr: github.PullRequest{
				AuthorLogin:  "user",
				Body:         "",
				BranchName:   "branch",
				FilesChanged: 0,
				Number:       100,
				State:        github.PRStateOpen,
				Title:        "Test",
				UpdatedAt:    now,
			},
			files: []github.PullRequestFile{},
			wantContains: []string{
				"\u2500", // horizontal line character
			},
		},
		{
			name: "PR with empty body",
			pr: github.PullRequest{
				AuthorLogin:  "user",
				Body:         "",
				BranchName:   "branch",
				FilesChanged: 0,
				Number:       200,
				State:        github.PRStateOpen,
				Title:        "No description",
				UpdatedAt:    now,
			},
			files: []github.PullRequestFile{},
			wantContains: []string{
				"PR #200",
				"Title:  No description",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			err := outputPRPreview(&buf, tt.pr, tt.files)
			require.NoError(t, err)

			output := buf.String()

			for _, want := range tt.wantContains {
				assert.Contains(t, output, want, "output should contain %q", want)
			}

			for _, notWant := range tt.wantNotContains {
				assert.NotContains(t, output, notWant, "output should not contain %q", notWant)
			}
		})
	}
}

func TestOutputPRPreviewFileLimit(t *testing.T) {
	now := time.Now()

	// Create more than 30 files to test truncation
	files := make([]github.PullRequestFile, 35)
	for i := 0; i < 35; i++ {
		files[i] = github.PullRequestFile{
			Additions: i + 1,
			Deletions: i,
			Path:      strings.Repeat("a", i+1) + ".go",
		}
	}

	pr := github.PullRequest{
		AuthorLogin:  "user",
		Body:         "Description",
		BranchName:   "branch",
		FilesChanged: 35,
		Number:       100,
		State:        github.PRStateOpen,
		Title:        "Many files",
		UpdatedAt:    now,
	}

	var buf bytes.Buffer

	err := outputPRPreview(&buf, pr, files)
	require.NoError(t, err)

	output := buf.String()

	// Should show first 30 files
	assert.Contains(t, output, "a.go (+1, -0)") // first file
	assert.Contains(t, output, "(and 5 more files...)")

	// Should show correct total count
	assert.Contains(t, output, "Files changed (35):")

	// Count actual file lines displayed (each file line starts with "  " and contains ".go")
	lines := strings.Split(output, "\n")
	fileLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "  ") && strings.Contains(line, ".go") {
			fileLines++
		}
	}
	assert.Equal(t, 30, fileLines, "should display exactly 30 file lines")
}

func TestOutputPRPreviewExactly30Files(t *testing.T) {
	now := time.Now()

	// Create exactly 30 files - should NOT show "more files" message
	files := make([]github.PullRequestFile, 30)
	for i := 0; i < 30; i++ {
		files[i] = github.PullRequestFile{
			Additions: i + 1,
			Deletions: i,
			Path:      strings.Repeat("b", i+1) + ".go",
		}
	}

	pr := github.PullRequest{
		AuthorLogin:  "user",
		Body:         "Description",
		BranchName:   "branch",
		FilesChanged: 30,
		Number:       100,
		State:        github.PRStateOpen,
		Title:        "Exactly 30 files",
		UpdatedAt:    now,
	}

	var buf bytes.Buffer

	err := outputPRPreview(&buf, pr, files)
	require.NoError(t, err)

	output := buf.String()

	// Should NOT show "more files" message
	assert.NotContains(t, output, "more files")
	assert.Contains(t, output, "Files changed (30):")
}

func TestHandlePreviewError(t *testing.T) {
	tests := []struct {
		name       string
		fzfMode    bool
		err        error
		wantErr    bool
		wantOutput string
	}{
		{
			name:       "fzf mode prints error to stdout and returns nil",
			fzfMode:    true,
			err:        assert.AnError,
			wantErr:    false,
			wantOutput: "Error: assert.AnError general error for testing\n",
		},
		{
			name:       "normal mode returns error",
			fzfMode:    false,
			err:        assert.AnError,
			wantErr:    true,
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global flag
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

func TestIsValidPreviewStyle(t *testing.T) {
	tests := []struct {
		name  string
		style string
		want  bool
	}{
		{name: "card", style: "card", want: true},
		{name: "dashboard", style: "dashboard", want: true},
		{name: "minimal", style: "minimal", want: true},
		{name: "context", style: "context", want: true},
		{name: "board", style: "board", want: true},
		{name: "timeline", style: "timeline", want: true},
		{name: "review", style: "review", want: true},
		{name: "empty", style: "", want: false},
		{name: "invalid", style: "fancy", want: false},
		{name: "uppercase", style: "CARD", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidPreviewStyle(tt.style))
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

func testPR() github.PullRequest {
	return github.PullRequest{
		AuthorLogin:  "dev",
		Body:         "Fixes the login bug.",
		BranchName:   "fix/login",
		FilesChanged: 3,
		LinesAdded:   25,
		LinesDeleted: 10,
		Number:       42,
		State:        github.PRStateOpen,
		Title:        "Fix login flow",
	}
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
			{AuthorLogin: "reviewer1", State: "APPROVED", SubmittedAt: time.Now().Add(-1 * time.Hour)},
			{AuthorLogin: "reviewer2", State: "CHANGES_REQUESTED", SubmittedAt: time.Now().Add(-30 * time.Minute)},
		},
		State: github.PRStateOpen,
		StatusChecks: []github.StatusCheck{
			{Conclusion: "success", Name: "ci/test", Status: "COMPLETED"},
			{Conclusion: "failure", Name: "ci/lint", Status: "COMPLETED"},
			{Conclusion: "", Name: "ci/deploy", Status: "PENDING"},
		},
		Title: "Fix login flow",
	}
}

func testFiles() []github.PullRequestFile {
	return []github.PullRequestFile{
		{Path: "auth.go", Additions: 15, Deletions: 5},
		{Path: "auth_test.go", Additions: 10, Deletions: 5},
	}
}

func TestRenderGroupAStyles(t *testing.T) {
	renderers := []struct {
		name   string
		render func(io.Writer, github.PullRequest, []github.PullRequestFile, int) error
	}{
		{"card", renderCard},
		{"dashboard", renderDashboard},
		{"minimal", renderMinimal},
	}

	pr := testPR()
	files := testFiles()

	for _, r := range renderers {
		t.Run(r.name, func(t *testing.T) {
			tests := []struct {
				files        []github.PullRequestFile
				name         string
				pr           github.PullRequest
				wantContains []string
			}{
				{
					name:  "normal PR",
					pr:    pr,
					files: files,
					wantContains: []string{
						"#42",
						"Fix login flow",
						"dev",
						"fix/login",
						"open",
						"auth.go",
						"+15",
						"-5",
					},
				},
				{
					name: "empty body",
					pr: func() github.PullRequest {
						p := pr
						p.Body = ""
						return p
					}(),
					files:        files,
					wantContains: []string{"#42", "Fix login flow"},
				},
				{
					name: "zero files",
					pr: func() github.PullRequest {
						p := pr
						p.FilesChanged = 0
						return p
					}(),
					files:        []github.PullRequestFile{},
					wantContains: []string{"#42"},
				},
				{
					name: "draft state",
					pr: func() github.PullRequest {
						p := pr
						p.State = github.PRStateDraft
						return p
					}(),
					files:        files,
					wantContains: []string{"draft"},
				},
				{
					name: "merged state",
					pr: func() github.PullRequest {
						p := pr
						p.State = github.PRStateMerged
						return p
					}(),
					files:        files,
					wantContains: []string{"merged"},
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					var buf bytes.Buffer
					err := r.render(&buf, tt.pr, tt.files, 60)
					require.NoError(t, err)

					output := buf.String()
					assert.NotEmpty(t, output)
					for _, want := range tt.wantContains {
						assert.Contains(t, output, want)
					}
				})
			}
		})
	}
}

func TestRenderGroupAFileTruncation(t *testing.T) {
	renderers := []struct {
		name   string
		render func(io.Writer, github.PullRequest, []github.PullRequestFile, int) error
	}{
		{"card", renderCard},
		{"dashboard", renderDashboard},
		{"minimal", renderMinimal},
	}

	files := make([]github.PullRequestFile, 35)
	for i := range files {
		files[i] = github.PullRequestFile{
			Additions: 1,
			Deletions: 0,
			Path:      fmt.Sprintf("file%d.go", i),
		}
	}

	pr := testPR()
	pr.FilesChanged = 35

	for _, r := range renderers {
		t.Run(r.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := r.render(&buf, pr, files, 60)
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, "file0.go")
			assert.Contains(t, output, "5 more")
			assert.NotContains(t, output, "file30.go")
		})
	}
}

func TestRenderGroupBStyles(t *testing.T) {
	renderers := []struct {
		name   string
		render func(io.Writer, github.PullRequest, []github.PullRequestFile, int) error
	}{
		{"context", renderContext},
		{"board", renderBoard},
		{"timeline", renderTimeline},
	}

	pr := testPRWithExpanded()
	files := testFiles()

	for _, r := range renderers {
		t.Run(r.name, func(t *testing.T) {
			tests := []struct {
				files        []github.PullRequestFile
				name         string
				pr           github.PullRequest
				wantContains []string
			}{
				{
					name:  "full PR with all data",
					pr:    pr,
					files: files,
					wantContains: []string{
						"#42",
						"Fix login flow",
						"dev",
						"fix/login",
						"main",
						"open",
						"auth.go",
						"+15",
						"-5",
						"bug",
						"priority",
						"ci/test",
						"ci/lint",
						"reviewer1",
						"reviewer2",
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
					files:        files,
					wantContains: []string{"#42", "Fix login flow", "auth.go"},
				},
				{
					name: "empty body",
					pr: func() github.PullRequest {
						p := pr
						p.Body = ""
						return p
					}(),
					files:        files,
					wantContains: []string{"#42", "Fix login flow"},
				},
				{
					name: "no base ref",
					pr: func() github.PullRequest {
						p := pr
						p.BaseRefName = ""
						return p
					}(),
					files:        files,
					wantContains: []string{"fix/login"},
				},
				{
					name: "merged state",
					pr: func() github.PullRequest {
						p := pr
						p.State = github.PRStateMerged
						return p
					}(),
					files:        files,
					wantContains: []string{"merged"},
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					var buf bytes.Buffer
					err := r.render(&buf, tt.pr, tt.files, 60)
					require.NoError(t, err)

					output := buf.String()
					assert.NotEmpty(t, output)
					for _, want := range tt.wantContains {
						assert.Contains(t, output, want)
					}
				})
			}
		})
	}
}

func TestRenderGroupBFileTruncation(t *testing.T) {
	renderers := []struct {
		name   string
		render func(io.Writer, github.PullRequest, []github.PullRequestFile, int) error
	}{
		{"context", renderContext},
		{"board", renderBoard},
		{"timeline", renderTimeline},
	}

	files := make([]github.PullRequestFile, 35)
	for i := range files {
		files[i] = github.PullRequestFile{
			Additions: 1,
			Deletions: 0,
			Path:      fmt.Sprintf("file%d.go", i),
		}
	}

	pr := testPRWithExpanded()
	pr.FilesChanged = 35

	for _, r := range renderers {
		t.Run(r.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := r.render(&buf, pr, files, 60)
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, "file0.go")
			assert.Contains(t, output, "5 more")
			assert.NotContains(t, output, "file30.go")
		})
	}
}

func TestCheckIcon(t *testing.T) {
	tests := []struct {
		conclusion string
		name       string
		wantChar   string
	}{
		{name: "success", conclusion: "success", wantChar: "✓"},
		{name: "failure", conclusion: "failure", wantChar: "✗"},
		{name: "cancelled", conclusion: "cancelled", wantChar: "–"},
		{name: "pending empty", conclusion: "", wantChar: "◯"},
		{name: "timed_out", conclusion: "timed_out", wantChar: "✗"},
		{name: "skipped", conclusion: "skipped", wantChar: "–"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkIcon(tt.conclusion)
			assert.Contains(t, result, tt.wantChar)
		})
	}
}

func TestReviewIcon(t *testing.T) {
	tests := []struct {
		name     string
		state    string
		wantChar string
	}{
		{name: "approved", state: "APPROVED", wantChar: "✓"},
		{name: "changes requested", state: "CHANGES_REQUESTED", wantChar: "✗"},
		{name: "commented", state: "COMMENTED", wantChar: "●"},
		{name: "dismissed", state: "DISMISSED", wantChar: "●"},
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
		want lipgloss.Color
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

func TestTimelineEventSorting(t *testing.T) {
	pr := testPRWithExpanded()
	files := testFiles()

	var buf bytes.Buffer
	err := renderTimeline(&buf, pr, files, 80)
	require.NoError(t, err)

	output := buf.String()
	openedIdx := strings.Index(output, "opened this PR")
	approvedIdx := strings.Index(output, "approved")
	changesIdx := strings.Index(output, "requested changes")

	assert.Greater(t, openedIdx, -1, "should contain opened event")
	assert.Greater(t, approvedIdx, -1, "should contain approved event")
	assert.Greater(t, changesIdx, -1, "should contain changes requested event")
	assert.Less(t, openedIdx, approvedIdx, "opened should appear before approved")
	assert.Less(t, approvedIdx, changesIdx, "approved should appear before changes requested")
}

func TestOutputPRPreviewAllStates(t *testing.T) {
	now := time.Now()

	// Test that all PR states are formatted correctly
	states := []struct {
		state    github.PRState
		wantText string
	}{
		{github.PRStateOpen, "State:  open"},
		{github.PRStateDraft, "State:  draft"},
		{github.PRStateClosed, "State:  closed"},
		{github.PRStateMerged, "State:  merged"},
	}

	for _, st := range states {
		t.Run(string(st.state), func(t *testing.T) {
			pr := github.PullRequest{
				AuthorLogin:  "test",
				Body:         "",
				BranchName:   "test-branch",
				FilesChanged: 0,
				Number:       1,
				State:        st.state,
				Title:        "Test",
				UpdatedAt:    now,
			}

			var buf bytes.Buffer

			err := outputPRPreview(&buf, pr, []github.PullRequestFile{})
			require.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, st.wantText, "output should contain lowercase state")
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
		{Actor: "dev", CreatedAt: time.Now().Add(-90 * time.Minute), Details: "initial commit", Type: "committed"},
		{Actor: "reviewer1", CreatedAt: time.Now().Add(-1 * time.Hour), Details: "approved", Type: "reviewed"},
		{Actor: "merger", CreatedAt: time.Now().Add(-30 * time.Minute), Type: "merged"},
	}
}

func TestRenderReview(t *testing.T) {
	pr := testPRWithExpanded()
	files := testReviewFiles()
	comments := testFileComments()
	timeline := testTimelineEvents()

	tests := []struct {
		comments     map[string]int
		files        []github.PullRequestFile
		name         string
		pr           github.PullRequest
		timeline     []github.TimelineEvent
		wantContains []string
	}{
		{
			name:     "full PR with all data",
			pr:       pr,
			files:    files,
			comments: comments,
			timeline: timeline,
			wantContains: []string{
				"#42",
				"Fix login flow",
				"dev",
				"fix/login",
				"main",
				"open",
				"bug",
				"priority",
				"ci/test",
				"ci/lint",
				"reviewer1",
				"reviewer2",
				"server.go",
				"auth.go",
				"High Activity",
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
			files:    files,
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
			files:        files,
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
			files:        files,
			comments:     comments,
			timeline:     timeline,
			wantContains: []string{"merged"},
		},
		{
			name:         "empty timeline and comments",
			pr:           pr,
			files:        testFiles(),
			comments:     map[string]int{},
			timeline:     nil,
			wantContains: []string{"#42", "auth.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := renderReview(&buf, tt.pr, tt.files, tt.comments, tt.timeline, 60)
			require.NoError(t, err)

			output := buf.String()
			assert.NotEmpty(t, output)
			for _, want := range tt.wantContains {
				assert.Contains(t, output, want)
			}
		})
	}
}

func TestRenderReviewHighActivity(t *testing.T) {
	files := testReviewFiles()
	comments := testFileComments()

	scored := scoreFiles(files, comments)
	output := renderReviewHighActivity(scored, 56)

	assert.Contains(t, output, "High Activity")
	assert.Contains(t, output, "server.go", "highest churn non-test file should appear")
	assert.Contains(t, output, "auth.go", "second highest non-test file should appear")
	assert.NotContains(t, output, "server_test.go", "test files should be excluded")
	assert.NotContains(t, output, "auth_test.go", "test files should be excluded")

	// Test with 0 non-test files
	testOnlyFiles := []github.PullRequestFile{
		{Path: "foo_test.go", Additions: 100, Deletions: 50},
	}
	output = renderReviewHighActivity(scoreFiles(testOnlyFiles, map[string]int{}), 56)
	assert.Empty(t, output)
}

func TestRenderReviewFileComments(t *testing.T) {
	files := []github.PullRequestFile{
		{Path: "with_comments.go", Additions: 10, Deletions: 5},
		{Path: "no_comments.go", Additions: 3, Deletions: 1},
	}
	comments := map[string]int{
		"with_comments.go": 3,
	}

	output := formatReviewFileEntry(files[0], comments["with_comments.go"], 56)
	assert.Contains(t, output, "💬")
	assert.Contains(t, output, "3")

	output = formatReviewFileEntry(files[1], comments["no_comments.go"], 56)
	assert.NotContains(t, output, "💬")
}

func TestRenderReviewBody(t *testing.T) {
	tests := []struct {
		body string
		name string
		want func(string, *testing.T)
	}{
		{
			name: "empty body returns empty",
			body: "",
			want: func(output string, t *testing.T) {
				assert.Empty(t, output)
			},
		},
		{
			name: "markdown is rendered with ANSI",
			body: "## Heading\n\nSome **bold** text.",
			want: func(output string, t *testing.T) {
				assert.NotEmpty(t, output)
				assert.Contains(t, output, "Heading")
				assert.Contains(t, output, "bold")
			},
		},
		{
			name: "plain text passes through",
			body: "Just some plain text.",
			want: func(output string, t *testing.T) {
				assert.Contains(t, output, "plain text")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := renderReviewBody(tt.body, 60)
			tt.want(output, t)
		})
	}
}

func TestRenderReviewTimeline(t *testing.T) {
	pr := testPRWithExpanded()

	timeline := []github.TimelineEvent{
		{Actor: "dev", CreatedAt: time.Now().Add(-90 * time.Minute), Details: "initial commit", Type: "committed"},
		{Actor: "reviewer1", CreatedAt: time.Now().Add(-1 * time.Hour), Details: "approved", Type: "reviewed"},
		{Actor: "merger", CreatedAt: time.Now().Add(-30 * time.Minute), Type: "merged"},
	}

	output := renderReviewTimeline(pr, timeline, 60)

	assert.Contains(t, output, "Activity")
	assert.Contains(t, output, "opened this PR")
	assert.Contains(t, output, "initial commit")
	assert.Contains(t, output, "@reviewer1 approved")
	assert.Contains(t, output, "@merger merged")

	// Verify chronological order: opened(-2h) < committed(-90m) < approved(-1h) < merged(-30m)
	openedIdx := strings.Index(output, "opened this PR")
	commitIdx := strings.Index(output, "initial commit")
	approvedIdx := strings.Index(output, "@reviewer1 approved")
	mergedIdx := strings.Index(output, "@merger merged")
	assert.Less(t, openedIdx, commitIdx)
	assert.Less(t, commitIdx, approvedIdx)
	assert.Less(t, approvedIdx, mergedIdx)

	// Empty timeline with no createdAt
	emptyPR := github.PullRequest{}
	output = renderReviewTimeline(emptyPR, nil, 60)
	assert.Empty(t, output)

	// Consecutive commits by same author are collapsed
	// All 3 commits at -90m..-80m, which is after opened at -2h
	batchTimeline := []github.TimelineEvent{
		{Actor: "dev", CreatedAt: time.Now().Add(-90 * time.Minute), Details: "first", Type: "committed"},
		{Actor: "dev", CreatedAt: time.Now().Add(-85 * time.Minute), Details: "second", Type: "committed"},
		{Actor: "dev", CreatedAt: time.Now().Add(-80 * time.Minute), Details: "third", Type: "committed"},
		{Actor: "reviewer1", CreatedAt: time.Now().Add(-1 * time.Hour), Details: "approved", Type: "reviewed"},
	}
	output = renderReviewTimeline(pr, batchTimeline, 60)
	assert.Contains(t, output, "pushed 3 commits")
	assert.NotContains(t, output, "first")
	assert.NotContains(t, output, "second")
}
