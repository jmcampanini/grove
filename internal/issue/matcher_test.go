package issue

import (
	"testing"
	"time"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultSlugifyConfig() config.SlugifyConfig {
	return config.SlugifyConfig{
		CollapseDashes:     true,
		HashLength:         0,
		Lowercase:          true,
		MaxLength:          0,
		ReplaceNonAlphanum: true,
		TrimDashes:         true,
	}
}

func defaultIssueConfig() config.IssueConfig {
	return config.IssueConfig{
		BranchTemplate:     "issue/{{.Number}}-{{.TitleSlug}}",
		TitleSlugMaxLength: 40,
		WorktreePrefix:     "issue-",
	}
}

func createNamer(t *testing.T, issueCfg config.IssueConfig) *naming.IssueNamer {
	namer, err := naming.NewIssueNamer(issueCfg, defaultSlugifyConfig())
	require.NoError(t, err)
	return namer
}

func createWorktree(path string, branchName string) git.Worktree {
	commit := git.NewCommit("abc123", "Test commit", time.Now(), "tester")
	branch := git.NewLocalBranch(branchName, "", path, true, 0, 0, commit)
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
		AuthorLogin: "testuser",
		Number:      number,
		State:       github.IssueStateOpen,
		Title:       title,
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
				createWorktree("/workspace/issue-123-fix-login-crash", "issue/123-fix-login-crash"),
			},
			wantMatchIndex: 1,
		},
		{
			name:     "match survives issue title edits",
			issueCfg: defaultIssueConfig(),
			issue:    createIssue(123, "Completely renamed title"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/issue-123-fix-login-crash", "issue/123-fix-login-crash"),
			},
			wantMatchIndex: 0,
		},
		{
			name:     "longer issue number does not match",
			issueCfg: defaultIssueConfig(),
			issue:    createIssue(123, "Fix login crash"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/issue-1234-other", "issue/1234-other"),
			},
			wantMatchIndex: -1,
		},
		{
			name: "number-only template",
			issueCfg: config.IssueConfig{
				BranchTemplate:     "issue/{{.Number}}",
				TitleSlugMaxLength: 40,
				WorktreePrefix:     "issue-",
			},
			issue: createIssue(456, "Anything"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/issue-456", "issue/456"),
			},
			wantMatchIndex: 1,
		},
		{
			name: "title-first template matches exact regeneration",
			issueCfg: config.IssueConfig{
				BranchTemplate:     "{{.TitleSlug}}-{{.Number}}",
				TitleSlugMaxLength: 40,
				WorktreePrefix:     "issue-",
			},
			issue: createIssue(789, "Fix bug"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/issue-fix-bug-789", "fix-bug-789"),
			},
			wantMatchIndex: 0,
		},
		{
			name: "title-first template misses after title edit",
			issueCfg: config.IssueConfig{
				BranchTemplate:     "{{.TitleSlug}}-{{.Number}}",
				TitleSlugMaxLength: 40,
				WorktreePrefix:     "issue-",
			},
			issue: createIssue(789, "Renamed bug"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/issue-fix-bug-789", "fix-bug-789"),
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
			matcher := NewMatcher(namer)

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

func TestMatcher_MatchAll(t *testing.T) {
	tests := []struct {
		name      string
		issues    []github.Issue
		worktrees []git.Worktree
		wantPaths []string // WorktreePath for each issue (empty if no match)
	}{
		{
			name: "multiple issues with some matching",
			issues: []github.Issue{
				createIssue(123, "Fix login crash"),
				createIssue(456, "No worktree yet"),
				createIssue(789, "Add dark mode"),
			},
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/issue-123-fix-login-crash", "issue/123-fix-login-crash"),
				createWorktree("/workspace/issue-789-add-dark-mode", "issue/789-add-dark-mode"),
			},
			wantPaths: []string{"/workspace/issue-123-fix-login-crash", "", "/workspace/issue-789-add-dark-mode"},
		},
		{
			name:      "empty issues list",
			issues:    []github.Issue{},
			worktrees: []git.Worktree{createWorktree("/workspace/main", "main")},
			wantPaths: []string{},
		},
		{
			name: "no issues match",
			issues: []github.Issue{
				createIssue(1, "One"),
				createIssue(2, "Two"),
			},
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/wt-other", "feature/other"),
			},
			wantPaths: []string{"", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := createNamer(t, defaultIssueConfig())
			matcher := NewMatcher(namer)

			got := matcher.MatchAll(tt.issues, tt.worktrees)

			require.Len(t, got, len(tt.issues))
			for i, match := range got {
				assert.Equal(t, tt.issues[i].Number, match.Issue.Number, "issue number mismatch at index %d", i)
				assert.Equal(t, tt.wantPaths[i] != "", match.HasWorktree(), "HasWorktree mismatch at index %d", i)
				assert.Equal(t, tt.wantPaths[i], match.WorktreePath, "WorktreePath mismatch at index %d", i)
			}
		})
	}
}

func TestNewMatcher(t *testing.T) {
	namer := createNamer(t, defaultIssueConfig())
	matcher := NewMatcher(namer)
	assert.NotNil(t, matcher)
	assert.Equal(t, namer, matcher.namer)
	assert.NotNil(t, matcher.log)
	assert.Equal(t, "issue", matcher.log.GetPrefix())
}
