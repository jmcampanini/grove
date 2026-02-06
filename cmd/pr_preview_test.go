package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

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
