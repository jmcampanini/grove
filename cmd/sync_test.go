package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteSync(t *testing.T) {
	tests := []struct {
		force          bool
		gitMock        *mockGit
		name           string
		wantErr        bool
		wantErrContain string
		wantOutput     string
	}{
		{
			name: "clean worktree with upstream syncs successfully",
			gitMock: &mockGit{
				getCurrentBranchFn: func() (string, error) { return "feature/my-branch", nil },
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return []git.LocalBranch{
						git.NewLocalBranch("feature/my-branch", "origin/feature/my-branch", "", true, 0, 0, git.Commit{}),
					}, nil
				},
				fetchRemoteFn:     func(string) (string, error) { return "", nil },
				getWorktreeRootFn: func() (string, error) { return "/workspace/wt-my-branch", nil },
				isWorktreeDirtyFn: func(string) (bool, error) { return false, nil },
				resetHardFn:       func(string) error { return nil },
			},
			wantOutput: "Synced feature/my-branch to origin/feature/my-branch\n",
		},
		{
			name: "detached HEAD returns error",
			gitMock: &mockGit{
				getCurrentBranchFn: func() (string, error) { return "HEAD", nil },
			},
			wantErr:        true,
			wantErrContain: "detached HEAD",
		},
		{
			name: "no upstream prints message and exits",
			gitMock: &mockGit{
				getCurrentBranchFn: func() (string, error) { return "local-only", nil },
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return []git.LocalBranch{
						git.NewLocalBranch("local-only", "", "", true, 0, 0, git.Commit{}),
					}, nil
				},
			},
			wantOutput: "Branch \"local-only\" has no remote tracking branch.\n",
		},
		{
			name:  "dirty worktree with force syncs anyway",
			force: true,
			gitMock: &mockGit{
				getCurrentBranchFn: func() (string, error) { return "main", nil },
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return []git.LocalBranch{
						git.NewLocalBranch("main", "origin/main", "", true, 0, 0, git.Commit{}),
					}, nil
				},
				fetchRemoteFn:     func(string) (string, error) { return "", nil },
				getWorktreeRootFn: func() (string, error) { return "/workspace/main", nil },
				isWorktreeDirtyFn: func(string) (bool, error) { return true, nil },
				resetHardFn:       func(string) error { return nil },
			},
			wantOutput: "Synced main to origin/main\n",
		},
		{
			name: "fetch failure propagates error",
			gitMock: &mockGit{
				getCurrentBranchFn: func() (string, error) { return "main", nil },
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return []git.LocalBranch{
						git.NewLocalBranch("main", "origin/main", "", true, 0, 0, git.Commit{}),
					}, nil
				},
				fetchRemoteFn: func(string) (string, error) { return "", errors.New("network error") },
			},
			wantErr:        true,
			wantErrContain: "failed to fetch",
		},
		{
			name: "branch not found in list returns no upstream message",
			gitMock: &mockGit{
				getCurrentBranchFn: func() (string, error) { return "feature/x", nil },
				listLocalBranchesFn: func() ([]git.LocalBranch, error) {
					return []git.LocalBranch{
						git.NewLocalBranch("main", "origin/main", "", false, 0, 0, git.Commit{}),
					}, nil
				},
			},
			wantOutput: "Branch \"feature/x\" has no remote tracking branch.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			ctx := &syncContext{gitClient: tt.gitMock}

			err := executeSync(&buf, ctx, tt.force)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			if tt.wantOutput != "" {
				assert.Equal(t, tt.wantOutput, buf.String())
			}
		})
	}
}

func TestFindRemoteForUpstream(t *testing.T) {
	tests := []struct {
		name       string
		remotes    []string
		upstream   string
		wantErr    bool
		wantRemote string
	}{
		{
			name:       "simple origin",
			upstream:   "origin/main",
			remotes:    []string{"origin"},
			wantRemote: "origin",
		},
		{
			name:       "multiple remotes picks correct one",
			upstream:   "upstream/feature/foo",
			remotes:    []string{"origin", "upstream"},
			wantRemote: "upstream",
		},
		{
			name:       "remote with slash matches longest prefix",
			upstream:   "my/remote/main",
			remotes:    []string{"my", "my/remote"},
			wantRemote: "my/remote",
		},
		{
			name:     "no matching remote",
			upstream: "unknown/main",
			remotes:  []string{"origin"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remote, err := findRemoteForUpstream(tt.upstream, tt.remotes)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRemote, remote)
		})
	}
}
