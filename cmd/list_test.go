package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testNamer creates a LocalBranchNamer with the given prefix for testing.
func testNamer(prefix string) *naming.LocalBranchNamer {
	return naming.NewLocalBranchNamer(
		config.LocalBranchConfig{WorktreePrefix: prefix},
		config.SlugifyConfig{},
	)
}

func TestGetDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		absPath  string
		wtPrefix string
		want     string
	}{
		{
			name:     "standard worktree with prefix",
			absPath:  "/workspace/wt-add-auth",
			wtPrefix: "wt-",
			want:     "add-auth",
		},
		{
			name:     "main worktree without prefix",
			absPath:  "/workspace/main",
			wtPrefix: "wt-",
			want:     "[main]",
		},
		{
			name:     "different prefix",
			absPath:  "/workspace/work-feature",
			wtPrefix: "work-",
			want:     "feature",
		},
		{
			name:     "empty prefix matches everything",
			absPath:  "/workspace/anything",
			wtPrefix: "",
			want:     "anything",
		},
		{
			name:     "partial prefix match wraps in brackets",
			absPath:  "/workspace/wt_add-auth",
			wtPrefix: "wt-",
			want:     "[wt_add-auth]",
		},
		{
			name:     "nested path extracts basename",
			absPath:  "/deep/nested/path/wt-feature",
			wtPrefix: "wt-",
			want:     "feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := testNamer(tt.wtPrefix)
			got := getDisplayName(namer, tt.absPath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShortSHASafe(t *testing.T) {
	tests := []struct {
		name   string
		sha    string
		maxLen int
		want   string
	}{
		{
			name:   "normal SHA truncated",
			sha:    "abc1234def5678",
			maxLen: 7,
			want:   "abc1234",
		},
		{
			name:   "SHA exactly maxLen",
			sha:    "abc1234",
			maxLen: 7,
			want:   "abc1234",
		},
		{
			name:   "SHA shorter than maxLen",
			sha:    "abc",
			maxLen: 7,
			want:   "abc",
		},
		{
			name:   "empty SHA returns placeholder",
			sha:    "",
			maxLen: 7,
			want:   "(no sha)",
		},
		{
			name:   "maxLen of 0 returns empty",
			sha:    "abc1234",
			maxLen: 0,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortSHASafe(tt.sha, tt.maxLen)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatWorktree(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		prPrefix    string
		wantDisplay string
		wantPath    string
		worktree    git.Worktree
		wtPrefix    string
	}{
		{
			name: "local branch with prefix stripped",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-add-auth",
				Ref: git.NewLocalBranch(
					"feature/add-auth",
					"origin/feature/add-auth",
					"/ws/wt-add-auth",
					true,
					0, 0,
					git.NewCommit("abc1234def5678", "Add auth", now, "user"),
				),
			},
			wtPrefix:    "wt-",
			prPrefix:    "pr-",
			wantPath:    "/ws/wt-add-auth",
			wantDisplay: "local branch add-auth feature/add-auth",
		},
		{
			name: "local branch without prefix match",
			worktree: git.Worktree{
				AbsolutePath: "/ws/main",
				Ref: git.NewLocalBranch(
					"main",
					"origin/main",
					"/ws/main",
					true,
					0, 0,
					git.NewCommit("abc1234def5678", "Initial", now, "user"),
				),
			},
			wtPrefix:    "wt-",
			prPrefix:    "pr-",
			wantPath:    "/ws/main",
			wantDisplay: "local branch [main] main",
		},
		{
			name: "tag worktree",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-v1",
				Ref: git.NewTag(
					"v1.0.0",
					git.NewCommit("abc1234def5678", "Release", now, "user"),
					"Release v1.0.0",
					"Tagger",
					"tagger@example.com",
					now,
				),
			},
			wtPrefix:    "wt-",
			prPrefix:    "pr-",
			wantPath:    "/ws/wt-v1",
			wantDisplay: "tag v1 v1.0.0",
		},
		{
			name: "detached HEAD worktree",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-hotfix",
				Ref:          git.NewCommit("abc1234def5678", "Hotfix", now, "user"),
			},
			wtPrefix:    "wt-",
			prPrefix:    "pr-",
			wantPath:    "/ws/wt-hotfix",
			wantDisplay: "detached hotfix abc1234",
		},
		{
			name: "detached HEAD with short SHA",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-short",
				Ref:          git.NewCommit("abc", "Short SHA", now, "user"),
			},
			wtPrefix:    "wt-",
			prPrefix:    "pr-",
			wantPath:    "/ws/wt-short",
			wantDisplay: "detached short abc",
		},
		{
			name: "detached HEAD with empty SHA",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-empty",
				Ref:          git.NewCommit("", "No SHA", now, "user"),
			},
			wtPrefix:    "wt-",
			prPrefix:    "pr-",
			wantPath:    "/ws/wt-empty",
			wantDisplay: "detached empty (no sha)",
		},
		{
			name: "PR worktree shows [PR] marker",
			worktree: git.Worktree{
				AbsolutePath: "/ws/pr-feature-auth",
				Ref: git.NewLocalBranch(
					"feature/auth",
					"origin/feature/auth",
					"/ws/pr-feature-auth",
					true,
					0, 0,
					git.NewCommit("abc1234def5678", "Add auth", now, "user"),
				),
			},
			wtPrefix:    "wt-",
			prPrefix:    "pr-",
			wantPath:    "/ws/pr-feature-auth",
			wantDisplay: "local branch [PR] [pr-feature-auth] feature/auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := testNamer(tt.wtPrefix)
			gotPath, gotDisplay := formatWorktree(tt.worktree, namer, tt.prPrefix)
			assert.Equal(t, tt.wantPath, gotPath)
			assert.Equal(t, tt.wantDisplay, gotDisplay)
		})
	}
}

func TestFormatWorktreeTabSeparation(t *testing.T) {
	now := time.Now()

	worktree := git.Worktree{
		AbsolutePath: "/ws/wt-test",
		Ref: git.NewLocalBranch(
			"feature/test",
			"",
			"/ws/wt-test",
			true,
			0, 0,
			git.NewCommit("abc1234", "Test", now, "user"),
		),
	}

	namer := testNamer("wt-")
	_, display := formatWorktree(worktree, namer, "pr-")

	// Verify no tabs in display string
	assert.NotContains(t, display, "\t", "display string should not contain tabs")

	// Verify no trailing whitespace
	assert.Equal(t, display, strings.TrimRight(display, " \t"), "display string should have no trailing whitespace")

	// Verify proper spacing (single spaces between parts)
	assert.NotContains(t, display, "  ", "display string should not have double spaces")
}

func TestFormatWorktreeName(t *testing.T) {
	tests := []struct {
		dirName     string
		displayName string
		name        string
		prPrefix    string
		want        string
	}{
		{
			name:        "PR worktree gets marker",
			displayName: "feature-auth",
			dirName:     "pr-feature-auth",
			prPrefix:    "pr-",
			want:        "[PR] feature-auth",
		},
		{
			name:        "regular worktree no marker",
			displayName: "feature-auth",
			dirName:     "wt-feature-auth",
			prPrefix:    "pr-",
			want:        "feature-auth",
		},
		{
			name:        "main worktree no marker",
			displayName: "[main]",
			dirName:     "main",
			prPrefix:    "pr-",
			want:        "[main]",
		},
		{
			name:        "custom PR prefix",
			displayName: "bug-fix",
			dirName:     "review-bug-fix",
			prPrefix:    "review-",
			want:        "[PR] bug-fix",
		},
		{
			name:        "empty prefix never matches",
			displayName: "feature",
			dirName:     "feature",
			prPrefix:    "",
			want:        "feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatWorktreeName(tt.displayName, tt.dirName, tt.prPrefix)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExecuteList(t *testing.T) {
	now := time.Now()
	commit := func(sha string) git.Commit {
		return git.NewCommit(sha, "msg", now, "user")
	}
	branchWT := func(path, branchName, sha string) git.Worktree {
		return git.Worktree{
			AbsolutePath: path,
			Ref:          git.NewLocalBranch(branchName, "", path, true, 0, 0, commit(sha)),
		}
	}

	tests := []struct {
		fzf              bool
		listWorktreesFn  func() ([]git.Worktree, error)
		mainWorktreePath string
		name             string
		wantErr          string
		wantOutput       string
	}{
		{
			name:             "plain mode outputs one path per line",
			mainWorktreePath: "/ws/main",
			fzf:              false,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return []git.Worktree{
					branchWT("/ws/main", "main", "aaa1111"),
					branchWT("/ws/wt-alpha", "feature/alpha", "bbb2222"),
					branchWT("/ws/wt-beta", "feature/beta", "ccc3333"),
				}, nil
			},
			wantOutput: "/ws/main\n/ws/wt-alpha\n/ws/wt-beta\n",
		},
		{
			name:             "main worktree listed first",
			mainWorktreePath: "/ws/main",
			fzf:              false,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return []git.Worktree{
					branchWT("/ws/wt-zebra", "feature/zebra", "ddd4444"),
					branchWT("/ws/wt-alpha", "feature/alpha", "eee5555"),
					branchWT("/ws/main", "main", "fff6666"),
				}, nil
			},
			wantOutput: "/ws/main\n/ws/wt-alpha\n/ws/wt-zebra\n",
		},
		{
			name:             "other worktrees sorted alphabetically",
			mainWorktreePath: "/ws/main",
			fzf:              false,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return []git.Worktree{
					branchWT("/ws/main", "main", "aaa1111"),
					branchWT("/ws/wt-charlie", "feature/charlie", "bbb2222"),
					branchWT("/ws/wt-alpha", "feature/alpha", "ccc3333"),
					branchWT("/ws/wt-bravo", "feature/bravo", "ddd4444"),
				}, nil
			},
			wantOutput: "/ws/main\n/ws/wt-alpha\n/ws/wt-bravo\n/ws/wt-charlie\n",
		},
		{
			name:             "fzf mode outputs tab-separated format",
			mainWorktreePath: "/ws/main",
			fzf:              true,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return []git.Worktree{
					branchWT("/ws/main", "main", "aaa1111aaa1111"),
					branchWT("/ws/wt-feat", "feature/feat", "bbb2222bbb2222"),
				}, nil
			},
			wantOutput: "/ws/main\tlocal branch [main] main\n" +
				"/ws/wt-feat\tlocal branch feat feature/feat\n",
		},
		{
			name:             "fzf mode with mixed ref types",
			mainWorktreePath: "/ws/main",
			fzf:              true,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return []git.Worktree{
					branchWT("/ws/main", "main", "aaa1111aaa1111"),
					{
						AbsolutePath: "/ws/wt-release",
						Ref: git.NewTag(
							"v1.0.0",
							commit("bbb2222bbb2222"),
							"Release", "Tagger", "t@e.com", now,
						),
					},
					{
						AbsolutePath: "/ws/wt-hotfix",
						Ref:          commit("ccc3333ccc3333"),
					},
				}, nil
			},
			wantOutput: "/ws/main\tlocal branch [main] main\n" +
				"/ws/wt-hotfix\tdetached hotfix ccc3333\n" +
				"/ws/wt-release\ttag release v1.0.0\n",
		},
		{
			name:             "empty worktree list produces no output",
			mainWorktreePath: "/ws/main",
			fzf:              false,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return []git.Worktree{}, nil
			},
			wantOutput: "",
		},
		{
			name:             "only main worktree",
			mainWorktreePath: "/ws/main",
			fzf:              false,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return []git.Worktree{
					branchWT("/ws/main", "main", "aaa1111"),
				}, nil
			},
			wantOutput: "/ws/main\n",
		},
		{
			name:             "git client error propagates",
			mainWorktreePath: "/ws/main",
			fzf:              false,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return nil, errors.New("connection refused")
			},
			wantErr: "failed to list worktrees: connection refused",
		},
		{
			name:             "PR worktree gets PR marker in fzf mode",
			mainWorktreePath: "/ws/main",
			fzf:              true,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return []git.Worktree{
					branchWT("/ws/pr-auth-fix", "feature/auth-fix", "aaa1111aaa1111"),
				}, nil
			},
			wantOutput: "/ws/pr-auth-fix\tlocal branch [PR] [pr-auth-fix] feature/auth-fix\n",
		},
		{
			name:             "main worktree path not found outputs all worktrees sorted",
			mainWorktreePath: "/ws/nonexistent",
			fzf:              false,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return []git.Worktree{
					branchWT("/ws/wt-beta", "feature/beta", "bbb2222"),
					branchWT("/ws/wt-alpha", "feature/alpha", "aaa1111"),
				}, nil
			},
			wantOutput: "/ws/wt-alpha\n/ws/wt-beta\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			ctx := &listContext{
				cfg:              defaultTestConfig(),
				gitClient:        &mockGit{listWorktreesFn: tt.listWorktreesFn},
				mainWorktreePath: tt.mainWorktreePath,
			}

			err := executeList(&buf, ctx, tt.fzf)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErr, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantOutput, buf.String())
			}
		})
	}
}
