package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/grove/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteCheckout(t *testing.T) {
	tests := []struct {
		gitMock          func(workspaceDir string) *mockGit
		name             string
		ref              string
		setupFS          func(t *testing.T, workspaceDir string)
		wantErr          bool
		wantErrContain   string
		wantOutput       string
		worktreeTemplate string
	}{
		{
			name: "local branch exists",
			ref:  "feature/add-user-authentication",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(branchName string, _ bool) (bool, error) {
						return branchName == "feature/add-user-authentication", nil
					},
				}
			},
			wantOutput: "wt-add-user-authentication",
		},
		{
			name: "local branch without prefix",
			ref:  "my-experiment",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(branchName string, _ bool) (bool, error) {
						return branchName == "my-experiment", nil
					},
				}
			},
			wantOutput: "wt-my-experiment",
		},
		{
			name: "local branch not found suggests remote",
			ref:  "nonexistent-branch",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				}
			},
			wantErr:        true,
			wantErrContain: `did you mean "origin/nonexistent-branch"`,
		},
		{
			name: "local branch with slash not found suggests remote",
			ref:  "feature/nonexistent",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					listRemotesFn: func() ([]string, error) {
						return []string{"origin"}, nil
					},
				}
			},
			wantErr:        true,
			wantErrContain: `did you mean "origin/feature/nonexistent"`,
		},
		{
			name: "remote branch fetches and creates worktree",
			ref:  "origin/feature/fix-login",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					listRemotesFn: func() ([]string, error) {
						return []string{"origin"}, nil
					},
					fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
						assert.Equal(t, "origin", remote)
						assert.Equal(t, "feature/fix-login", remoteRef)
						assert.Equal(t, "feature/fix-login", localRef)
						return nil
					},
				}
			},
			wantOutput: "wt-fix-login",
		},
		{
			name: "remote branch with local already existing skips fetch",
			ref:  "origin/feature/fix-login",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					listRemotesFn: func() ([]string, error) {
						return []string{"origin"}, nil
					},
					branchExistsFn: func(branchName string, _ bool) (bool, error) {
						return branchName == "feature/fix-login", nil
					},
					fetchRemoteBranchFn: func(_, _, _ string) error {
						t.Error("FetchRemoteBranch should not be called when branch exists locally")
						return nil
					},
				}
			},
			wantOutput: "wt-fix-login",
		},
		{
			name: "worktree already exists for branch",
			ref:  "feature/fix-login",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					listWorktreesFn: func() ([]git.Worktree, error) {
						return []git.Worktree{
							{
								AbsolutePath: filepath.Join(workspaceDir, "wt-fix-login"),
								Ref:          git.NewLocalBranch("feature/fix-login", "", "", true, 0, 0, git.Commit{}),
							},
						}, nil
					},
				}
			},
			wantErr:        true,
			wantErrContain: "worktree already exists for branch",
		},
		{
			name: "first slash segment not a known remote falls through to local",
			ref:  "feature/fix-login",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					listRemotesFn: func() ([]string, error) {
						return []string{"origin", "upstream"}, nil
					},
					branchExistsFn: func(branchName string, _ bool) (bool, error) {
						return branchName == "feature/fix-login", nil
					},
				}
			},
			wantOutput: "wt-fix-login",
		},
		{
			name: "worktree path already exists on disk",
			ref:  "feature/fix-login",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
				}
			},
			setupFS: func(t *testing.T, workspaceDir string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(filepath.Join(workspaceDir, "wt-fix-login"), 0o755))
			},
			wantErr:        true,
			wantErrContain: "already exists on disk",
		},
		{
			name: "empty ref",
			ref:  "   ",
			gitMock: func(_ string) *mockGit {
				return &mockGit{}
			},
			wantErr:        true,
			wantErrContain: "branch name cannot be empty",
		},
		{
			name:             "invalid naming template",
			ref:              "feature/fix-login",
			worktreeTemplate: "{{.Unknown}}",
			gitMock: func(_ string) *mockGit {
				return &mockGit{
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to initialize local branch namer",
		},
		{
			name: "workspace path error",
			ref:  "feature/fix-login",
			gitMock: func(_ string) *mockGit {
				return &mockGit{
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					getWorkspacePathFn: func() (string, error) {
						return "", fmt.Errorf("git error")
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to get workspace path",
		},
		{
			name: "create worktree error",
			ref:  "feature/fix-login",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					createWorktreeForExistingBranchFn: func(_, _ string) error {
						return assert.AnError
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to create worktree",
		},
		{
			name: "list remotes error",
			ref:  "origin/feature/fix-login",
			gitMock: func(_ string) *mockGit {
				return &mockGit{
					listRemotesFn: func() ([]string, error) {
						return nil, assert.AnError
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to list remotes",
		},
		{
			name: "fetch remote branch error",
			ref:  "origin/feature/fix-login",
			gitMock: func(_ string) *mockGit {
				return &mockGit{
					listRemotesFn: func() ([]string, error) {
						return []string{"origin"}, nil
					},
					fetchRemoteBranchFn: func(_, _, _ string) error {
						return assert.AnError
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to fetch branch",
		},
		{
			name: "upstream remote works",
			ref:  "upstream/hotfix-123",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					listRemotesFn: func() ([]string, error) {
						return []string{"origin", "upstream"}, nil
					},
					fetchRemoteBranchFn: func(remote, remoteRef, localRef string) error {
						assert.Equal(t, "upstream", remote)
						assert.Equal(t, "hotfix-123", remoteRef)
						assert.Equal(t, "hotfix-123", localRef)
						return nil
					},
				}
			},
			wantOutput: "wt-hotfix-123",
		},
		{
			name: "remote ref with empty branch after slash",
			ref:  "origin/",
			gitMock: func(_ string) *mockGit {
				return &mockGit{
					listRemotesFn: func() ([]string, error) {
						return []string{"origin"}, nil
					},
				}
			},
			wantErr:        true,
			wantErrContain: "branch name cannot be empty after remote prefix",
		},
		{
			name: "BranchExists error in local branch path",
			ref:  "my-branch",
			gitMock: func(_ string) *mockGit {
				return &mockGit{
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return false, assert.AnError
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to check if branch exists",
		},
		{
			name: "BranchExists error in remote branch path",
			ref:  "origin/feature/fix-login",
			gitMock: func(_ string) *mockGit {
				return &mockGit{
					listRemotesFn: func() ([]string, error) {
						return []string{"origin"}, nil
					},
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return false, assert.AnError
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to check if branch exists",
		},
		{
			name: "ListWorktrees error",
			ref:  "feature/fix-login",
			gitMock: func(_ string) *mockGit {
				return &mockGit{
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					listWorktreesFn: func() ([]git.Worktree, error) {
						return nil, assert.AnError
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to list worktrees",
		},
		{
			name: "GetDefaultRemote error in local branch not found",
			ref:  "my-branch",
			gitMock: func(_ string) *mockGit {
				return &mockGit{
					getDefaultRemoteFn: func(_ string) (string, error) {
						return "", assert.AnError
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to determine remote",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaceDir := t.TempDir()

			if tt.setupFS != nil {
				tt.setupFS(t, workspaceDir)
			}

			var stdout bytes.Buffer
			cfg := defaultTestConfig()
			if tt.worktreeTemplate != "" {
				cfg.LocalBranch.WorktreeTemplate = tt.worktreeTemplate
			}
			ctx := &checkoutContext{
				cfg:       cfg,
				gitClient: tt.gitMock(workspaceDir),
				logger:    testLogger(),
			}

			err := executeCheckout(&stdout, ctx, tt.ref)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)

			if tt.wantOutput != "" {
				output := strings.TrimSpace(stdout.String())
				assert.Equal(t, filepath.Join(workspaceDir, tt.wantOutput), output)
			}
		})
	}
}

func TestExecuteCheckout_VerifiesGitArgs(t *testing.T) {
	workspaceDir := t.TempDir()

	var gotBranch, gotPath string
	gitMock := &mockGit{
		getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
		branchExistsFn: func(_ string, _ bool) (bool, error) {
			return true, nil
		},
		createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
			gotBranch = branchName
			gotPath = worktreeAbsPath
			return nil
		},
	}

	var stdout bytes.Buffer
	ctx := &checkoutContext{
		cfg:       defaultTestConfig(),
		gitClient: gitMock,
		logger:    testLogger(),
	}

	err := executeCheckout(&stdout, ctx, "feature/fix-login")
	require.NoError(t, err)

	assert.Equal(t, "feature/fix-login", gotBranch)
	assert.Equal(t, filepath.Join(workspaceDir, "wt-fix-login"), gotPath)
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		remotes    []string
		wantParsed parsedRef
		wantErr    bool
	}{
		{
			name:       "simple branch name",
			ref:        "my-branch",
			remotes:    []string{"origin"},
			wantParsed: parsedRef{branchName: "my-branch"},
		},
		{
			name:       "branch with slash not matching remote",
			ref:        "feature/fix-login",
			remotes:    []string{"origin"},
			wantParsed: parsedRef{branchName: "feature/fix-login"},
		},
		{
			name:    "remote ref",
			ref:     "origin/feature/fix-login",
			remotes: []string{"origin"},
			wantParsed: parsedRef{
				branchName: "feature/fix-login",
				remoteName: "origin",
			},
		},
		{
			name:    "upstream remote ref",
			ref:     "upstream/main",
			remotes: []string{"origin", "upstream"},
			wantParsed: parsedRef{
				branchName: "main",
				remoteName: "upstream",
			},
		},
		{
			name:       "no remotes configured",
			ref:        "origin/main",
			remotes:    []string{},
			wantParsed: parsedRef{branchName: "origin/main"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitMock := &mockGit{
				listRemotesFn: func() ([]string, error) {
					return tt.remotes, nil
				},
			}

			got, err := parseRef(gitMock, tt.ref)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantParsed, got)
		})
	}
}
