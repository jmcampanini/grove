package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/naming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLocalNamer(t *testing.T, worktreeTemplate string) *naming.LocalBranchNamer {
	t.Helper()
	cfg := defaultTestConfig()
	cfg.LocalBranch.WorktreeTemplate = worktreeTemplate
	namer, err := naming.NewLocalBranchNamer(cfg.LocalBranch, cfg.Naming)
	require.NoError(t, err)
	return namer
}

func testPRNamer(t *testing.T, worktreeTemplate string) *naming.PullRequestNamer {
	t.Helper()
	cfg := defaultTestConfig()
	cfg.PullRequest.WorktreeTemplate = worktreeTemplate
	namer, err := naming.NewPullRequestNamer(cfg.PullRequest, cfg.Naming)
	require.NoError(t, err)
	return namer
}

func TestGetDisplayName(t *testing.T) {
	tests := []struct {
		absPath    string
		name       string
		want       string
		wtTemplate string
	}{
		{
			name:       "standard worktree with template literal",
			absPath:    "/workspace/wt-add-auth",
			wtTemplate: "wt-{{.BranchSlug}}",
			want:       "add-auth",
		},
		{
			name:       "main worktree without literal match",
			absPath:    "/workspace/main",
			wtTemplate: "wt-{{.BranchSlug}}",
			want:       "[main]",
		},
		{
			name:       "custom template literal",
			absPath:    "/workspace/work-feature",
			wtTemplate: "work-{{.BranchSlug}}",
			want:       "feature",
		},
		{
			name:       "empty literal matches everything without stripping",
			absPath:    "/workspace/anything",
			wtTemplate: "{{.BranchSlug}}",
			want:       "anything",
		},
		{
			name:       "partial literal match wraps in brackets",
			absPath:    "/workspace/wt_add-auth",
			wtTemplate: "wt-{{.BranchSlug}}",
			want:       "[wt_add-auth]",
		},
		{
			name:       "nested path extracts basename",
			absPath:    "/deep/nested/path/wt-feature",
			wtTemplate: "wt-{{.BranchSlug}}",
			want:       "feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := testLocalNamer(t, tt.wtTemplate)
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
		localLiteral string
		name         string
		prLiteral    string
		wantDisplay  string
		wantPath     string
		worktree     git.Worktree
	}{
		{
			name: "local branch with template literal stripped",
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
			localLiteral: "wt-",
			prLiteral:    "pr-",
			wantPath:     "/ws/wt-add-auth",
			wantDisplay:  "local branch add-auth feature/add-auth",
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
			localLiteral: "wt-",
			prLiteral:    "pr-",
			wantPath:     "/ws/main",
			wantDisplay:  "local branch [main] main",
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
			localLiteral: "wt-",
			prLiteral:    "pr-",
			wantPath:     "/ws/wt-v1",
			wantDisplay:  "tag v1 v1.0.0",
		},
		{
			name: "detached HEAD worktree",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-hotfix",
				Ref:          git.NewCommit("abc1234def5678", "Hotfix", now, "user"),
			},
			localLiteral: "wt-",
			prLiteral:    "pr-",
			wantPath:     "/ws/wt-hotfix",
			wantDisplay:  "detached hotfix abc1234",
		},
		{
			name: "detached HEAD with short SHA",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-short",
				Ref:          git.NewCommit("abc", "Short SHA", now, "user"),
			},
			localLiteral: "wt-",
			prLiteral:    "pr-",
			wantPath:     "/ws/wt-short",
			wantDisplay:  "detached short abc",
		},
		{
			name: "detached HEAD with empty SHA",
			worktree: git.Worktree{
				AbsolutePath: "/ws/wt-empty",
				Ref:          git.NewCommit("", "No SHA", now, "user"),
			},
			localLiteral: "wt-",
			prLiteral:    "pr-",
			wantPath:     "/ws/wt-empty",
			wantDisplay:  "detached empty (no sha)",
		},
		{
			name: "custom PR template literal shows [PR] marker",
			worktree: git.Worktree{
				AbsolutePath: "/ws/review-42-feature-auth",
				Ref: git.NewLocalBranch(
					"feature/auth",
					"origin/feature/auth",
					"/ws/review-42-feature-auth",
					true,
					0, 0,
					git.NewCommit("abc1234def5678", "Add auth", now, "user"),
				),
			},
			localLiteral: "wt-",
			prLiteral:    "review-",
			wantPath:     "/ws/review-42-feature-auth",
			wantDisplay:  "local branch [PR] [review-42-feature-auth] feature/auth",
		},
		{
			name: "action-leading templates strip nothing and tag no PR",
			worktree: git.Worktree{
				AbsolutePath: "/ws/42-feature-auth",
				Ref: git.NewLocalBranch(
					"feature/auth",
					"origin/feature/auth",
					"/ws/42-feature-auth",
					true,
					0, 0,
					git.NewCommit("abc1234def5678", "Add auth", now, "user"),
				),
			},
			localLiteral: "",
			prLiteral:    "",
			wantPath:     "/ws/42-feature-auth",
			wantDisplay:  "local branch 42-feature-auth feature/auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			localNamer := testLocalNamer(t, tt.localLiteral+"{{.BranchSlug}}")
			prNamer := testPRNamer(t, tt.prLiteral+"{{.Number}}")
			gotPath, gotDisplay := formatWorktree(tt.worktree, localNamer, prNamer)
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

	localNamer := testLocalNamer(t, "wt-{{.BranchSlug}}")
	prNamer := testPRNamer(t, "pr-{{.Number}}")
	_, display := formatWorktree(worktree, localNamer, prNamer)

	// Verify no tabs in display string
	assert.NotContains(t, display, "\t", "display string should not contain tabs")

	// Verify no trailing whitespace
	assert.Equal(t, display, strings.TrimRight(display, " \t"), "display string should have no trailing whitespace")

	// Verify proper spacing (single spaces between parts)
	assert.NotContains(t, display, "  ", "display string should not have double spaces")
}

func TestFormatWorktreeName(t *testing.T) {
	tests := []struct {
		dirName   string
		display   string
		name      string
		prLiteral string
		want      string
	}{
		{
			name:      "PR worktree gets marker",
			display:   "feature-auth",
			dirName:   "pr-feature-auth",
			prLiteral: "pr-",
			want:      "[PR] feature-auth",
		},
		{
			name:      "regular worktree no marker",
			display:   "feature-auth",
			dirName:   "wt-feature-auth",
			prLiteral: "pr-",
			want:      "feature-auth",
		},
		{
			name:      "main worktree no marker",
			display:   "[main]",
			dirName:   "main",
			prLiteral: "pr-",
			want:      "[main]",
		},
		{
			name:      "custom PR literal",
			display:   "bug-fix",
			dirName:   "review-bug-fix",
			prLiteral: "review-",
			want:      "[PR] bug-fix",
		},
		{
			name:      "empty literal never matches",
			display:   "feature",
			dirName:   "feature",
			prLiteral: "",
			want:      "feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatWorktreeName(tt.display, tt.dirName, tt.prLiteral)
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
		configure        func(*config.Config)
		fzf              bool
		listWorktreesFn  func() ([]git.Worktree, error)
		mainWorktreePath string
		name             string
		wantErr          string
		wantErrContains  string
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
			name:             "numbered PR worktree gets PR marker in fzf mode",
			mainWorktreePath: "/ws/main",
			fzf:              true,
			listWorktreesFn: func() ([]git.Worktree, error) {
				return []git.Worktree{
					branchWT("/ws/pr-42-auth-fix", "feature/auth-fix", "aaa1111aaa1111"),
				}, nil
			},
			wantOutput: "/ws/pr-42-auth-fix\tlocal branch [PR] [pr-42-auth-fix] feature/auth-fix\n",
		},
		{
			name:             "local namer constructor error has context",
			mainWorktreePath: "/ws/main",
			fzf:              true,
			configure: func(cfg *config.Config) {
				cfg.LocalBranch.WorktreeTemplate = "{{.Unknown}}"
			},
			listWorktreesFn: func() ([]git.Worktree, error) {
				return nil, nil
			},
			wantErrContains: "failed to create local branch namer:",
		},
		{
			name:             "PR namer constructor error has context",
			mainWorktreePath: "/ws/main",
			fzf:              true,
			configure: func(cfg *config.Config) {
				cfg.PullRequest.WorktreeTemplate = "{{.Unknown}}"
			},
			listWorktreesFn: func() ([]git.Worktree, error) {
				return nil, nil
			},
			wantErrContains: "failed to create PR namer:",
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

			cfg := defaultTestConfig()
			if tt.configure != nil {
				tt.configure(&cfg)
			}
			ctx := &listContext{
				cfg:              cfg,
				gitClient:        &mockGit{listWorktreesFn: tt.listWorktreesFn},
				mainWorktreePath: tt.mainWorktreePath,
			}

			err := executeList(&buf, ctx, tt.fzf)

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantOutput, buf.String())
			}
		})
	}
}
