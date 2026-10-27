package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fake GitHub repository behind these tests has two pages of open
// issues: #18 and #17 on the first, #16 on the second.
const (
	issuesPageStart = `{"data": {"repository": {"issues": {
  "pageInfo": {"endCursor": "after-17", "hasNextPage": true},
  "nodes": [
    {"author": {"login": "bob"}, "createdAt": "2026-07-03T09:00:00Z", "labels": {"nodes": [{"name": "bug"}]},
     "number": 18, "state": "OPEN", "title": "Add logout", "updatedAt": "2026-07-04T09:00:00Z",
     "url": "https://github.com/acme/app/issues/18"},
    {"author": {"login": "alice"}, "createdAt": "2026-07-02T09:00:00Z", "labels": {"nodes": []},
     "number": 17, "state": "OPEN", "title": "Fix login", "updatedAt": "2026-07-02T10:00:00Z",
     "url": "https://github.com/acme/app/issues/17"}
  ]
}}}}`
	issuesPageAfter17 = `{"data": {"repository": {"issues": {
  "pageInfo": {"endCursor": "after-16", "hasNextPage": false},
  "nodes": [
    {"author": {"login": "carol"}, "createdAt": "2026-07-01T09:00:00Z", "labels": {"nodes": []},
     "number": 16, "state": "OPEN", "title": "Old bug", "updatedAt": "2026-07-01T09:00:00Z",
     "url": "https://github.com/acme/app/issues/16"}
  ]
}}}}`
)

type listCommandFixture struct {
	pagesDir    string
	worktreeDir string
}

// newListCommandFixture builds a repository with a worktree for issue #17,
// isolates every grove directory, and puts a fake gh first on PATH that
// prints the page file its after= argument names, or "start". A page file
// that is missing makes gh fail.
func newListCommandFixture(t *testing.T) listCommandFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake gh process uses a POSIX shell")
	}

	testDir := t.TempDir()
	mainDir := filepath.Join(testDir, "main")
	worktreeDir := filepath.Join(testDir, "wt-17")
	binDir := filepath.Join(testDir, "bin")
	pagesDir := filepath.Join(testDir, "pages")
	for _, dir := range []string{mainDir, binDir, pagesDir, filepath.Join(testDir, "home")} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}

	t.Setenv("HOME", filepath.Join(testDir, "home"))
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TERM", "dumb")
	t.Setenv("XDG_CACHE_HOME", filepath.Join(testDir, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(testDir, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(testDir, "state"))
	t.Setenv("FAKE_GH_PAGES", pagesDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	initGitRepo(t, mainDir)
	worktree := exec.Command("git", "worktree", "add", "-qb", "issue/17-fix-login", worktreeDir)
	worktree.Dir = mainDir
	out, err := worktree.CombinedOutput()
	require.NoError(t, err, string(out))

	require.NoError(t, os.WriteFile(filepath.Join(binDir, "gh"), []byte(`#!/bin/sh
if [ "$1" = repo ]; then
  if [ -f "$FAKE_GH_PAGES/repo" ]; then
    cat "$FAKE_GH_PAGES/repo"
  else
    printf '%s\n' '{"hasIssuesEnabled": true, "name": "app", "owner": {"login": "acme"}, "url": "https://github.example.com/acme/app"}'
  fi
  exit 0
fi
page=start
for argument in "$@"; do
  case "$argument" in
    after=*) page=${argument#after=} ;;
  esac
done
if [ ! -f "$FAKE_GH_PAGES/$page" ]; then
  echo "HTTP 502: page $page unavailable" >&2
  exit 1
fi
cat "$FAKE_GH_PAGES/$page"
`), 0o755))

	t.Chdir(mainDir)
	return listCommandFixture{pagesDir: pagesDir, worktreeDir: worktreeDir}
}

func (f listCommandFixture) writePage(t *testing.T, name, response string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.pagesDir, name), []byte(response), 0o600))
}

func runListCommand(args ...string) (stdout, stderr string, err error) {
	var out, errOut bytes.Buffer
	err = executeWithFileLogging(strings.NewReader(""), &out, &errOut, args)
	return out.String(), errOut.String(), err
}

func TestIssueListJSONStopsAtTheLimit(t *testing.T) {
	fixture := newListCommandFixture(t)
	fixture.writePage(t, "start", issuesPageStart)

	stdout, stderr, err := runListCommand("issue", "list", "--json", "--limit", "2")

	require.NoError(t, err, stderr)
	var doc struct {
		Complete   bool              `json:"complete"`
		HasMore    bool              `json:"hasMore"`
		Items      []json.RawMessage `json:"items"`
		NextCursor string            `json:"nextCursor"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc), stdout)
	assert.True(t, doc.Complete)
	assert.True(t, doc.HasMore)
	assert.Equal(t, "after-17", doc.NextCursor)
	require.Len(t, doc.Items, 2)
	assert.NotContains(t, stderr, "more issues exist", "--json carries hasMore in the payload")

	var first, second issueRecord
	require.NoError(t, json.Unmarshal(doc.Items[0], &first))
	require.NoError(t, json.Unmarshal(doc.Items[1], &second))
	assert.Equal(t, 18, first.Number)
	assert.Equal(t, []string{"bug"}, first.Labels)
	assert.Nil(t, first.Worktree)
	assert.Equal(t, 17, second.Number)
	require.NotNil(t, second.Worktree, "issue #17 has a worktree")
	assert.Equal(t, "wt-17", filepath.Base(*second.Worktree))
}

func TestIssueListCursorResumes(t *testing.T) {
	fixture := newListCommandFixture(t)
	fixture.writePage(t, "after-17", issuesPageAfter17)

	stdout, stderr, err := runListCommand("issue", "list", "--jsonl", "--cursor", "after-17")

	require.NoError(t, err, stderr)
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], `"number":16`)
	assert.NotContains(t, stderr, "more issues exist")
}

func TestIssueListCSVReportsMoreOnStderr(t *testing.T) {
	fixture := newListCommandFixture(t)
	fixture.writePage(t, "start", issuesPageStart)

	stdout, stderr, err := runListCommand("issue", "list", "--csv", "--no-header", "--limit", "2")

	require.NoError(t, err, stderr)
	assert.Equal(t, exitSuccess, exitStatus(err))
	assert.Len(t, strings.Split(strings.TrimSuffix(stdout, "\n"), "\n"), 2, "only the two records reach stdout")
	assert.NotContains(t, stdout, "more issues")
	assert.Contains(t, stderr, "more issues exist; continue with --cursor after-17\n")
}

func TestIssueListAllFetchesEveryPage(t *testing.T) {
	fixture := newListCommandFixture(t)
	fixture.writePage(t, "start", issuesPageStart)
	fixture.writePage(t, "after-17", issuesPageAfter17)

	stdout, stderr, err := runListCommand("issue", "list", "--csv", "--all")

	require.NoError(t, err, stderr)
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	require.Len(t, lines, 4)
	assert.Equal(t, strings.Join(issueRecordColumns, ","), lines[0])
	assert.True(t, strings.HasPrefix(lines[3], "16,Old bug,carol,[],"), lines[3])
	assert.NotContains(t, stderr, "more issues exist")
}

func TestIssueListFailedPageWritesNoRecords(t *testing.T) {
	for _, format := range []string{"--json", "--jsonl", "--csv"} {
		t.Run(format, func(t *testing.T) {
			fixture := newListCommandFixture(t)
			// The second page is missing, so its gh command fails.
			fixture.writePage(t, "start", issuesPageStart)
			output := filepath.Join(t.TempDir(), "issues.out")
			require.NoError(t, os.WriteFile(output, []byte("previous\n"), 0o600))

			stdout, _, err := runListCommand("issue", "list", format, "--all")
			require.ErrorContains(t, err, "page after-17 unavailable")
			assert.Equal(t, exitFailure, exitStatus(err))
			assert.Empty(t, stdout)
			assert.Equal(t, 1, strings.Count(err.Error(), "failed to list issues"), err.Error())

			_, _, err = runListCommand("issue", "list", format, "--all", "--output-file", output, "--overwrite")
			require.Error(t, err)
			assertFileContent(t, output, "previous\n")
		})
	}
}

func TestIssueListOutputFile(t *testing.T) {
	fixture := newListCommandFixture(t)
	fixture.writePage(t, "start", issuesPageStart)
	output := filepath.Join(t.TempDir(), "issues.jsonl")

	stdout, stderr, err := runListCommand("issue", "list", "--jsonl", "--limit", "2", "--output-file", output)
	require.NoError(t, err, stderr)
	assert.Empty(t, stdout, "the payload goes to the file")
	assert.Contains(t, stderr, "more issues exist; continue with --cursor after-17")
	data, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(string(data), "\n"))

	_, _, err = runListCommand("issue", "list", "--jsonl", "--output-file", output)
	require.ErrorContains(t, err, "already exists; pass --overwrite to replace it")
	assert.Equal(t, exitFailure, exitStatus(err))
	assertFileContent(t, output, string(data))

	_, stderr, err = runListCommand("issue", "list", "--json", "--limit", "2", "--output-file", output, "--overwrite")
	require.NoError(t, err, stderr)
	replaced, err := os.ReadFile(output)
	require.NoError(t, err)
	assert.True(t, json.Valid(replaced))
	assert.Contains(t, string(replaced), `"hasMore":true`)
}

func TestIssueListTableReportsMoreOnStderr(t *testing.T) {
	fixture := newListCommandFixture(t)
	fixture.writePage(t, "start", issuesPageStart)

	stdout, stderr, err := runListCommand("issue", "list", "--limit", "2")

	require.NoError(t, err, stderr)
	assert.Contains(t, stdout, "Fix login")
	assert.NotContains(t, stdout, "more issues")
	assert.Contains(t, stderr, "more issues exist; continue with --cursor after-17")
}

func TestPRListJSONDropsDrafts(t *testing.T) {
	fixture := newListCommandFixture(t)
	fixture.writePage(t, "start", `{"data": {"repository": {"pullRequests": {
  "pageInfo": {"endCursor": "after-41", "hasNextPage": false},
  "nodes": [
    {"author": {"login": "alice"}, "baseRefName": "main", "createdAt": "2026-07-03T09:00:00Z",
     "headRefName": "feature/login", "isCrossRepository": false, "isDraft": false,
     "labels": {"nodes": []}, "number": 42, "state": "OPEN", "title": "Fix login",
     "updatedAt": "2026-07-04T09:00:00Z", "url": "https://github.com/acme/app/pull/42"},
    {"headRefName": "wip", "isDraft": true, "number": 41, "state": "OPEN", "title": "WIP"}
  ]
}}}}`)

	stdout, stderr, err := runListCommand("pr", "list", "--json")

	require.NoError(t, err, stderr)
	assert.Equal(t, `{"items":[{"number":42,"title":"Fix login","author":"alice","branch":"feature/login",`+
		`"baseBranch":"main","isCrossRepository":false,"labels":[],"createdAt":"2026-07-03T09:00:00Z",`+
		`"updatedAt":"2026-07-04T09:00:00Z","url":"https://github.com/acme/app/pull/42","worktree":null}],`+
		`"hasMore":false,"complete":true}`+"\n", stdout)
}
