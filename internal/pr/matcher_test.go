package pr

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

func defaultPRConfig() config.PullRequestConfig {
	return config.PullRequestConfig{
		BranchTemplate: "{{.BranchName}}",
		WorktreePrefix: "pr-",
	}
}

func createNamer(t *testing.T, prCfg config.PullRequestConfig) *naming.PullRequestNamer {
	namer, err := naming.NewPullRequestNamer(prCfg, defaultSlugifyConfig())
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

func createPR(number int, branchName string) github.PullRequest {
	return github.PullRequest{
		AuthorLogin: "testuser",
		BranchName:  branchName,
		Number:      number,
		State:       github.PRStateOpen,
		Title:       "Test PR",
	}
}

func TestMatcher_FindWorktreeForPR(t *testing.T) {
	tests := []struct {
		name           string
		prCfg          config.PullRequestConfig
		pr             github.PullRequest
		worktrees      []git.Worktree
		wantWorktree   *git.Worktree
		wantMatchIndex int // -1 if no match expected
	}{
		{
			name:  "template-generated name match",
			prCfg: defaultPRConfig(),
			pr:    createPR(123, "feature/add-auth"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/pr-feature-add-auth", "feature/add-auth"),
			},
			wantMatchIndex: 1,
		},
		{
			name: "template with PR number match",
			prCfg: config.PullRequestConfig{
				BranchTemplate: "pr/{{.Number}}",
				WorktreePrefix: "pr-",
			},
			pr: createPR(456, "feature/test"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/pr-456", "pr/456"),
			},
			wantMatchIndex: 1,
		},
		{
			name:  "direct branch name match (manual worktree)",
			prCfg: defaultPRConfig(),
			pr:    createPR(789, "fix/bug"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/wt-fix-bug", "fix/bug"), // manually created with different naming
			},
			wantMatchIndex: 1,
		},
		{
			name:  "no match - different branches",
			prCfg: defaultPRConfig(),
			pr:    createPR(123, "feature/add-auth"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/other", "feature/other"),
			},
			wantMatchIndex: -1,
		},
		{
			name:           "no match - empty worktrees",
			prCfg:          defaultPRConfig(),
			pr:             createPR(123, "feature/add-auth"),
			worktrees:      []git.Worktree{},
			wantMatchIndex: -1,
		},
		{
			name:  "no match - worktree has detached HEAD (commit ref)",
			prCfg: defaultPRConfig(),
			pr:    createPR(123, "feature/add-auth"),
			worktrees: []git.Worktree{
				createCommitWorktree("/workspace/detached"),
			},
			wantMatchIndex: -1,
		},
		{
			name:  "matches first matching worktree",
			prCfg: defaultPRConfig(),
			pr:    createPR(123, "feature/add-auth"),
			worktrees: []git.Worktree{
				createWorktree("/workspace/first", "feature/add-auth"),
				createWorktree("/workspace/second", "feature/add-auth"),
			},
			wantMatchIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := createNamer(t, tt.prCfg)
			matcher := NewMatcher(namer)

			got := matcher.FindWorktreeForPR(tt.pr, tt.worktrees)

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
		prCfg     config.PullRequestConfig
		prs       []github.PullRequest
		worktrees []git.Worktree
		wantPaths []string // WorktreePath for each PR (empty if no match)
	}{
		{
			name:  "multiple PRs with some matching",
			prCfg: defaultPRConfig(),
			prs: []github.PullRequest{
				createPR(123, "feature/add-auth"),
				createPR(456, "feature/no-match"),
				createPR(789, "fix/bug"),
			},
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/pr-feature-add-auth", "feature/add-auth"),
				createWorktree("/workspace/pr-fix-bug", "fix/bug"),
			},
			wantPaths: []string{"/workspace/pr-feature-add-auth", "", "/workspace/pr-fix-bug"},
		},
		{
			name:      "empty PRs list",
			prCfg:     defaultPRConfig(),
			prs:       []github.PullRequest{},
			worktrees: []git.Worktree{createWorktree("/workspace/main", "main")},
			wantPaths: []string{},
		},
		{
			name:  "all PRs match",
			prCfg: defaultPRConfig(),
			prs: []github.PullRequest{
				createPR(1, "branch-a"),
				createPR(2, "branch-b"),
			},
			worktrees: []git.Worktree{
				createWorktree("/workspace/pr-branch-a", "branch-a"),
				createWorktree("/workspace/pr-branch-b", "branch-b"),
			},
			wantPaths: []string{"/workspace/pr-branch-a", "/workspace/pr-branch-b"},
		},
		{
			name:  "no PRs match",
			prCfg: defaultPRConfig(),
			prs: []github.PullRequest{
				createPR(1, "branch-a"),
				createPR(2, "branch-b"),
			},
			worktrees: []git.Worktree{
				createWorktree("/workspace/main", "main"),
				createWorktree("/workspace/other", "other"),
			},
			wantPaths: []string{"", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := createNamer(t, tt.prCfg)
			matcher := NewMatcher(namer)

			got := matcher.MatchAll(tt.prs, tt.worktrees)

			require.Len(t, got, len(tt.prs))
			for i, match := range got {
				assert.Equal(t, tt.prs[i].Number, match.PR.Number, "PR number mismatch at index %d", i)
				assert.Equal(t, tt.wantPaths[i] != "", match.HasWorktree(), "HasWorktree mismatch at index %d", i)
				assert.Equal(t, tt.wantPaths[i], match.WorktreePath, "WorktreePath mismatch at index %d", i)
			}
		})
	}
}

func TestNewMatcher(t *testing.T) {
	namer := createNamer(t, defaultPRConfig())
	matcher := NewMatcher(namer)
	assert.NotNil(t, matcher)
	assert.Equal(t, namer, matcher.namer)
}
