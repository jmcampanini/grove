package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteCreate(t *testing.T) {
	tests := []struct {
		name           string
		phrase         string
		reuse          bool
		gitMock        func(workspaceDir string) *mockGit
		setupFS        func(t *testing.T, workspaceDir string)
		wantErr        bool
		wantErrContain string
		wantOutput     string
	}{
		{
			name:   "simple phrase",
			phrase: "add logging support",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				}
			},
			wantOutput: "wt-add-logging-support",
		},
		{
			name:   "special characters",
			phrase: "fix: handle 404 & 500 errors!",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				}
			},
			wantOutput: "wt-fix-handle-404-500-errors",
		},
		{
			name:   "mixed casing",
			phrase: "Add OAuth2 Google Integration",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				}
			},
			wantOutput: "wt-add-oauth2-google-integration",
		},
		{
			name:   "long phrase triggers hash truncation",
			phrase: "implement comprehensive user authentication and authorization system with role based access",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				}
			},
			wantOutput: "wt-implement-comprehensive-user-authentication-a-nquu",
		},
		{
			name:   "duplicate branch",
			phrase: "add logging support",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(branchName string, caseInsensitive bool) (bool, error) {
						return true, nil
					},
				}
			},
			wantErr:        true,
			wantErrContain: "already exists",
		},
		{
			name:           "empty phrase",
			phrase:         "   ",
			gitMock:        func(workspaceDir string) *mockGit { return &mockGit{} },
			wantErr:        true,
			wantErrContain: "phrase cannot be empty",
		},
		{
			name:   "all special chars slugifies to empty",
			phrase: "@#$%^&*",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				}
			},
			wantErr:        true,
			wantErrContain: "empty branch name after slugification",
		},
		{
			name:   "worktree path already exists on disk",
			phrase: "add logging support",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				}
			},
			setupFS: func(t *testing.T, workspaceDir string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(filepath.Join(workspaceDir, "wt-add-logging-support"), 0o755))
			},
			wantErr:        true,
			wantErrContain: "already exists",
		},
		{
			name:   "workspace path error",
			phrase: "add logging support",
			gitMock: func(_ string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) {
						return "", fmt.Errorf("git error")
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to get workspace path",
		},
		{
			name:   "worktree creation error",
			phrase: "add logging support",
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					createWorktreeForNewBranchFromRefFn: func(_, _, _ string) error {
						return assert.AnError
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to create branch and worktree",
		},
		{
			name:   "reuse with existing worktree",
			phrase: "add logging support",
			reuse:  true,
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					listWorktreesFn: func() ([]git.Worktree, error) {
						return []git.Worktree{
							{
								AbsolutePath: filepath.Join(workspaceDir, "wt-add-logging-support"),
								Ref:          git.NewLocalBranch("feature/add-logging-support", "", "", false, 0, 0, git.Commit{}),
							},
						}, nil
					},
				}
			},
			setupFS: func(t *testing.T, workspaceDir string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(filepath.Join(workspaceDir, "wt-add-logging-support"), 0o755))
			},
			wantOutput: "wt-add-logging-support",
		},
		{
			name:   "reuse with stale worktree entry",
			phrase: "add logging support",
			reuse:  true,
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					listWorktreesFn: func() ([]git.Worktree, error) {
						return []git.Worktree{
							{
								AbsolutePath: filepath.Join(workspaceDir, "nonexistent-dir"),
								Ref:          git.NewLocalBranch("feature/add-logging-support", "", "", false, 0, 0, git.Commit{}),
							},
						}, nil
					},
				}
			},
			wantOutput: "wt-add-logging-support",
		},
		{
			name:   "reuse with stale worktree entry and prune fails",
			phrase: "add logging support",
			reuse:  true,
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					listWorktreesFn: func() ([]git.Worktree, error) {
						return []git.Worktree{
							{
								AbsolutePath: filepath.Join(workspaceDir, "nonexistent-dir"),
								Ref:          git.NewLocalBranch("feature/add-logging-support", "", "", false, 0, 0, git.Commit{}),
							},
						}, nil
					},
					pruneWorktreesFn: func() error {
						return assert.AnError
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to prune stale worktrees",
		},
		{
			name:   "reuse with nothing existing",
			phrase: "add logging support",
			reuse:  true,
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				}
			},
			wantOutput: "wt-add-logging-support",
		},
		{
			name:   "reuse with branch but no worktree",
			phrase: "add logging support",
			reuse:  true,
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					listWorktreesFn: func() ([]git.Worktree, error) {
						return nil, nil
					},
				}
			},
			wantOutput: "wt-add-logging-support",
		},
		{
			name:   "reuse with branch but no worktree and create fails",
			phrase: "add logging support",
			reuse:  true,
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					listWorktreesFn: func() ([]git.Worktree, error) {
						return nil, nil
					},
					createWorktreeForExistingBranchFn: func(_, _ string) error {
						return assert.AnError
					},
				}
			},
			wantErr:        true,
			wantErrContain: "failed to create worktree for existing branch",
		},
		{
			name:   "reuse with branch and orphaned dir on disk",
			phrase: "add logging support",
			reuse:  true,
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					listWorktreesFn: func() ([]git.Worktree, error) {
						return nil, nil
					},
				}
			},
			setupFS: func(t *testing.T, workspaceDir string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(filepath.Join(workspaceDir, "wt-add-logging-support"), 0o755))
			},
			wantErr:        true,
			wantErrContain: "already exists",
		},
		{
			name:   "reuse with ListWorktrees error",
			phrase: "add logging support",
			reuse:  true,
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
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
			name:   "reuse with orphaned dir on disk",
			phrase: "add logging support",
			reuse:  true,
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				}
			},
			setupFS: func(t *testing.T, workspaceDir string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(filepath.Join(workspaceDir, "wt-add-logging-support"), 0o755))
			},
			wantErr:        true,
			wantErrContain: "already exists",
		},
		{
			name:   "reuse with non-matching worktrees",
			phrase: "add logging support",
			reuse:  true,
			gitMock: func(workspaceDir string) *mockGit {
				return &mockGit{
					getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
					branchExistsFn: func(_ string, _ bool) (bool, error) {
						return true, nil
					},
					listWorktreesFn: func() ([]git.Worktree, error) {
						return []git.Worktree{
							{
								AbsolutePath: filepath.Join(workspaceDir, "wt-something-else"),
								Ref:          git.NewLocalBranch("feature/something-else", "", "", false, 0, 0, git.Commit{}),
							},
						}, nil
					},
				}
			},
			wantOutput: "wt-add-logging-support",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaceDir := t.TempDir()

			if tt.setupFS != nil {
				tt.setupFS(t, workspaceDir)
			}

			var stdout bytes.Buffer
			ctx := &createContext{
				cfg:       defaultTestConfig(),
				gitClient: tt.gitMock(workspaceDir),
				reuse:     tt.reuse,
			}

			err := executeCreate(&stdout, ctx, tt.phrase)

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

func TestExecuteCreate_ValidatesIncompatibleOptions(t *testing.T) {
	tests := []struct {
		name           string
		ctx            createContext
		wantErrContain string
	}{
		{
			name: "--reuse and --from cannot be used together",
			ctx: createContext{
				baseRef: "main",
				reuse:   true,
			},
			wantErrContain: "--reuse and --from cannot be used together",
		},
		{
			name: "--from and --from-remote-primary cannot be used together",
			ctx: createContext{
				baseRef:           "main",
				fromRemotePrimary: true,
			},
			wantErrContain: "--from and --from-remote-primary cannot be used together",
		},
		{
			name: "--reuse and --from-remote-primary cannot be used together",
			ctx: createContext{
				fromRemotePrimary: true,
				reuse:             true,
			},
			wantErrContain: "--reuse and --from-remote-primary cannot be used together",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.ctx
			ctx.cfg = defaultTestConfig()
			ctx.gitClient = &mockGit{}

			var stdout bytes.Buffer
			err := executeCreate(&stdout, &ctx, "add logging support")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrContain)
		})
	}
}

func TestExecuteCreate_FromRemotePrimary(t *testing.T) {
	const expectedWorktreeName = "wt-add-logging-support"

	tests := []struct {
		name                    string
		defaultBranch           string
		defaultRemote           string
		fetchErr                error
		getDefaultRemoteErr     error
		getRemoteBranchErr      error
		wantBaseRef             string
		wantDefaultBranchRemote string
		wantErrContain          string
		wantFetchBranch         string
		wantFetchRemote         string
	}{
		{
			name:                    "success with origin main",
			defaultBranch:           "main",
			defaultRemote:           "origin",
			wantBaseRef:             "origin/main",
			wantDefaultBranchRemote: "origin",
			wantFetchBranch:         "main",
			wantFetchRemote:         "origin",
		},
		{
			name:                    "custom default branch",
			defaultBranch:           "develop",
			defaultRemote:           "origin",
			wantBaseRef:             "origin/develop",
			wantDefaultBranchRemote: "origin",
			wantFetchBranch:         "develop",
			wantFetchRemote:         "origin",
		},
		{
			name:                    "custom default remote",
			defaultBranch:           "trunk",
			defaultRemote:           "upstream",
			wantBaseRef:             "upstream/trunk",
			wantDefaultBranchRemote: "upstream",
			wantFetchBranch:         "trunk",
			wantFetchRemote:         "upstream",
		},
		{
			name:                    "missing default branch",
			defaultBranch:           "",
			defaultRemote:           "origin",
			wantDefaultBranchRemote: "origin",
			wantErrContain:          "could not determine default branch for remote \"origin\"",
		},
		{
			name:                    "fetch failure propagates",
			defaultBranch:           "main",
			defaultRemote:           "origin",
			fetchErr:                assert.AnError,
			wantDefaultBranchRemote: "origin",
			wantErrContain:          "failed to fetch default branch \"main\" from remote \"origin\"",
			wantFetchBranch:         "main",
			wantFetchRemote:         "origin",
		},
		{
			name:                "default remote failure propagates",
			getDefaultRemoteErr: assert.AnError,
			wantErrContain:      "failed to determine default remote",
		},
		{
			name:                    "default branch failure propagates",
			defaultRemote:           "origin",
			getRemoteBranchErr:      assert.AnError,
			wantDefaultBranchRemote: "origin",
			wantErrContain:          "failed to determine default branch for remote \"origin\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaceDir := t.TempDir()

			var gotBaseRef, gotBranch, gotDefaultBranchRemote, gotFetchBranch, gotFetchRemote, gotPath string
			gitMock := &mockGit{
				getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				getDefaultRemoteFn: func(fallback string) (string, error) {
					assert.Equal(t, "origin", fallback)
					if tt.getDefaultRemoteErr != nil {
						return "", tt.getDefaultRemoteErr
					}
					return tt.defaultRemote, nil
				},
				getRemoteDefaultBranchFn: func(remoteName string) (string, error) {
					gotDefaultBranchRemote = remoteName
					if tt.getRemoteBranchErr != nil {
						return "", tt.getRemoteBranchErr
					}
					return tt.defaultBranch, nil
				},
				fetchRemoteTrackingBranchFn: func(remoteName, branchName string) error {
					gotFetchRemote = remoteName
					gotFetchBranch = branchName
					return tt.fetchErr
				},
				createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
					gotBranch = newBranchName
					gotPath = worktreeAbsPath
					gotBaseRef = baseRef
					return nil
				},
			}

			var stdout bytes.Buffer
			ctx := &createContext{
				cfg:               defaultTestConfig(),
				fromRemotePrimary: true,
				gitClient:         gitMock,
			}

			err := executeCreate(&stdout, ctx, "add logging support")
			if tt.wantErrContain != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContain)
				assert.Empty(t, gotBaseRef)
				assert.Empty(t, gotBranch)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "feature/add-logging-support", gotBranch)
				wantOutputPath := filepath.Join(workspaceDir, expectedWorktreeName)
				assert.Equal(t, wantOutputPath, gotPath)
				assert.Equal(t, tt.wantBaseRef, gotBaseRef)
				assert.Equal(t, wantOutputPath+"\n", stdout.String())
			}

			assert.Equal(t, tt.wantDefaultBranchRemote, gotDefaultBranchRemote)
			assert.Equal(t, tt.wantFetchRemote, gotFetchRemote)
			assert.Equal(t, tt.wantFetchBranch, gotFetchBranch)
		})
	}
}

func TestCreateCommandHasFromRemotePrimaryFlag(t *testing.T) {
	assert.NotNil(t, createCmd.Flags().Lookup("from-remote-primary"))
}

func TestExecuteCreate_ReuseVerifiesGitArgs(t *testing.T) {
	workspaceDir := t.TempDir()

	var gotBranch, gotPath string
	gitMock := &mockGit{
		getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
		branchExistsFn: func(_ string, _ bool) (bool, error) {
			return true, nil
		},
		listWorktreesFn: func() ([]git.Worktree, error) {
			return nil, nil
		},
		createWorktreeForExistingBranchFn: func(branchName, worktreeAbsPath string) error {
			gotBranch = branchName
			gotPath = worktreeAbsPath
			return nil
		},
	}

	var stdout bytes.Buffer
	ctx := &createContext{
		cfg:       defaultTestConfig(),
		gitClient: gitMock,
		reuse:     true,
	}

	err := executeCreate(&stdout, ctx, "add logging support")
	require.NoError(t, err)

	assert.Equal(t, "feature/add-logging-support", gotBranch)
	assert.Equal(t, filepath.Join(workspaceDir, "wt-add-logging-support"), gotPath)
}

func TestExecuteCreate_VerifiesGitArgs(t *testing.T) {
	tests := []struct {
		name        string
		baseRef     string
		wantBaseRef string
	}{
		{
			name:        "default HEAD when no --from",
			baseRef:     "",
			wantBaseRef: "",
		},
		{
			name:        "--from with branch name",
			baseRef:     "main",
			wantBaseRef: "main",
		},
		{
			name:        "--from with remote tracking branch",
			baseRef:     "origin/develop",
			wantBaseRef: "origin/develop",
		},
		{
			name:        "--from with tag",
			baseRef:     "v1.2.0",
			wantBaseRef: "v1.2.0",
		},
		{
			name:        "--from with commit SHA",
			baseRef:     "abc1234",
			wantBaseRef: "abc1234",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaceDir := t.TempDir()

			var gotBranch, gotPath, gotBaseRef string
			gitMock := &mockGit{
				getWorkspacePathFn: func() (string, error) { return workspaceDir, nil },
				createWorktreeForNewBranchFromRefFn: func(newBranchName, worktreeAbsPath, baseRef string) error {
					gotBranch = newBranchName
					gotPath = worktreeAbsPath
					gotBaseRef = baseRef
					return nil
				},
			}

			var stdout bytes.Buffer
			ctx := &createContext{
				baseRef:   tt.baseRef,
				cfg:       defaultTestConfig(),
				gitClient: gitMock,
			}

			err := executeCreate(&stdout, ctx, "add logging support")
			require.NoError(t, err)

			assert.Equal(t, "feature/add-logging-support", gotBranch)
			assert.Equal(t, filepath.Join(workspaceDir, "wt-add-logging-support"), gotPath)
			assert.Equal(t, tt.wantBaseRef, gotBaseRef)
		})
	}
}
