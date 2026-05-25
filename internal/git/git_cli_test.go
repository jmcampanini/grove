package git

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// splitIntoBlocks tests
// =============================================================================

func TestSplitIntoBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  [][]string
	}{
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "single line no newline",
			input: "line1",
			want:  [][]string{{"line1"}},
		},
		{
			name:  "single line with newline",
			input: "line1\n",
			want:  [][]string{{"line1"}},
		},
		{
			name:  "multiple lines single block",
			input: "line1\nline2\nline3",
			want:  [][]string{{"line1", "line2", "line3"}},
		},
		{
			name:  "two blocks separated by blank line",
			input: "block1line1\nblock1line2\n\nblock2line1",
			want:  [][]string{{"block1line1", "block1line2"}, {"block2line1"}},
		},
		{
			name:  "three blocks",
			input: "a1\na2\n\nb1\n\nc1\nc2\nc3",
			want:  [][]string{{"a1", "a2"}, {"b1"}, {"c1", "c2", "c3"}},
		},
		{
			name:  "trailing blank line",
			input: "line1\nline2\n\n",
			want:  [][]string{{"line1", "line2"}},
		},
		{
			name:  "leading blank line",
			input: "\nline1\nline2",
			want:  [][]string{{"line1", "line2"}},
		},
		{
			name:  "multiple consecutive blank lines",
			input: "block1\n\n\n\nblock2",
			want:  [][]string{{"block1"}, {"block2"}},
		},
		{
			name:  "only blank lines",
			input: "\n\n\n",
			want:  nil,
		},
		{
			name:  "single blank line",
			input: "\n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitIntoBlocks(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// parseTrackInfo tests
// =============================================================================

func TestParseTrackInfo(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAhead  int
		wantBehind int
	}{
		{
			name:       "empty string",
			input:      "",
			wantAhead:  0,
			wantBehind: 0,
		},
		{
			name:       "gone upstream",
			input:      "[gone]",
			wantAhead:  0,
			wantBehind: 0,
		},
		{
			name:       "ahead only",
			input:      "[ahead 5]",
			wantAhead:  5,
			wantBehind: 0,
		},
		{
			name:       "behind only",
			input:      "[behind 3]",
			wantAhead:  0,
			wantBehind: 3,
		},
		{
			name:       "ahead and behind",
			input:      "[ahead 2, behind 4]",
			wantAhead:  2,
			wantBehind: 4,
		},
		{
			name:       "large numbers",
			input:      "[ahead 100, behind 200]",
			wantAhead:  100,
			wantBehind: 200,
		},
		{
			name:       "single digit",
			input:      "[ahead 1]",
			wantAhead:  1,
			wantBehind: 0,
		},
		{
			name:       "invalid format",
			input:      "[invalid]",
			wantAhead:  0,
			wantBehind: 0,
		},
		{
			name:       "malformed brackets",
			input:      "ahead 5",
			wantAhead:  0,
			wantBehind: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ahead, behind := parseTrackInfo(tt.input)
			assert.Equal(t, tt.wantAhead, ahead, "ahead mismatch")
			assert.Equal(t, tt.wantBehind, behind, "behind mismatch")
		})
	}
}

// =============================================================================
// parseISO8601Date tests
// =============================================================================

func TestParseISO8601Date(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "valid RFC3339",
			input: "2024-01-15T10:30:00Z",
			want:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			name:  "valid with timezone offset",
			input: "2024-06-20T15:45:30-07:00",
			want:  time.Date(2024, 6, 20, 15, 45, 30, 0, time.FixedZone("", -7*3600)),
		},
		{
			name:  "empty string",
			input: "",
			want:  time.Time{},
		},
		{
			name:  "invalid format",
			input: "not-a-date",
			want:  time.Time{},
		},
		{
			name:  "partial date",
			input: "2024-01-15",
			want:  time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseISO8601Date(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// parseBranchBlock tests
// =============================================================================

func TestParseBranchBlock(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  LocalBranch
	}{
		{
			name: "complete branch with all fields",
			input: []string{
				"branch main",
				"checkedOut true",
				"commit abc1234",
				"upstream origin/main",
				"track [ahead 2, behind 1]",
				"committedOn 2024-01-15T10:30:00Z",
				"committedBy John Doe",
				"subject Initial commit",
				"worktreepath /home/user/project",
			},
			want: NewLocalBranch(
				"main",
				"origin/main",
				"/home/user/project",
				true,
				2,
				1,
				NewCommit("abc1234", "Initial commit", time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC), "John Doe"),
			),
		},
		{
			name: "branch without upstream",
			input: []string{
				"branch feature-x",
				"checkedOut false",
				"commit def5678",
				"committedOn 2024-02-20T14:00:00Z",
				"committedBy Jane Smith",
				"subject Add new feature",
				"worktreepath ",
			},
			want: NewLocalBranch(
				"feature-x",
				"",
				"",
				false,
				0,
				0,
				NewCommit("def5678", "Add new feature", time.Date(2024, 2, 20, 14, 0, 0, 0, time.UTC), "Jane Smith"),
			),
		},
		{
			name: "branch with gone upstream",
			input: []string{
				"branch stale-branch",
				"checkedOut false",
				"commit ghi9012",
				"upstream origin/deleted-branch",
				"track [gone]",
				"committedOn 2024-03-01T09:00:00Z",
				"committedBy Developer",
				"subject Some work",
				"worktreepath ",
			},
			want: NewLocalBranch(
				"stale-branch",
				"origin/deleted-branch",
				"",
				false,
				0,
				0,
				NewCommit("ghi9012", "Some work", time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC), "Developer"),
			),
		},
		{
			name: "branch with only ahead",
			input: []string{
				"branch dev",
				"checkedOut true",
				"commit jkl3456",
				"upstream origin/dev",
				"track [ahead 5]",
				"committedOn 2024-04-10T11:30:00Z",
				"committedBy Alice",
				"subject WIP",
				"worktreepath /workspace/dev",
			},
			want: NewLocalBranch(
				"dev",
				"origin/dev",
				"/workspace/dev",
				true,
				5,
				0,
				NewCommit("jkl3456", "WIP", time.Date(2024, 4, 10, 11, 30, 0, 0, time.UTC), "Alice"),
			),
		},
		{
			name: "branch with invalid date",
			input: []string{
				"branch broken",
				"checkedOut false",
				"commit xyz000",
				"committedOn invalid-date",
				"committedBy Unknown",
				"subject Test",
				"worktreepath ",
			},
			want: NewLocalBranch(
				"broken",
				"",
				"",
				false,
				0,
				0,
				NewCommit("xyz000", "Test", time.Time{}, "Unknown"),
			),
		},
		{
			name:  "empty input",
			input: []string{},
			want:  NewLocalBranch("", "", "", false, 0, 0, NewCommit("", "", time.Time{}, "")),
		},
		{
			name: "branch with slash in name",
			input: []string{
				"branch feature/my-feature",
				"checkedOut false",
				"commit abc123",
				"committedOn 2024-01-01T00:00:00Z",
				"committedBy Dev",
				"subject Feature work",
				"worktreepath ",
			},
			want: NewLocalBranch(
				"feature/my-feature",
				"",
				"",
				false,
				0,
				0,
				NewCommit("abc123", "Feature work", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "Dev"),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBranchBlock(tt.input)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.UpstreamName, got.UpstreamName)
			assert.Equal(t, tt.want.WorktreeAbsolutePath, got.WorktreeAbsolutePath)
			assert.Equal(t, tt.want.IsCheckedOut, got.IsCheckedOut)
			assert.Equal(t, tt.want.Ahead, got.Ahead)
			assert.Equal(t, tt.want.Behind, got.Behind)
			assert.Equal(t, tt.want.Commit().SHA, got.Commit().SHA)
			assert.Equal(t, tt.want.Commit().Subject, got.Commit().Subject)
			assert.Equal(t, tt.want.Commit().CommittedBy, got.Commit().CommittedBy)
			assert.True(t, tt.want.Commit().CommittedOn.Equal(got.Commit().CommittedOn), "commit time mismatch")
		})
	}
}

// =============================================================================
// parseBranchesFromFormat tests
// =============================================================================

func TestParseBranchesFromFormat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNames  []string
		wantLength int
	}{
		{
			name:       "empty output",
			input:      "",
			wantNames:  []string{},
			wantLength: 0,
		},
		{
			name: "single branch",
			input: `branch main
checkedOut true
commit abc1234
committedOn 2024-01-15T10:30:00Z
committedBy John
subject Initial
worktreepath /home/user
`,
			wantNames:  []string{"main"},
			wantLength: 1,
		},
		{
			name: "multiple branches",
			input: `branch main
checkedOut true
commit abc1234
committedOn 2024-01-15T10:30:00Z
committedBy John
subject Initial
worktreepath /home/user

branch feature
checkedOut false
commit def5678
committedOn 2024-01-16T11:00:00Z
committedBy Jane
subject Feature
worktreepath

branch develop
checkedOut false
commit ghi9012
committedOn 2024-01-17T12:00:00Z
committedBy Bob
subject Develop
worktreepath
`,
			wantNames:  []string{"main", "feature", "develop"},
			wantLength: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBranchesFromFormat(tt.input)
			assert.Len(t, got, tt.wantLength)
			assert.Equal(t, tt.wantNames, branchNames(got))
		})
	}
}

// =============================================================================
// parseRemoteBranchBlock tests
// =============================================================================

func TestParseRemoteBranchBlock(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  RemoteBranch
	}{
		{
			name: "standard remote branch",
			input: []string{
				"ref origin/main",
				"commit abc1234",
				"committedOn 2024-01-15T10:30:00Z",
				"committedBy John Doe",
				"subject Initial commit",
			},
			want: NewRemoteBranch(
				"main",
				"origin",
				NewCommit("abc1234", "Initial commit", time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC), "John Doe"),
			),
		},
		{
			name: "remote branch with slash in name",
			input: []string{
				"ref origin/feature/awesome",
				"commit def5678",
				"committedOn 2024-02-20T14:00:00Z",
				"committedBy Jane Smith",
				"subject Add feature",
			},
			want: NewRemoteBranch(
				"feature/awesome",
				"origin",
				NewCommit("def5678", "Add feature", time.Date(2024, 2, 20, 14, 0, 0, 0, time.UTC), "Jane Smith"),
			),
		},
		{
			name: "upstream remote",
			input: []string{
				"ref upstream/develop",
				"commit ghi9012",
				"committedOn 2024-03-01T09:00:00Z",
				"committedBy Developer",
				"subject Work in progress",
			},
			want: NewRemoteBranch(
				"develop",
				"upstream",
				NewCommit("ghi9012", "Work in progress", time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC), "Developer"),
			),
		},
		{
			name: "ref without slash",
			input: []string{
				"ref invalid",
				"commit xyz000",
				"committedOn 2024-01-01T00:00:00Z",
				"committedBy Unknown",
				"subject Test",
			},
			want: NewRemoteBranch(
				"",
				"",
				NewCommit("xyz000", "Test", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "Unknown"),
			),
		},
		{
			name:  "empty input",
			input: []string{},
			want:  NewRemoteBranch("", "", NewCommit("", "", time.Time{}, "")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRemoteBranchBlock(tt.input)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.RemoteName, got.RemoteName)
			assert.Equal(t, tt.want.Commit().SHA, got.Commit().SHA)
			assert.Equal(t, tt.want.Commit().Subject, got.Commit().Subject)
			assert.Equal(t, tt.want.Commit().CommittedBy, got.Commit().CommittedBy)
		})
	}
}

// =============================================================================
// parseRemoteDefaultBranchFromLSRemote tests
// =============================================================================

func TestParseRemoteDefaultBranchFromLSRemote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name: "main branch",
			input: "ref: refs/heads/main\tHEAD\n" +
				"abc123\tHEAD",
			want: "main",
		},
		{
			name: "branch with slash",
			input: "ref: refs/heads/release/next\tHEAD\n" +
				"abc123\tHEAD",
			want: "release/next",
		},
		{
			name:  "missing symref",
			input: "abc123\tHEAD",
			want:  "",
		},
		{
			name:  "HEAD points outside heads",
			input: "ref: refs/tags/v1.0.0\tHEAD\nabc123\tHEAD",
			want:  "",
		},
		{
			name:  "empty output",
			input: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRemoteDefaultBranchFromLSRemote(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// parseRemoteBranchesFromFormat tests
// =============================================================================

func TestParseRemoteBranchesFromFormat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNames  []string
		wantLength int
	}{
		{
			name:       "empty output",
			input:      "",
			wantNames:  []string{},
			wantLength: 0,
		},
		{
			name: "single remote branch",
			input: `ref origin/main
commit abc1234
committedOn 2024-01-15T10:30:00Z
committedBy John
subject Initial
`,
			wantNames:  []string{"main"},
			wantLength: 1,
		},
		{
			name: "multiple remote branches",
			input: `ref origin/main
commit abc1234
committedOn 2024-01-15T10:30:00Z
committedBy John
subject Initial

ref origin/develop
commit def5678
committedOn 2024-01-16T11:00:00Z
committedBy Jane
subject Develop

ref origin/feature/test
commit ghi9012
committedOn 2024-01-17T12:00:00Z
committedBy Bob
subject Feature
`,
			wantNames:  []string{"main", "develop", "feature/test"},
			wantLength: 3,
		},
		{
			name: "filters out symbolic HEAD ref",
			input: `ref origin/HEAD
commit abc1234
committedOn 2024-01-15T10:30:00Z
committedBy John
subject Initial

ref origin/main
commit abc1234
committedOn 2024-01-15T10:30:00Z
committedBy John
subject Initial
`,
			wantNames:  []string{"main"},
			wantLength: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRemoteBranchesFromFormat(tt.input)
			assert.Len(t, got, tt.wantLength)
			assert.Equal(t, tt.wantNames, remoteBranchNames(got))
		})
	}
}

// =============================================================================
// parseTagBlock tests
// =============================================================================

func TestParseTagBlock(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  Tag
	}{
		{
			name: "annotated tag with all fields",
			input: []string{
				"name v1.0.0",
				"objecttype tag",
				"objectsha tag123",
				"derefsha abc1234",
				"taggername John Doe",
				"taggeremail john@example.com",
				"taggedon 2024-01-15T10:30:00Z",
				"message Release version 1.0.0",
				"committedby Jane Smith",
				"committedon 2024-01-14T09:00:00Z",
				"committerdate ",
				"commitsubject Initial commit",
			},
			want: NewTag(
				"v1.0.0",
				NewCommit("abc1234", "Initial commit", time.Date(2024, 1, 14, 9, 0, 0, 0, time.UTC), "Jane Smith"),
				"Release version 1.0.0",
				"John Doe",
				"john@example.com",
				time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			),
		},
		{
			name: "lightweight tag",
			input: []string{
				"name v0.1.0",
				"objecttype commit",
				"objectsha def5678",
				"derefsha ",
				"taggername ",
				"taggeremail ",
				"taggedon ",
				"message ",
				"committedby ",
				"committedon ",
				"committerdate 2024-01-14T09:00:00Z",
				"commitsubject ",
			},
			want: NewTag(
				"v0.1.0",
				NewCommit("def5678", "", time.Date(2024, 1, 14, 9, 0, 0, 0, time.UTC), ""),
				"",
				"",
				"",
				time.Time{},
			),
		},
		{
			name: "annotated tag without commit subject fallback",
			input: []string{
				"name v2.0.0",
				"objecttype tag",
				"objectsha tag456",
				"derefsha ghi9012",
				"taggername Alice",
				"taggeremail alice@example.com",
				"taggedon 2024-06-01T12:00:00Z",
				"message Major release",
				"committedby Bob",
				"committedon 2024-05-30T08:00:00Z",
				"committerdate ",
				"commitsubject Big feature",
			},
			want: NewTag(
				"v2.0.0",
				NewCommit("ghi9012", "Big feature", time.Date(2024, 5, 30, 8, 0, 0, 0, time.UTC), "Bob"),
				"Major release",
				"Alice",
				"alice@example.com",
				time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
			),
		},
		{
			name:  "empty input",
			input: []string{},
			want:  NewTag("", NewCommit("", "", time.Time{}, ""), "", "", "", time.Time{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTagBlock(tt.input)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.Message, got.Message)
			assert.Equal(t, tt.want.TaggerName, got.TaggerName)
			assert.Equal(t, tt.want.TaggerEmail, got.TaggerEmail)
			assert.True(t, tt.want.TaggedOn.Equal(got.TaggedOn), "tagged on time mismatch")
			assert.Equal(t, tt.want.Commit().SHA, got.Commit().SHA)
			assert.Equal(t, tt.want.Commit().Subject, got.Commit().Subject)
			assert.Equal(t, tt.want.Commit().CommittedBy, got.Commit().CommittedBy)
			assert.True(t, tt.want.Commit().CommittedOn.Equal(got.Commit().CommittedOn), "committed on time mismatch")
		})
	}
}

// =============================================================================
// parseTagsFromFormat tests
// =============================================================================

func TestParseTagsFromFormat(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNames  []string
		wantLength int
	}{
		{
			name:       "empty output",
			input:      "",
			wantNames:  []string{},
			wantLength: 0,
		},
		{
			name: "single tag",
			input: `name v1.0.0
objecttype tag
objectsha tag123
derefsha abc1234
taggername John
taggeremail john@example.com
taggedon 2024-01-15T10:30:00Z
message Release
committedby Jane
committedon 2024-01-14T09:00:00Z
commitsubject Initial
`,
			wantNames:  []string{"v1.0.0"},
			wantLength: 1,
		},
		{
			name: "mixed annotated and lightweight tags",
			input: `name v1.0.0
objecttype tag
objectsha tag123
derefsha abc1234
taggername John
taggeremail john@example.com
taggedon 2024-01-15T10:30:00Z
message Release 1.0
committedby Jane
committedon 2024-01-14T09:00:00Z
commitsubject Initial

name v0.1.0
objecttype commit
objectsha def5678
derefsha
taggername
taggeremail
taggedon
message
committedby
committedon
commitsubject

name v2.0.0-beta
objecttype tag
objectsha tag789
derefsha ghi9012
taggername Alice
taggeremail alice@example.com
taggedon 2024-06-01T12:00:00Z
message Beta release
committedby Bob
committedon 2024-05-30T08:00:00Z
commitsubject Beta feature
`,
			wantNames:  []string{"v1.0.0", "v0.1.0", "v2.0.0-beta"},
			wantLength: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTagsFromFormat(tt.input)
			assert.Len(t, got, tt.wantLength)
			assert.Equal(t, tt.wantNames, tagNames(got))
		})
	}
}

// =============================================================================
// parseWorktreeBlock tests
// =============================================================================

func TestParseWorktreeBlock(t *testing.T) {
	g := newTestGitCli()

	// Setup branch and tag maps for tests
	mainBranch := NewLocalBranch("main", "", "/home/user/project", true, 0, 0,
		NewCommit("abc1234567890abcdef1234567890abcdef12345", "Initial", time.Time{}, "John"))
	featureBranch := NewLocalBranch("feature", "", "/home/user/feature", true, 0, 0,
		NewCommit("def5678901234567890abcdef1234567890abcde", "Feature", time.Time{}, "Jane"))

	branchMap := map[string]LocalBranch{
		"main":    mainBranch,
		"feature": featureBranch,
	}

	tagV1 := NewTag("v1.0.0",
		NewCommit("9012345678901234567890abcdef1234567890ab", "Release", time.Time{}, "Bob"),
		"Release", "Bob", "bob@example.com", time.Time{})

	tagMap := map[string]Tag{
		"9012345678901234567890abcdef1234567890ab": tagV1,
	}

	tests := []struct {
		name      string
		input     []string
		branchMap map[string]LocalBranch
		tagMap    map[string]Tag
		wantPath  string
		wantErr   bool
	}{
		{
			name: "worktree with branch",
			input: []string{
				"worktree /home/user/project",
				"HEAD abc1234567890abcdef1234567890abcdef12345",
				"branch refs/heads/main",
			},
			branchMap: branchMap,
			tagMap:    tagMap,
			wantPath:  "/home/user/project",
			wantErr:   false,
		},
		{
			name: "worktree with different branch",
			input: []string{
				"worktree /home/user/feature",
				"HEAD def5678901234567890abcdef1234567890abcde",
				"branch refs/heads/feature",
			},
			branchMap: branchMap,
			tagMap:    tagMap,
			wantPath:  "/home/user/feature",
			wantErr:   false,
		},
		{
			name: "detached HEAD worktree with tag",
			input: []string{
				"worktree /home/user/release",
				"HEAD 9012345678901234567890abcdef1234567890ab",
				"detached",
			},
			branchMap: branchMap,
			tagMap:    tagMap,
			wantPath:  "/home/user/release",
			wantErr:   false,
		},
		{
			name: "bare worktree",
			input: []string{
				"worktree /home/user/bare.git",
				"bare",
			},
			branchMap: branchMap,
			tagMap:    tagMap,
			wantPath:  "/home/user/bare.git",
			wantErr:   false,
		},
		{
			name: "worktree with unknown branch",
			input: []string{
				"worktree /home/user/unknown",
				"HEAD 0000567890abcdef1234567890abcdef12345678",
				"branch refs/heads/unknown",
			},
			branchMap: branchMap,
			tagMap:    tagMap,
			wantPath:  "",
			wantErr:   true,
		},
		{
			name:      "empty input",
			input:     []string{},
			branchMap: branchMap,
			tagMap:    tagMap,
			wantPath:  "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := g.parseWorktreeBlock(tt.input, tt.branchMap, tt.tagMap)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantPath, got.AbsolutePath)
		})
	}
}

// =============================================================================
// Tag helper method tests
// =============================================================================

func TestTag_IsAnnotated(t *testing.T) {
	tests := []struct {
		name string
		tag  Tag
		want bool
	}{
		{
			name: "annotated tag with all fields",
			tag: NewTag("v1.0.0",
				NewCommit("abc123", "Release", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "John"),
				"Release message",
				"John Doe",
				"john@example.com",
				time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)),
			want: true,
		},
		{
			name: "annotated tag with only tagger name",
			tag:  NewTag("v1.0.0", NewCommit("abc123", "", time.Time{}, ""), "", "John", "", time.Time{}),
			want: true,
		},
		{
			name: "annotated tag with only message",
			tag:  NewTag("v1.0.0", NewCommit("abc123", "", time.Time{}, ""), "Release", "", "", time.Time{}),
			want: true,
		},
		{
			name: "annotated tag with only tagged date",
			tag:  NewTag("v1.0.0", NewCommit("abc123", "", time.Time{}, ""), "", "", "", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
			want: true,
		},
		{
			name: "lightweight tag",
			tag:  NewTag("v0.1.0", NewCommit("def456", "", time.Time{}, ""), "", "", "", time.Time{}),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.tag.IsAnnotated())
		})
	}
}

func TestTag_Date(t *testing.T) {
	taggedOn := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	committedOn := time.Date(2024, 6, 10, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		tag  Tag
		want time.Time
	}{
		{
			name: "annotated tag returns TaggedOn",
			tag:  NewTag("v1.0.0", NewCommit("abc123", "Subject", committedOn, "John"), "Msg", "John", "j@e.com", taggedOn),
			want: taggedOn,
		},
		{
			name: "lightweight tag returns CommittedOn",
			tag:  NewTag("v0.1.0", NewCommit("def456", "Subject", committedOn, "John"), "", "", "", time.Time{}),
			want: committedOn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.want.Equal(tt.tag.Date()))
		})
	}
}

// =============================================================================
// WorktreeRef interface tests
// =============================================================================

func TestWorktreeRef_Type(t *testing.T) {
	commit := NewCommit("abc123", "Subject", time.Now(), "Author")
	branch := NewLocalBranch("main", "", "", true, 0, 0, commit)
	tag := NewTag("v1.0.0", commit, "Msg", "Tagger", "t@e.com", time.Now())

	assert.Equal(t, WorktreeRefTypeCommit, commit.Type())
	assert.Equal(t, WorktreeRefTypeBranch, branch.Type())
	assert.Equal(t, WorktreeRefTypeTag, tag.Type())
}

func TestWorktreeRef_FullBranch(t *testing.T) {
	commit := NewCommit("abc123", "Subject", time.Now(), "Author")
	branch := NewLocalBranch("main", "", "", true, 0, 0, commit)
	tag := NewTag("v1.0.0", commit, "Msg", "Tagger", "t@e.com", time.Now())

	b, ok := commit.FullBranch()
	assert.False(t, ok)
	assert.Nil(t, b)

	b, ok = branch.FullBranch()
	assert.True(t, ok)
	assert.NotNil(t, b)
	assert.Equal(t, "main", b.Name)

	b, ok = tag.FullBranch()
	assert.False(t, ok)
	assert.Nil(t, b)
}

func TestWorktreeRef_FullTag(t *testing.T) {
	commit := NewCommit("abc123", "Subject", time.Now(), "Author")
	branch := NewLocalBranch("main", "", "", true, 0, 0, commit)
	tag := NewTag("v1.0.0", commit, "Msg", "Tagger", "t@e.com", time.Now())

	tg, ok := commit.FullTag()
	assert.False(t, ok)
	assert.Nil(t, tg)

	tg, ok = branch.FullTag()
	assert.False(t, ok)
	assert.Nil(t, tg)

	tg, ok = tag.FullTag()
	assert.True(t, ok)
	assert.NotNil(t, tg)
	assert.Equal(t, "v1.0.0", tg.Name)
}

// =============================================================================
// RemoteBranch tests
// =============================================================================

func TestRemoteBranch_FullName(t *testing.T) {
	branch := NewRemoteBranch("main", "origin", NewCommit("abc123", "Subject", time.Now(), "Author"))
	assert.Equal(t, "origin/main", branch.FullName())

	branch2 := NewRemoteBranch("feature/test", "upstream", NewCommit("def456", "Subject", time.Now(), "Author"))
	assert.Equal(t, "upstream/feature/test", branch2.FullName())
}
