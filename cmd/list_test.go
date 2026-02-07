package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/stretchr/testify/assert"
)

// testNamer creates a LocalBranchNamer with the given prefix for testing.
func testNamer(prefix string) *naming.LocalBranchNamer {
	return naming.NewLocalBranchNamer(
		config.LocalBranchConfig{NewPrefix: prefix},
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
