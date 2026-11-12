package issue

import (
	"testing"
	"time"

	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/naming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultNamingConfig() config.NamingConfig {
	return config.NamingConfig{
		Lowercase:     true,
		MaxLength:     30,
		StripPrefixes: []string{"feature/", "fix/", "issue/"},
	}
}

func defaultIssueConfig() config.IssueConfig {
	return config.IssueConfig{
		BranchTemplate:   "issue/{{.Number}}-{{.TitleSlug}}",
		WorktreeTemplate: "is-{{.Number}}-{{.TitleSlug}}",
	}
}

func createNamer(t *testing.T, issueCfg config.IssueConfig) *naming.IssueNamer {
	namer, err := naming.NewIssueNamer(issueCfg, defaultNamingConfig())
	require.NoError(t, err)
	return namer
}

func createWorktree(path string, branchName string) git.Worktree {
	commit := git.NewCommit("abc123", "Test commit", time.Now(), "tester")
	branch := git.NewLocalBranch(branchName, "", path, true, commit)
	return git.Worktree{
		AbsolutePath: path,
		Ref:          branch,
	}
}

func createCommitWorktree(path string) git.Worktree {
	commit := git.NewCommit("abc123", "Test commit", time.Now(), "tester")
	return git.Worktree{
		AbsolutePath: path,
		Ref:          commit,
	}
}

func createIssue(number int, title string) github.Issue {
	return github.Issue{
		Number: number,
		State:  github.IssueStateOpen,
		Title:  title,
	}
}

func TestMatcher_FindWorktreeForIssue(t *testing.T) {
	tests := []struct {
		name           string
		issueCfg       config.IssueConfig
		issue          github.Issue
		worktrees      []git.Worktree
		wantMatchIndex int // -1 if no match expected
	}{
		{
			name:     "anchored match on issue number",
			issueCfg: defaultIssueConfig(),
			issue:    createIssue(123, "Fix login crash"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/is-123-fix-login-crash", "issue/123-fix-login-crash"),
			},
			wantMatchIndex: 1,
		},
		{
			name:     "match survives issue title edits",
			issueCfg: defaultIssueConfig(),
			issue:    createIssue(123, "Completely renamed title"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/is-123-fix-login-crash", "issue/123-fix-login-crash"),
			},
			wantMatchIndex: 0,
		},
		{
			name:     "longer issue number does not match",
			issueCfg: defaultIssueConfig(),
			issue:    createIssue(123, "Fix login crash"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/is-1234-other", "issue/1234-other"),
			},
			wantMatchIndex: -1,
		},
		{
			name: "number-only template",
			issueCfg: config.IssueConfig{
				BranchTemplate:   "issue/{{.Number}}",
				WorktreeTemplate: "is-{{.Number}}-{{.TitleSlug}}",
			},
			issue: createIssue(456, "Anything"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/is-456", "issue/456"),
			},
			wantMatchIndex: 1,
		},
		{
			name: "title-first template matches exact regeneration",
			issueCfg: config.IssueConfig{
				BranchTemplate:   "{{.TitleSlug}}-{{.Number}}",
				WorktreeTemplate: "is-{{.Number}}-{{.TitleSlug}}",
			},
			issue: createIssue(789, "Fix bug"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/is-fix-bug-789", "fix-bug-789"),
			},
			wantMatchIndex: 0,
		},
		{
			name: "title-first template misses after title edit",
			issueCfg: config.IssueConfig{
				BranchTemplate:   "{{.TitleSlug}}-{{.Number}}",
				WorktreeTemplate: "is-{{.Number}}-{{.TitleSlug}}",
			},
			issue: createIssue(789, "Renamed bug"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/is-fix-bug-789", "fix-bug-789"),
			},
			wantMatchIndex: -1,
		},
		{
			name:           "no match - empty worktrees",
			issueCfg:       defaultIssueConfig(),
			issue:          createIssue(123, "Fix login crash"),
			worktrees:      []git.Worktree{},
			wantMatchIndex: -1,
		},
		{
			name:     "no match - worktree has detached HEAD (commit ref)",
			issueCfg: defaultIssueConfig(),
			issue:    createIssue(123, "Fix login crash"),
			worktrees: []git.Worktree{
				createCommitWorktree("/workspace/detached"),
			},
			wantMatchIndex: -1,
		},
		{
			name:     "matches first matching worktree",
			issueCfg: defaultIssueConfig(),
			issue:    createIssue(123, "Fix login crash"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/first", "issue/123-fix-login-crash"),
				createWorktree("/workspace/second", "issue/123-fix-login-crash"),
			},
			wantMatchIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := createNamer(t, tt.issueCfg)
			matcher := NewMatcher(namer, nil)

			got := matcher.FindWorktreeForIssue(tt.issue, tt.worktrees)

			if tt.wantMatchIndex == -1 {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.worktrees[tt.wantMatchIndex].AbsolutePath, got.AbsolutePath)
			}
		})
	}
}

func TestNewMatcher(t *testing.T) {
	namer := createNamer(t, defaultIssueConfig())
	matcher := NewMatcher(namer, nil)
	assert.NotNil(t, matcher)
	assert.Equal(t, namer, matcher.namer)
	assert.NotNil(t, matcher.log)
	assert.Equal(t, "issue", matcher.log.GetPrefix())
}
