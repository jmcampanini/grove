package github

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testTimeout = 30 * time.Second

func TestNew(t *testing.T) {
	gh := New(context.Background(), "/some/path", 60*time.Second, nil, nil)

	require.NotNil(t, gh)

	// Verify it implements the interface (checked at compile time by var _ GitHub = &CLI{})
	_, ok := gh.(*CLI)
	assert.True(t, ok, "expected *CLI")
}

func TestDefaultPRLimit(t *testing.T) {
	assert.Equal(t, 20, DefaultPRLimit)
}

func TestDefaultIssueLimit(t *testing.T) {
	assert.Equal(t, 20, DefaultIssueLimit)
}

// TestGhOnPath fails, rather than skipping, when gh is missing. The README
// lists gh as a required external program, and the other tests in this
// package run a fake gh, so this is the check that a runner without gh does
// not pass the suite.
func TestGhOnPath(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Fatalf("gh is a required external program: %v", err)
	}
}

func TestCLIGetIssue(t *testing.T) {
	argsPath := installRecordingGh(t, 0, `{
		"author": {"login": "alice"},
		"comments": [{"author": {"login": "bob"}, "body": "Reproduced."}],
		"number": 17,
		"state": "OPEN",
		"title": "Fix login crash"
	}`)
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	issue, err := gh.GetIssue(17)

	require.NoError(t, err)
	assert.Equal(t, 17, issue.Number)
	assert.Equal(t, "Fix login crash", issue.Title)
	assert.Equal(t, []IssueComment{{AuthorLogin: "bob", Body: "Reproduced."}}, issue.Comments)
	args := recordedArgs(t, argsPath)
	assert.Equal(t, []string{"issue", "view", "17"}, args[:3])
	assert.Contains(t, args, "--json")
}

func TestCLIGetIssueFailure(t *testing.T) {
	installRecordingGh(t, 1, "")
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	_, err := gh.GetIssue(17)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "issue #17")
}

func TestCLIGetPullRequest(t *testing.T) {
	argsPath := installRecordingGh(t, 0, `{
		"author": {"login": "alice"},
		"files": [{"additions": 3, "deletions": 1, "path": "main.go"}],
		"headRefName": "feature/login",
		"isDraft": false,
		"number": 42,
		"state": "OPEN",
		"title": "Fix login"
	}`)
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	pr, err := gh.GetPullRequest(42)

	require.NoError(t, err)
	assert.Equal(t, 42, pr.Number)
	assert.Equal(t, "feature/login", pr.BranchName)
	assert.Equal(t, []PullRequestFile{{Additions: 3, Deletions: 1, Path: "main.go"}}, pr.Files)
	args := recordedArgs(t, argsPath)
	assert.Equal(t, []string{"pr", "view", "42"}, args[:3])
	assert.Contains(t, flagValue(args, "--json"), "files")
}

func TestCLIGetPullRequestByBranch(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		wantNumber int
	}{
		{
			name:     "no pull request for the branch",
			response: "[]",
		},
		{
			name:       "pull request found",
			response:   `[{"headRefName": "feature/login", "isDraft": false, "number": 42, "state": "MERGED", "title": "Fix login"}]`,
			wantNumber: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsPath := installRecordingGh(t, 0, tt.response)
			gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

			pr, err := gh.GetPullRequestByBranch("feature/login")

			require.NoError(t, err)
			if tt.wantNumber == 0 {
				assert.Nil(t, pr)
			} else {
				require.NotNil(t, pr)
				assert.Equal(t, tt.wantNumber, pr.Number)
				assert.Equal(t, PRStateMerged, pr.State)
			}
			args := recordedArgs(t, argsPath)
			assert.Equal(t, []string{"pr", "list"}, args[:2])
			assert.Equal(t, "feature/login", flagValue(args, "--head"))
			assert.Equal(t, "all", flagValue(args, "--state"))
			assert.Equal(t, "1", flagValue(args, "--limit"))
		})
	}
}

func TestCLIListIssues(t *testing.T) {
	argsPath := installRecordingGh(t, 0, `[
		{"author": {"login": "alice"}, "number": 17, "state": "OPEN", "title": "Fix login crash"},
		{"author": {"login": "bob"}, "number": 18, "state": "OPEN", "title": "Add logout"}
	]`)
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	issues, err := gh.ListIssues(IssueQuery{State: IssueStateOpen}, DefaultIssueLimit)

	require.NoError(t, err)
	require.Len(t, issues, 2)
	assert.Equal(t, 17, issues[0].Number)
	assert.Equal(t, 18, issues[1].Number)
	args := recordedArgs(t, argsPath)
	assert.Equal(t, []string{"issue", "list"}, args[:2])
	assert.Equal(t, "is:issue is:open", flagValue(args, "--search"))
	assert.Equal(t, "20", flagValue(args, "--limit"))
}

func TestCLIListPullRequests(t *testing.T) {
	argsPath := installRecordingGh(t, 0, `[
		{"headRefName": "feature/login", "isDraft": false, "number": 42, "state": "OPEN", "title": "Fix login"}
	]`)
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	prs, err := gh.ListPullRequests(PRQuery{State: PRStateOpen}, DefaultPRLimit)

	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, 42, prs[0].Number)
	assert.Equal(t, PRStateOpen, prs[0].State)
	args := recordedArgs(t, argsPath)
	assert.Equal(t, []string{"pr", "list"}, args[:2])
	assert.Equal(t, "is:pr is:open draft:false", flagValue(args, "--search"))
	assert.Equal(t, "20", flagValue(args, "--limit"))
}

// installRecordingGh puts a fake gh first on PATH. It records its arguments,
// one per line, in the returned file, prints response on stdout, and exits
// with exitCode.
func installRecordingGh(t *testing.T, exitCode int, response string) string {
	t.Helper()
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	responsePath := filepath.Join(dir, "response")
	require.NoError(t, os.WriteFile(responsePath, []byte(response), 0o600))
	script := `#!/bin/sh
printf '%s\n' "$@" >"$FAKE_GH_ARGS"
cat "$FAKE_GH_RESPONSE"
exit "$FAKE_GH_EXIT"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("FAKE_GH_ARGS", argsPath)
	t.Setenv("FAKE_GH_EXIT", strconv.Itoa(exitCode))
	t.Setenv("FAKE_GH_RESPONSE", responsePath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath
}

// recordedArgs returns the arguments the fake gh received.
func recordedArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// flagValue returns the argument that follows flag, or "" when flag is absent.
func flagValue(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
