package cli

import (
	"bytes"
	"testing"
	"time"

	"github.com/jmcampanini/grove/internal/git"
	"github.com/jmcampanini/grove/internal/github"
	"github.com/jmcampanini/grove/internal/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatherStatuses(t *testing.T) {
	commit := git.NewCommit("abc123", "test", time.Now(), "tester")
	mainBranch := git.NewLocalBranch("main", "origin/main", "/workspace/main", true, 0, 0, commit)
	featureBranch := git.NewLocalBranch("feature/auth", "origin/feature/auth", "/workspace/wt-auth", true, 2, 1, commit)
	localBranch := git.NewLocalBranch("refactor/core", "", "/workspace/wt-refactor", true, 0, 0, commit)

	tests := []struct {
		ghMock        *mockGitHub
		gitMock       *mockGit
		mainWorktree  string
		name          string
		wantCount     int
		wantErr       bool
		wantFirstMain bool
		wantKinds     []string
	}{
		{
			name:         "mixed worktrees with PR and local",
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						{AbsolutePath: "/workspace/main", Ref: &mainBranch},
						{AbsolutePath: "/workspace/wt-auth", Ref: &featureBranch},
						{AbsolutePath: "/workspace/wt-refactor", Ref: &localBranch},
					}, nil
				},
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return []git.LocalBranch{mainBranch, featureBranch, localBranch}, nil
				},
				isWorktreeDirtyFn: func(path string) (bool, error) {
					if path == "/workspace/wt-auth" {
						return true, nil
					}
					return false, nil
				},
			},
			ghMock: &mockGitHub{
				getPullRequestByBranchFn: func(branch string) (*github.PullRequest, error) {
					if branch == "feature/auth" {
						return &github.PullRequest{
							Number: 123,
							State:  github.PRStateOpen,
							Title:  "Add auth",
						}, nil
					}
					return nil, nil
				},
			},
			wantCount:     3,
			wantFirstMain: true,
			wantKinds:     []string{"", "PR", "local"},
		},
		{
			name:         "detached HEAD worktree",
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn: func() ([]git.Worktree, error) {
					return []git.Worktree{
						{AbsolutePath: "/workspace/main", Ref: &mainBranch},
						testWorktreeDetached("/workspace/wt-detached"),
					}, nil
				},
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return []git.LocalBranch{mainBranch}, nil
				},
				isWorktreeDirtyFn: func(_ string) (bool, error) { return false, nil },
			},
			ghMock:        &mockGitHub{},
			wantCount:     2,
			wantFirstMain: true,
			wantKinds:     []string{"", ""},
		},
		{
			name:         "empty worktree list",
			mainWorktree: "/workspace/main",
			gitMock: &mockGit{
				listWorktreesFn:     func() ([]git.Worktree, error) { return nil, nil },
				listLocalBranchesFn: func() ([]git.LocalBranch, error) { return nil, nil },
			},
			ghMock:    &mockGitHub{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &worktree.StatusContext{
				GitHubClient:     tt.ghMock,
				GitClient:        tt.gitMock,
				MainWorktreePath: tt.mainWorktree,
			}

			statuses, err := gatherStatuses(ctx)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, statuses, tt.wantCount)

			if tt.wantFirstMain && len(statuses) > 0 {
				assert.True(t, statuses[0].IsMain, "first worktree should be main")
			}

			if tt.wantKinds != nil {
				for i, kind := range tt.wantKinds {
					assert.Equal(t, kind, statuses[i].Kind, "kind[%d]", i)
				}
			}
		})
	}
}

func TestRenderStatusTable(t *testing.T) {
	tests := []struct {
		name       string
		statuses   []worktree.Status
		wantErr    bool
		wantOutput []string
	}{
		{
			name:       "empty list shows message",
			statuses:   nil,
			wantOutput: []string{"No worktrees found."},
		},
		{
			name: "main worktree renders",
			statuses: []worktree.Status{
				{
					AbsPath:    "/workspace/main",
					BranchName: "main",
					IsMain:     true,
					Tracking:   worktree.TrackingInfo{Upstream: "origin/main"},
				},
			},
			wantOutput: []string{"main", "\u2261"},
		},
		{
			name: "dirty worktree shows dirty",
			statuses: []worktree.Status{
				{
					AbsPath:    "/workspace/wt-feature",
					BranchName: "feature/test",
					Dirty:      true,
					Kind:       "local",
				},
			},
			wantOutput: []string{"wt-feature", "feature/test", "dirty", "local"},
		},
		{
			name: "PR worktree shows PR info",
			statuses: []worktree.Status{
				{
					AbsPath:    "/workspace/wt-auth",
					BranchName: "feature/auth",
					Kind:       "PR",
					PR: &github.PullRequest{
						LinesAdded:   140,
						LinesDeleted: 30,
						Number:       123,
						State:        github.PRStateOpen,
					},
				},
			},
			wantOutput: []string{"wt-auth", "PR", "#123", "open", "+140", "-30"},
		},
		{
			name: "tracking ahead and behind",
			statuses: []worktree.Status{
				{
					AbsPath:    "/workspace/wt-feature",
					BranchName: "feature/test",
					Kind:       "local",
					Tracking:   worktree.TrackingInfo{Ahead: 2, Behind: 1, Upstream: "origin/feature/test"},
				},
			},
			wantOutput: []string{"\u21912", "\u21931"},
		},
		{
			name: "detached HEAD no branch",
			statuses: []worktree.Status{
				{AbsPath: "/workspace/wt-detached"},
			},
			wantOutput: []string{"wt-detached"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := renderStatusTable(plainWriter(&buf), tt.statuses)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			output := buf.String()
			for _, want := range tt.wantOutput {
				assert.Contains(t, output, want, "output should contain %q", want)
			}
		})
	}
}

func TestFormatTracking(t *testing.T) {
	tests := []struct {
		name     string
		tracking worktree.TrackingInfo
		want     string
	}{
		{
			name:     "no upstream",
			tracking: worktree.TrackingInfo{},
			want:     "",
		},
		{
			name:     "in sync",
			tracking: worktree.TrackingInfo{Upstream: "origin/main"},
			want:     "\u2261",
		},
		{
			name:     "ahead only",
			tracking: worktree.TrackingInfo{Ahead: 3, Upstream: "origin/main"},
			want:     "\u21913",
		},
		{
			name:     "behind only",
			tracking: worktree.TrackingInfo{Behind: 2, Upstream: "origin/main"},
			want:     "\u21932",
		},
		{
			name:     "ahead and behind",
			tracking: worktree.TrackingInfo{Ahead: 1, Behind: 4, Upstream: "origin/main"},
			want:     "\u21911 \u21934",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTracking(tt.tracking)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatChecksSummary(t *testing.T) {
	tests := []struct {
		checks []github.StatusCheck
		name   string
		want   string
	}{
		{
			name:   "no checks",
			checks: nil,
			want:   "",
		},
		{
			name: "all passing",
			checks: []github.StatusCheck{
				{Conclusion: github.CheckConclusionSuccess},
				{Conclusion: github.CheckConclusionSuccess},
			},
			want: "2/2 checks",
		},
		{
			name: "some failing",
			checks: []github.StatusCheck{
				{Conclusion: github.CheckConclusionSuccess},
				{Conclusion: github.CheckConclusionFailure},
				{Conclusion: github.CheckConclusionSuccess},
			},
			want: "2/3 checks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatChecksSummary(tt.checks)
			if tt.want == "" {
				assert.Empty(t, got)
			} else {
				assert.Contains(t, got, tt.want)
			}
		})
	}
}

func TestFormatReviewSummary(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		reviews []github.Review
		want    string
	}{
		{
			name:    "no reviews",
			reviews: nil,
			want:    "",
		},
		{
			name: "approved",
			reviews: []github.Review{
				{AuthorLogin: "alice", State: github.ReviewStateApproved, SubmittedAt: now},
			},
			want: "approved",
		},
		{
			name: "changes requested takes priority",
			reviews: []github.Review{
				{AuthorLogin: "alice", State: github.ReviewStateApproved, SubmittedAt: now},
				{AuthorLogin: "bob", State: github.ReviewStateChangesRequested, SubmittedAt: now},
			},
			want: "changes requested",
		},
		{
			name: "comment only shows count",
			reviews: []github.Review{
				{AuthorLogin: "alice", State: github.ReviewStateCommented, SubmittedAt: now},
			},
			want: "1 review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatReviewSummary(tt.reviews)
			if tt.want == "" {
				assert.Empty(t, got)
			} else {
				assert.Contains(t, got, tt.want)
			}
		})
	}
}
