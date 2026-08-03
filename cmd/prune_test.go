package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindPrunable(t *testing.T) {
	workspace := t.TempDir()
	for _, name := range []string{"main", "wt-merged", "wt-closed", "wt-open", "wt-gone", "wt-keep"} {
		require.NoError(t, os.Mkdir(filepath.Join(workspace, name), 0o755))
	}

	remoteBranches := map[string]bool{
		"origin/main":         true,
		"origin/feature/keep": true,
	}

	tests := []struct {
		name        string
		statuses    []worktreeStatus
		wantCount   int
		wantNames   []string
		wantReasons []string
	}{
		{
			name: "merged PR is prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-merged"),
					branchName: "feature/merged",
					pr:         &github.PullRequest{Number: 10, State: github.PRStateMerged},
				},
			},
			wantCount:   1,
			wantNames:   []string{"wt-merged"},
			wantReasons: []string{"PR #10 merged"},
		},
		{
			name: "closed PR is prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-closed"),
					branchName: "feature/closed",
					pr:         &github.PullRequest{Number: 20, State: github.PRStateClosed},
				},
			},
			wantCount:   1,
			wantNames:   []string{"wt-closed"},
			wantReasons: []string{"PR #20 closed"},
		},
		{
			name: "open PR is not prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-open"),
					branchName: "feature/open",
					pr:         &github.PullRequest{Number: 30, State: github.PRStateOpen},
				},
			},
			wantCount: 0,
		},
		{
			name: "upstream gone is prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-gone"),
					branchName: "feature/gone",
					kind:       "local",
					tracking:   trackingInfo{upstream: "origin/feature/gone"},
				},
			},
			wantCount:   1,
			wantNames:   []string{"wt-gone"},
			wantReasons: []string{"upstream gone"},
		},
		{
			name: "upstream exists is not prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-keep"),
					branchName: "feature/keep",
					kind:       "local",
					tracking:   trackingInfo{upstream: "origin/feature/keep"},
				},
			},
			wantCount: 0,
		},
		{
			name: "main worktree is never prunable",
			statuses: []worktreeStatus{
				{
					absPath:    filepath.Join(workspace, "main"),
					branchName: "main",
					isMain:     true,
					pr:         &github.PullRequest{Number: 1, State: github.PRStateMerged},
				},
			},
			wantCount: 0,
		},
		{
			name:      "no prunable worktrees",
			statuses:  []worktreeStatus{{absPath: filepath.Join(workspace, "main"), isMain: true}},
			wantCount: 0,
		},
		{
			name: "multiple prunable reasons",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-merged"),
					branchName: "feature/merged",
					pr:         &github.PullRequest{Number: 10, State: github.PRStateMerged},
				},
				{
					absPath:    filepath.Join(workspace, "wt-gone"),
					branchName: "feature/gone",
					kind:       "local",
					tracking:   trackingInfo{upstream: "origin/feature/gone"},
				},
			},
			wantCount:   2,
			wantNames:   []string{"wt-merged", "wt-gone"},
			wantReasons: []string{"PR #10 merged", "upstream gone"},
		},
		{
			name: "orphaned worktree is prunable",
			statuses: []worktreeStatus{
				{absPath: filepath.Join(workspace, "main"), isMain: true},
				{
					absPath:    filepath.Join(workspace, "wt-orphaned"),
					branchName: "feature/orphaned",
				},
			},
			wantCount:   1,
			wantNames:   []string{"wt-orphaned"},
			wantReasons: []string{"orphaned"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findPrunable(tt.statuses, remoteBranches)

			assert.Len(t, result, tt.wantCount)

			if tt.wantNames != nil {
				for i, name := range tt.wantNames {
					assert.Equal(t, name, result[i].name, "name[%d]", i)
				}
			}

			if tt.wantReasons != nil {
				for i, reason := range tt.wantReasons {
					assert.Equal(t, reason, result[i].evidence.String(), "reason[%d]", i)
				}
			}
		})
	}
}

func TestPruneReason(t *testing.T) {
	wtDir := t.TempDir()

	remoteBranches := map[string]bool{
		"origin/main": true,
	}

	tests := []struct {
		name   string
		status worktreeStatus
		want   string
	}{
		{
			name: "merged PR",
			status: worktreeStatus{
				absPath: wtDir,
				pr:      &github.PullRequest{Number: 5, State: github.PRStateMerged},
			},
			want: "PR #5 merged",
		},
		{
			name: "closed PR",
			status: worktreeStatus{
				absPath: wtDir,
				pr:      &github.PullRequest{Number: 7, State: github.PRStateClosed},
			},
			want: "PR #7 closed",
		},
		{
			name: "open PR not prunable",
			status: worktreeStatus{
				absPath: wtDir,
				pr:      &github.PullRequest{Number: 8, State: github.PRStateOpen},
			},
			want: "",
		},
		{
			name: "upstream gone",
			status: worktreeStatus{
				absPath:  wtDir,
				tracking: trackingInfo{upstream: "origin/feature/gone"},
			},
			want: "upstream gone",
		},
		{
			name: "upstream exists",
			status: worktreeStatus{
				absPath:  wtDir,
				tracking: trackingInfo{upstream: "origin/main"},
			},
			want: "",
		},
		{
			name: "no tracking no PR",
			status: worktreeStatus{
				absPath: wtDir,
			},
			want: "",
		},
		{
			name: "orphaned directory",
			status: worktreeStatus{
				absPath: "/nonexistent/wt-orphaned",
			},
			want: "orphaned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pruneReason(tt.status, remoteBranches)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRevalidatePrunable(t *testing.T) {
	tests := []struct {
		branchAfterGitHub   string
		candidateEvidence   pruneEvidence
		currentBranch       string
		currentPR           *github.PullRequest
		currentUpstream     string
		ghErr               error
		name                string
		pathMissing         bool
		registered          bool
		remoteExists        bool
		upstreamAfterGitHub string
		wantCurrent         string
		wantErr             string
		wantUnchanged       bool
	}{
		{
			candidateEvidence: pruneEvidence{kind: pruneEvidencePR, prNumber: 10, prState: github.PRStateMerged},
			currentBranch:     "feature/test",
			currentPR:         &github.PullRequest{Number: 10, State: github.PRStateMerged},
			name:              "unchanged merged PR",
			registered:        true,
			wantCurrent:       "PR #10 merged",
			wantUnchanged:     true,
		},
		{
			candidateEvidence: pruneEvidence{kind: pruneEvidencePR, prNumber: 10, prState: github.PRStateClosed},
			currentBranch:     "feature/test",
			currentPR:         &github.PullRequest{Number: 10, State: github.PRStateOpen},
			name:              "reopened PR",
			registered:        true,
			wantCurrent:       "PR #10 open",
		},
		{
			candidateEvidence: pruneEvidence{kind: pruneEvidencePR, prNumber: 10, prState: github.PRStateMerged},
			currentBranch:     "feature/test",
			currentPR:         &github.PullRequest{Number: 15, State: github.PRStateMerged},
			name:              "branch reused by different merged PR",
			registered:        true,
			wantCurrent:       "PR #15 merged",
		},
		{
			candidateEvidence: pruneEvidence{kind: pruneEvidencePR, prNumber: 10, prState: github.PRStateClosed},
			currentBranch:     "feature/test",
			currentPR:         &github.PullRequest{Number: 10, State: github.PRStateOpen},
			currentUpstream:   "origin/feature/test",
			name:              "reason changes from closed PR to upstream gone",
			registered:        true,
			wantCurrent:       "upstream gone",
		},
		{
			candidateEvidence: pruneEvidence{kind: pruneEvidenceUpstreamGone, upstream: "origin/feature/test"},
			currentBranch:     "feature/test",
			currentUpstream:   "origin/feature/test",
			name:              "unchanged missing upstream",
			registered:        true,
			wantCurrent:       "upstream gone",
			wantUnchanged:     true,
		},
		{
			candidateEvidence: pruneEvidence{kind: pruneEvidenceUpstreamGone, upstream: "origin/feature/test"},
			currentBranch:     "feature/test",
			currentUpstream:   "origin/feature/test",
			name:              "upstream reappears",
			registered:        true,
			remoteExists:      true,
			wantCurrent:       `upstream "origin/feature/test" exists`,
		},
		{
			candidateEvidence: pruneEvidence{kind: pruneEvidencePR, prNumber: 10, prState: github.PRStateMerged},
			currentBranch:     "feature/reused",
			name:              "path maps to another branch",
			registered:        true,
			wantCurrent:       `path now maps to branch "feature/reused"`,
		},
		{
			candidateEvidence: pruneEvidence{kind: pruneEvidenceOrphaned},
			currentBranch:     "feature/test",
			name:              "unchanged orphan",
			pathMissing:       true,
			registered:        true,
			wantCurrent:       "orphaned",
			wantUnchanged:     true,
		},
		{
			candidateEvidence: pruneEvidence{kind: pruneEvidencePR, prNumber: 10, prState: github.PRStateMerged},
			currentBranch:     "feature/test",
			name:              "worktree no longer registered",
			wantCurrent:       "worktree is no longer registered",
		},
		{
			candidateEvidence: pruneEvidence{kind: pruneEvidencePR, prNumber: 10, prState: github.PRStateMerged},
			currentBranch:     "feature/test",
			ghErr:             errors.New("gh timed out"),
			name:              "GitHub lookup error",
			registered:        true,
			wantErr:           "failed to refresh PR state",
		},
		{
			branchAfterGitHub: "feature/reused",
			candidateEvidence: pruneEvidence{kind: pruneEvidencePR, prNumber: 10, prState: github.PRStateMerged},
			currentBranch:     "feature/test",
			currentPR:         &github.PullRequest{Number: 10, State: github.PRStateMerged},
			name:              "path changes branch during GitHub lookup",
			registered:        true,
			wantCurrent:       `path now maps to branch "feature/reused"`,
		},
		{
			candidateEvidence:   pruneEvidence{kind: pruneEvidenceUpstreamGone, upstream: "origin/feature/test"},
			currentBranch:       "feature/test",
			currentUpstream:     "origin/feature/test",
			name:                "upstream changes during GitHub lookup",
			registered:          true,
			upstreamAfterGitHub: "origin/replacement",
			wantCurrent:         `upstream is now "origin/replacement"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := t.TempDir()
			path := filepath.Join(workspace, "wt-test")
			require.NoError(t, os.Mkdir(path, 0o755))
			if tt.pathMissing {
				require.NoError(t, os.Remove(path))
			}

			commit := git.NewCommit("abc123", "test", time.Unix(0, 0), "tester")
			currentBranch := tt.currentBranch
			currentUpstream := tt.currentUpstream
			makeBranch := func() git.LocalBranch {
				return git.NewLocalBranch(currentBranch, currentUpstream, path, true, 0, 0, commit)
			}

			gitMock := &mockGit{
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return []git.LocalBranch{makeBranch()}, nil
				},
				listRemoteBranchesFn: func(remote string) ([]git.RemoteBranch, error) {
					if tt.remoteExists && remote == "origin" {
						return []git.RemoteBranch{git.NewRemoteBranch("feature/test", "origin", commit)}, nil
					}
					return nil, nil
				},
				listWorktreesFn: func() ([]git.Worktree, error) {
					if !tt.registered {
						return nil, nil
					}
					branch := makeBranch()
					return []git.Worktree{{AbsolutePath: path, Ref: &branch}}, nil
				},
			}
			ghMock := &mockGitHub{
				getPullRequestByBranchFn: func(string) (*github.PullRequest, error) {
					if tt.branchAfterGitHub != "" {
						currentBranch = tt.branchAfterGitHub
					}
					if tt.upstreamAfterGitHub != "" {
						currentUpstream = tt.upstreamAfterGitHub
					}
					return tt.currentPR, tt.ghErr
				},
			}
			ctx := &statusContext{
				ghClient:         ghMock,
				gitClient:        gitMock,
				mainWorktreePath: filepath.Join(workspace, "main"),
			}
			candidate := prunable{
				branchName: "feature/test",
				evidence:   tt.candidateEvidence,
				name:       "wt-test",
				path:       path,
			}

			current, unchanged, err := revalidatePrunable(ctx, candidate)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				assert.False(t, unchanged)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantCurrent, current)
			assert.Equal(t, tt.wantUnchanged, unchanged)
		})
	}
}

func TestExecuteRemovals(t *testing.T) {
	upstreamEvidence := func(branch string) pruneEvidence {
		return pruneEvidence{kind: pruneEvidenceUpstreamGone, upstream: "origin/" + branch}
	}

	tests := []struct {
		changed          map[string]string
		name             string
		prunables        []prunable
		removeErr        map[string]error
		revalidationErr  map[string]error
		wantContains     []string
		wantErr          bool
		wantErrContain   string
		wantRemovedPaths []string
	}{
		{
			name: "successful removal",
			prunables: []prunable{
				{branchName: "feature/test", evidence: upstreamEvidence("feature/test"), name: "wt-test", path: "/workspace/wt-test"},
			},
			wantContains:     []string{"wt-test", "feature/test", "upstream gone", "1 removed"},
			wantRemovedPaths: []string{"/workspace/wt-test"},
		},
		{
			name: "multiple removals",
			prunables: []prunable{
				{branchName: "feature/a", evidence: upstreamEvidence("feature/a"), name: "wt-a", path: "/workspace/wt-a"},
				{
					branchName: "feature/b",
					evidence:   pruneEvidence{kind: pruneEvidencePR, prNumber: 5, prState: github.PRStateMerged},
					name:       "wt-b",
					path:       "/workspace/wt-b",
				},
			},
			wantContains:     []string{"wt-a", "wt-b", "2 removed"},
			wantRemovedPaths: []string{"/workspace/wt-a", "/workspace/wt-b"},
		},
		{
			changed: map[string]string{
				"/workspace/wt-changed": "PR #10 open",
			},
			name: "changed candidate is skipped while another is removed",
			prunables: []prunable{
				{
					branchName: "feature/changed",
					evidence:   pruneEvidence{kind: pruneEvidencePR, prNumber: 10, prState: github.PRStateMerged},
					name:       "wt-changed",
					path:       "/workspace/wt-changed",
				},
				{branchName: "feature/keep", evidence: upstreamEvidence("feature/keep"), name: "wt-keep", path: "/workspace/wt-keep"},
			},
			wantContains: []string{
				"skipped: was PR #10 merged, now PR #10 open; rerun grove prune",
				"1 removed",
				"1 skipped",
			},
			wantRemovedPaths: []string{"/workspace/wt-keep"},
		},
		{
			name: "revalidation error fails while another is removed",
			prunables: []prunable{
				{branchName: "feature/fail", evidence: upstreamEvidence("feature/fail"), name: "wt-fail", path: "/workspace/wt-fail"},
				{branchName: "feature/keep", evidence: upstreamEvidence("feature/keep"), name: "wt-keep", path: "/workspace/wt-keep"},
			},
			revalidationErr: map[string]error{
				"/workspace/wt-fail": errors.New("gh timed out"),
			},
			wantContains:     []string{"wt-fail", "revalidation failed: gh timed out", "1 removed", "1 failed"},
			wantErr:          true,
			wantErrContain:   "1 removal(s) failed",
			wantRemovedPaths: []string{"/workspace/wt-keep"},
		},
		{
			name: "removal error is reported",
			prunables: []prunable{
				{branchName: "feature/fail", evidence: upstreamEvidence("feature/fail"), name: "wt-fail", path: "/workspace/wt-fail"},
			},
			removeErr: map[string]error{
				"/workspace/wt-fail": errors.New("permission denied"),
			},
			wantContains:   []string{"wt-fail", "permission denied", "1 failed"},
			wantErr:        true,
			wantErrContain: "1 removal(s) failed",
		},
		{
			name: "orphaned worktree uses targeted removal",
			prunables: []prunable{
				{
					branchName: "feature/orphan",
					evidence:   pruneEvidence{kind: pruneEvidenceOrphaned},
					name:       "wt-orphan",
					path:       "/workspace/wt-orphan",
				},
			},
			wantContains:     []string{"wt-orphan", "orphaned", "1 removed"},
			wantRemovedPaths: []string{"/workspace/wt-orphan"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var removedPaths []string
			mock := &mockGit{
				removeWorktreeFn: func(path string, _ bool) error {
					if err := tt.removeErr[path]; err != nil {
						return err
					}
					removedPaths = append(removedPaths, path)
					return nil
				},
			}
			revalidate := func(p prunable) (string, bool, error) {
				if err := tt.revalidationErr[p.path]; err != nil {
					return "", false, err
				}
				if current, ok := tt.changed[p.path]; ok {
					return current, false, nil
				}
				return p.evidence.String(), true, nil
			}

			var buf bytes.Buffer
			err := executeRemovals(&buf, mock, tt.prunables, revalidate)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantRemovedPaths, removedPaths)
			output := buf.String()
			for _, s := range tt.wantContains {
				assert.Contains(t, output, s, "output should contain %q", s)
			}
		})
	}
}
