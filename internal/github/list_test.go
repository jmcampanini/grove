package github

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectPages(t *testing.T) {
	// The fake connection holds the records 1..total, with odd numbers
	// marked as drafts when drafts is set; a cursor is the number of
	// nodes already read.
	type call struct {
		after string
		first int
	}
	newFetch := func(total int, calls *[]call) fetchPage[int] {
		return func(first int, after string) ([]int, pageInfo, error) {
			*calls = append(*calls, call{after: after, first: first})
			start := 0
			if after != "" {
				_, err := fmt.Sscanf(after, "c%d", &start)
				require.NoError(t, err)
			}
			end := min(start+first, total)
			var nodes []int
			for n := start + 1; n <= end; n++ {
				nodes = append(nodes, n)
			}
			return nodes, pageInfo{EndCursor: fmt.Sprintf("c%d", end), HasNextPage: end < total}, nil
		}
	}
	all := func(int) bool { return true }
	even := func(n int) bool { return n%2 == 0 }

	tests := []struct {
		keep        func(int) bool
		name        string
		req         ListRequest
		total       int
		wantCalls   []call
		wantCursor  string
		wantHasMore bool
		wantRecords []int
	}{
		{
			name:        "empty connection",
			req:         ListRequest{Limit: 20},
			keep:        all,
			wantCalls:   []call{{first: 20}},
			wantRecords: []int{},
		},
		{
			name:        "fewer records than the limit",
			req:         ListRequest{Limit: 20},
			total:       3,
			keep:        all,
			wantCalls:   []call{{first: 20}},
			wantRecords: []int{1, 2, 3},
		},
		{
			name:        "exactly the limit has no more",
			req:         ListRequest{Limit: 3},
			total:       3,
			keep:        all,
			wantCalls:   []call{{first: 3}},
			wantRecords: []int{1, 2, 3},
		},
		{
			name:        "stops at the limit with the cursor after the last record",
			req:         ListRequest{Limit: 2},
			total:       5,
			keep:        all,
			wantCalls:   []call{{first: 2}},
			wantCursor:  "c2",
			wantHasMore: true,
			wantRecords: []int{1, 2},
		},
		{
			name:        "resumes after the cursor",
			req:         ListRequest{After: "c2", Limit: 2},
			total:       5,
			keep:        all,
			wantCalls:   []call{{after: "c2", first: 2}},
			wantCursor:  "c4",
			wantHasMore: true,
			wantRecords: []int{3, 4},
		},
		{
			name:        "a limit above the page size takes several pages",
			req:         ListRequest{Limit: 150},
			total:       250,
			keep:        all,
			wantCalls:   []call{{first: 100}, {after: "c100", first: 50}},
			wantCursor:  "c150",
			wantHasMore: true,
			wantRecords: seq(1, 150),
		},
		{
			name:        "all reads until the connection ends",
			req:         ListRequest{All: true},
			total:       250,
			keep:        all,
			wantCalls:   []call{{first: 100}, {after: "c100", first: 100}, {after: "c200", first: 100}},
			wantRecords: seq(1, 250),
		},
		{
			name:        "dropped nodes do not count toward the limit",
			req:         ListRequest{Limit: 2},
			total:       6,
			keep:        even,
			wantCalls:   []call{{first: 2}, {after: "c2", first: 1}, {after: "c3", first: 1}},
			wantCursor:  "c4",
			wantHasMore: true,
			wantRecords: []int{2, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []call
			records, hasMore, cursor, err := collectPages(tt.req, newFetch(tt.total, &calls), tt.keep)

			require.NoError(t, err)
			assert.Equal(t, tt.wantRecords, records)
			assert.Equal(t, tt.wantHasMore, hasMore)
			assert.Equal(t, tt.wantCursor, cursor)
			assert.Equal(t, tt.wantCalls, calls)
		})
	}
}

func TestCollectPagesFailsOnAFailedPage(t *testing.T) {
	pages := 0
	fetch := func(int, string) ([]int, pageInfo, error) {
		pages++
		if pages == 2 {
			return nil, pageInfo{}, fmt.Errorf("gh api graphql failed")
		}
		return []int{1}, pageInfo{EndCursor: "c1", HasNextPage: true}, nil
	}

	records, hasMore, cursor, err := collectPages(ListRequest{All: true}, fetch, func(int) bool { return true })

	require.ErrorContains(t, err, "gh api graphql failed")
	assert.Nil(t, records, "a failed page returns no records")
	assert.False(t, hasMore)
	assert.Empty(t, cursor)
}

func TestCollectPagesRejectsAPageWithoutACursor(t *testing.T) {
	fetch := func(int, string) ([]int, pageInfo, error) {
		return nil, pageInfo{HasNextPage: true}, nil
	}

	_, _, _, err := collectPages(ListRequest{All: true}, fetch, func(int) bool { return true })

	require.ErrorContains(t, err, "without returning a cursor")
}

func TestCLIListOpenIssues(t *testing.T) {
	pagesDir, argsPath := installPagingGh(t)
	writePage(t, pagesDir, "start", `{"data": {"repository": {"issues": {
		"pageInfo": {"endCursor": "Y2:1", "hasNextPage": true},
		"nodes": [{"author": {"login": "alice"}, "labels": {"nodes": [{"color": "d73a4a", "name": "bug"}]},
			"number": 18, "state": "OPEN", "title": "Add logout", "url": "https://github.com/acme/app/issues/18"}]
	}}}}`)
	writePage(t, pagesDir, "Y2:1", `{"data": {"repository": {"issues": {
		"pageInfo": {"endCursor": "Y2:2", "hasNextPage": false},
		"nodes": [{"author": null, "labels": {"nodes": []}, "number": 17, "state": "OPEN", "title": "Fix login crash"}]
	}}}}`)
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	list, err := gh.ListOpenIssues(ListRequest{All: true})

	require.NoError(t, err)
	assert.False(t, list.HasMore)
	assert.Empty(t, list.NextCursor)
	require.Len(t, list.Issues, 2)
	assert.Equal(t, Issue{
		AuthorLogin: "alice",
		Labels:      []Label{{Color: "d73a4a", Name: "bug"}},
		Number:      18,
		State:       IssueStateOpen,
		Title:       "Add logout",
		URL:         "https://github.com/acme/app/issues/18",
	}, list.Issues[0])
	assert.Equal(t, 17, list.Issues[1].Number)
	assert.Empty(t, list.Issues[1].AuthorLogin, "a deleted author has no login")
	assert.Equal(t, []Label{}, list.Issues[1].Labels)

	calls := recordedCalls(t, argsPath)
	require.Len(t, calls, 2)
	assert.Equal(t, []string{"api", "graphql"}, calls[0][:2])
	assert.Equal(t, "github.example.com", flagValue(calls[0], "--hostname"), "the query goes to the host of the repository")
	assert.Contains(t, calls[0], "owner=acme")
	assert.Contains(t, calls[0], "name=app")
	assert.Contains(t, calls[0], "first=100")
	assert.NotContains(t, strings.Join(calls[0], "\n"), "after=")
	assert.Contains(t, calls[1], "after=Y2:1")
	assert.Contains(t, strings.Join(calls[0], "\n"), "states: [OPEN]")
}

func TestCLIListOpenIssuesStopsAtTheLimit(t *testing.T) {
	pagesDir, argsPath := installPagingGh(t)
	writePage(t, pagesDir, "start", `{"data": {"repository": {"issues": {
		"pageInfo": {"endCursor": "Y2:1", "hasNextPage": true},
		"nodes": [{"number": 18, "state": "OPEN", "title": "Add logout"}]
	}}}}`)
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	list, err := gh.ListOpenIssues(ListRequest{Limit: 1})

	require.NoError(t, err)
	assert.True(t, list.HasMore)
	assert.Equal(t, "Y2:1", list.NextCursor)
	require.Len(t, list.Issues, 1)
	calls := recordedCalls(t, argsPath)
	require.Len(t, calls, 1)
	assert.Contains(t, calls[0], "first=1")
}

func TestCLIListOpenPullRequestsDropsDrafts(t *testing.T) {
	pagesDir, _ := installPagingGh(t)
	writePage(t, pagesDir, "start", `{"data": {"repository": {"pullRequests": {
		"pageInfo": {"endCursor": "UFI:2", "hasNextPage": false},
		"nodes": [
			{"author": {"login": "alice"}, "baseRefName": "main", "headRefName": "feature/login", "isCrossRepository": true,
				"isDraft": false, "labels": {"nodes": []}, "number": 42, "state": "OPEN", "title": "Fix login",
				"url": "https://github.com/acme/app/pull/42"},
			{"headRefName": "wip", "isDraft": true, "number": 43, "state": "OPEN", "title": "WIP"}
		]
	}}}}`)
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	list, err := gh.ListOpenPullRequests(ListRequest{Limit: 20})

	require.NoError(t, err)
	assert.False(t, list.HasMore)
	assert.Equal(t, []PullRequest{{
		AuthorLogin:       "alice",
		BaseRefName:       "main",
		BranchName:        "feature/login",
		IsCrossRepository: true,
		Labels:            []Label{},
		Number:            42,
		State:             PRStateOpen,
		Title:             "Fix login",
		URL:               "https://github.com/acme/app/pull/42",
	}}, list.PullRequests)
}

func TestCLIListFailures(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantErr  string
	}{
		{name: "not JSON", response: "oops", wantErr: "failed to list issues: failed to parse GraphQL response"},
		{name: "GraphQL errors", response: `{"errors": [{"message": "Bad cursor"}]}`, wantErr: "failed to list issues: GraphQL query failed: Bad cursor"},
		{name: "no repository", response: `{"data": {"repository": null}}`, wantErr: "GraphQL response has no repository"},
		{name: "unknown state", response: `{"data": {"repository": {"issues": {"nodes": [{"number": 1, "state": "LOCKED"}]}}}}`, wantErr: "unknown issue state: LOCKED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pagesDir, _ := installPagingGh(t)
			writePage(t, pagesDir, "start", tt.response)
			gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

			_, err := gh.ListOpenIssues(ListRequest{Limit: 20})

			require.ErrorContains(t, err, tt.wantErr)
			assert.Equal(t, 1, strings.Count(err.Error(), "failed to list issues"), "the prefix appears once: %v", err)
		})
	}
}

func TestCLIListGhFailure(t *testing.T) {
	// No page file exists, so the fake gh fails the query.
	installPagingGh(t)
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	_, err := gh.ListOpenPullRequests(ListRequest{Limit: 20})

	require.ErrorContains(t, err, "failed to list pull requests: gh api graphql")
	assert.Equal(t, 1, strings.Count(err.Error(), "failed to list pull requests"), "the prefix appears once: %v", err)
	assert.Contains(t, err.Error(), "query=...")
	assert.NotContains(t, err.Error(), "pullRequests(", "the query text stays out of the failure line")
}

func TestCLIListRepositoryFailure(t *testing.T) {
	installRecordingGh(t, 1, "")
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	_, err := gh.ListOpenIssues(ListRequest{Limit: 20})

	require.ErrorContains(t, err, "failed to list issues: gh repo view")
}

func TestCLIListOpenIssuesRejectsDisabledIssues(t *testing.T) {
	pagesDir, argsPath := installPagingGh(t)
	writePage(t, pagesDir, "repo", `{"hasIssuesEnabled": false, "name": "app", "owner": {"login": "acme"}, "url": "https://github.com/acme/app"}`)
	gh := New(context.Background(), t.TempDir(), testTimeout, nil, nil)

	_, err := gh.ListOpenIssues(ListRequest{Limit: 20})

	require.EqualError(t, err, "failed to list issues: the acme/app repository has disabled issues")
	_, statErr := os.Stat(argsPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "no query runs")
}

// installPagingGh puts a fake gh first on PATH that prints the page file
// named by its after= argument, or the file named start when there is none.
// Each invocation appends its arguments, one per line, to the returned args
// file, followed by a line holding only "--".
func installPagingGh(t *testing.T) (pagesDir, argsPath string) {
	t.Helper()
	dir := t.TempDir()
	pagesDir = filepath.Join(dir, "pages")
	require.NoError(t, os.Mkdir(pagesDir, 0o755))
	argsPath = filepath.Join(dir, "args")
	script := `#!/bin/sh
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
  printf '%s\n' "$argument" >>"$FAKE_GH_ARGS"
  case "$argument" in
    after=*) page=${argument#after=} ;;
  esac
done
printf -- '--\n' >>"$FAKE_GH_ARGS"
cat "$FAKE_GH_PAGES/$page"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755))
	t.Setenv("FAKE_GH_ARGS", argsPath)
	t.Setenv("FAKE_GH_PAGES", pagesDir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return pagesDir, argsPath
}

func writePage(t *testing.T, pagesDir, name, response string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(pagesDir, name), []byte(response), 0o600))
}

// recordedCalls returns the arguments of each fake gh invocation.
func recordedCalls(t *testing.T, path string) [][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var calls [][]string
	var current []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "--" {
			calls = append(calls, current)
			current = nil
			continue
		}
		current = append(current, line)
	}
	return calls
}

func seq(from, to int) []int {
	out := make([]int, 0, to-from+1)
	for n := from; n <= to; n++ {
		out = append(out, n)
	}
	return out
}
