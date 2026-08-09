package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 5*time.Second, cfg.Git.Timeout)
	assert.Equal(t, 5*time.Minute, cfg.GitHub.PreviewCacheTTL)
	assert.Equal(t, IssueConfig{
		BranchTemplate:   "issue/{{.Number}}-{{.TitleSlug}}",
		WorktreeTemplate: "is-{{.Number}}-{{.TitleSlug}}",
	}, cfg.Issue)
	assert.Equal(t, LocalBranchConfig{
		BranchTemplate:   "feature/{{.PhraseSlug}}",
		WorktreeTemplate: "wt-{{.BranchSlug}}",
	}, cfg.LocalBranch)
	assert.Equal(t, NamingConfig{
		Lowercase:     true,
		MaxLength:     30,
		StripPrefixes: []string{"feature/", "fix/", "issue/"},
	}, cfg.Naming)
	assert.Equal(t, PullRequestConfig{
		BranchTemplate:   "{{.Branch}}",
		WorktreeTemplate: "pr-{{.Number}}-{{.TitleSlug}}",
	}, cfg.PullRequest)
	assert.Equal(t, []string{"main", "develop", "master"}, cfg.Workspace.PrimaryBranches)
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr string
	}{
		{name: "valid defaults", modify: func(*Config) {}},
		{
			name: "zero git timeout",
			modify: func(cfg *Config) {
				cfg.Git.Timeout = 0
			},
		},
		{
			name: "negative git timeout",
			modify: func(cfg *Config) {
				cfg.Git.Timeout = -time.Second
			},
			wantErr: "git.timeout cannot be negative",
		},
		{
			name: "negative GitHub preview cache TTL",
			modify: func(cfg *Config) {
				cfg.GitHub.PreviewCacheTTL = -time.Second
			},
			wantErr: "github.preview_cache_ttl cannot be negative",
		},
		{
			name: "empty issue branch template",
			modify: func(cfg *Config) {
				cfg.Issue.BranchTemplate = ""
			},
			wantErr: "issue.branch_template cannot be empty",
		},
		{
			name: "empty issue worktree template",
			modify: func(cfg *Config) {
				cfg.Issue.WorktreeTemplate = ""
			},
			wantErr: "issue.worktree_template cannot be empty",
		},
		{
			name: "empty local branch template",
			modify: func(cfg *Config) {
				cfg.LocalBranch.BranchTemplate = ""
			},
			wantErr: "local_branch.branch_template cannot be empty",
		},
		{
			name: "empty local worktree template",
			modify: func(cfg *Config) {
				cfg.LocalBranch.WorktreeTemplate = ""
			},
			wantErr: "local_branch.worktree_template cannot be empty",
		},
		{
			name: "negative naming max length",
			modify: func(cfg *Config) {
				cfg.Naming.MaxLength = -1
			},
			wantErr: "naming.max_length cannot be negative",
		},
		{
			name: "zero naming max length",
			modify: func(cfg *Config) {
				cfg.Naming.MaxLength = 0
			},
		},
		{
			name: "empty pull request branch template",
			modify: func(cfg *Config) {
				cfg.PullRequest.BranchTemplate = ""
			},
			wantErr: "pull_request.branch_template cannot be empty",
		},
		{
			name: "empty pull request worktree template",
			modify: func(cfg *Config) {
				cfg.PullRequest.WorktreeTemplate = ""
			},
			wantErr: "pull_request.worktree_template cannot be empty",
		},
		{
			name: "empty workspace primary branches",
			modify: func(cfg *Config) {
				cfg.Workspace.PrimaryBranches = nil
			},
			wantErr: "workspace.primary_branches cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestConfigPaths(t *testing.T) {
	tests := []struct {
		name         string
		cwd          string
		worktreeRoot string
		gitRoot      string
		homeDir      string
		wantContains []string // paths that should be in result
		wantOrder    []string // expected order (subset, for key paths)
	}{
		{
			name:         "all paths same directory",
			cwd:          "/Users/jim/project",
			worktreeRoot: "/Users/jim/project",
			gitRoot:      "/Users/jim/project",
			homeDir:      "/Users/jim",
			wantContains: []string{
				"/Users/jim/project/grove.toml",
				"/Users/jim/grove.toml",
			},
		},
		{
			name:         "worktree in sibling directory",
			cwd:          "/Users/jim/wt-feature",
			worktreeRoot: "/Users/jim/wt-feature",
			gitRoot:      "/Users/jim/project",
			homeDir:      "/Users/jim",
			wantContains: []string{
				"/Users/jim/wt-feature/grove.toml",
				"/Users/jim/project/grove.toml",
				"/Users/jim/grove.toml",
			},
			wantOrder: []string{
				"/Users/jim/grove.toml",            // lowest priority
				"/Users/jim/project/grove.toml",    // git root
				"/Users/jim/wt-feature/grove.toml", // cwd (highest)
			},
		},
		{
			name:         "nested project structure",
			cwd:          "/Users/jim/code/org/project",
			worktreeRoot: "/Users/jim/code/org/project",
			gitRoot:      "/Users/jim/code/org/project",
			homeDir:      "/Users/jim",
			wantContains: []string{
				"/Users/jim/code/org/project/grove.toml",
				"/Users/jim/code/org/grove.toml",
				"/Users/jim/code/grove.toml",
				"/Users/jim/grove.toml",
			},
			wantOrder: []string{
				"/Users/jim/grove.toml",                  // home (lowest)
				"/Users/jim/code/grove.toml",             // ancestor
				"/Users/jim/code/org/grove.toml",         // ancestor
				"/Users/jim/code/org/project/grove.toml", // git root = cwd (highest)
			},
		},
		{
			name:         "empty worktree root",
			cwd:          "/Users/jim/project",
			worktreeRoot: "",
			gitRoot:      "/Users/jim/project",
			homeDir:      "/Users/jim",
			wantContains: []string{
				"/Users/jim/project/grove.toml",
			},
		},
		{
			name:         "empty git root",
			cwd:          "/Users/jim/project",
			worktreeRoot: "/Users/jim/project",
			gitRoot:      "",
			homeDir:      "/Users/jim",
			wantContains: []string{
				"/Users/jim/project/grove.toml",
			},
		},
		{
			name:         "cwd is workspace root (parent of gitRoot)",
			cwd:          "/Users/jim/code/org/project",
			worktreeRoot: "/Users/jim/code/org/project/main",
			gitRoot:      "/Users/jim/code/org/project/main",
			homeDir:      "/Users/jim",
			wantContains: []string{
				"/Users/jim/code/org/project/grove.toml",
				"/Users/jim/code/org/project/main/grove.toml",
			},
			wantOrder: []string{
				"/Users/jim/grove.toml",                       // home
				"/Users/jim/code/grove.toml",                  // ancestor
				"/Users/jim/code/org/grove.toml",              // ancestor
				"/Users/jim/code/org/project/main/grove.toml", // gitRoot = worktreeRoot
				"/Users/jim/code/org/project/grove.toml",      // cwd (highest)
			},
		},
		{
			name:         "cwd equals homeDir",
			cwd:          "/Users/jim",
			worktreeRoot: "/Users/jim/project/main",
			gitRoot:      "/Users/jim/project/main",
			homeDir:      "/Users/jim",
			wantOrder: []string{
				"/Users/jim/grove.toml",              // home (lowest ancestor)
				"/Users/jim/project/grove.toml",      // ancestor
				"/Users/jim/project/main/grove.toml", // gitRoot = worktreeRoot
				// cwd == homeDir, so deduped — does NOT appear again at highest
			},
		},
		{
			name:         "cwd differs from worktree root",
			cwd:          "/Users/jim/project/src/subdir",
			worktreeRoot: "/Users/jim/project",
			gitRoot:      "/Users/jim/project",
			homeDir:      "/Users/jim",
			wantContains: []string{
				"/Users/jim/project/src/subdir/grove.toml",
				"/Users/jim/project/grove.toml",
			},
			wantOrder: []string{
				"/Users/jim/grove.toml",                    // home
				"/Users/jim/project/grove.toml",            // worktree/git root
				"/Users/jim/project/src/subdir/grove.toml", // cwd (highest)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := ConfigPaths(tt.cwd, tt.worktreeRoot, tt.gitRoot, tt.homeDir)

			// Check that expected paths are present
			for _, want := range tt.wantContains {
				assert.Contains(t, paths, want, "expected path to be present")
			}

			// Check ordering if specified
			if len(tt.wantOrder) > 0 {
				var foundOrder []string
				for _, p := range paths {
					for _, expected := range tt.wantOrder {
						if p == expected {
							foundOrder = append(foundOrder, p)
						}
					}
				}
				assert.Equal(t, tt.wantOrder, foundOrder, "paths should be in priority order (lowest to highest)")
			}

			// Check no duplicates
			seen := make(map[string]bool)
			for _, p := range paths {
				assert.False(t, seen[p], "duplicate path: %s", p)
				seen[p] = true
			}
		})
	}
}

func TestBootstrapConfigPaths(t *testing.T) {
	tests := []struct {
		name      string
		cwd       string
		homeDir   string
		xdgConfig string
		want      []string
	}{
		{
			name:    "XDG, ancestors, and CWD included",
			cwd:     "/Users/jim/code/org/project",
			homeDir: "/Users/jim",
			want: []string{
				"/Users/jim/.config/grove/grove.toml",
				"/Users/jim/grove.toml",
				"/Users/jim/code/grove.toml",
				"/Users/jim/code/org/grove.toml",
				"/Users/jim/code/org/project/grove.toml",
			},
		},
		{
			name:      "custom XDG_CONFIG_HOME with ancestors",
			cwd:       "/Users/jim/project",
			homeDir:   "/Users/jim",
			xdgConfig: "/custom/xdg",
			want: []string{
				"/custom/xdg/grove/grove.toml",
				"/Users/jim/grove.toml",
				"/Users/jim/project/grove.toml",
			},
		},
		{
			name:    "empty homeDir",
			cwd:     "/Users/jim/project",
			homeDir: "",
			want: []string{
				"/Users/jim/project/grove.toml",
			},
		},
		{
			name:    "empty CWD",
			cwd:     "",
			homeDir: "/Users/jim",
			want: []string{
				"/Users/jim/.config/grove/grove.toml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", tt.xdgConfig)

			paths := BootstrapConfigPaths(tt.cwd, tt.homeDir)
			assert.Equal(t, tt.want, paths)
		})
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, report, err := LoadFiles([]string{"/nonexistent/grove.toml"})
	require.NoError(t, err)
	assert.Equal(t, DefaultConfig(), cfg)
	assert.Empty(t, report.LoadedFiles)
}

func TestLoad_SingleFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(*testing.T, Config)
	}{
		{
			name: "git timeout",
			content: `[git]
timeout = "10s"
`,
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, 10*time.Second, cfg.Git.Timeout)
			},
		},
		{
			name: "GitHub preview cache TTL",
			content: `[github]
preview_cache_ttl = "10m"
`,
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, 10*time.Minute, cfg.GitHub.PreviewCacheTTL)
			},
		},
		{
			name: "zero GitHub preview cache TTL",
			content: `[github]
preview_cache_ttl = "0s"
`,
			check: func(t *testing.T, cfg Config) {
				assert.Zero(t, cfg.GitHub.PreviewCacheTTL)
			},
		},
		{
			name: "issue and pull request templates",
			content: `[issue]
branch_template = "ticket/{{.Number}}-{{.TitleSlug}}"
worktree_template = "task-{{.Number}}-{{.BranchSlug}}"

[pull_request]
branch_template = "review/{{.Number}}/{{.Branch}}"
worktree_template = "review-{{.Number}}-{{.TitleSlug}}"
`,
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, "ticket/{{.Number}}-{{.TitleSlug}}", cfg.Issue.BranchTemplate)
				assert.Equal(t, "task-{{.Number}}-{{.BranchSlug}}", cfg.Issue.WorktreeTemplate)
				assert.Equal(t, "review/{{.Number}}/{{.Branch}}", cfg.PullRequest.BranchTemplate)
				assert.Equal(t, "review-{{.Number}}-{{.TitleSlug}}", cfg.PullRequest.WorktreeTemplate)
			},
		},
		{
			name: "workspace primary branches",
			content: `[workspace]
primary_branches = ["trunk", "main"]
`,
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, []string{"trunk", "main"}, cfg.Workspace.PrimaryBranches)
			},
		},
		{
			name:    "empty file",
			content: "",
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, DefaultConfig(), cfg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "grove.toml")
			require.NoError(t, os.WriteFile(configPath, []byte(tt.content), 0644))

			cfg, report, err := LoadFiles([]string{configPath})
			require.NoError(t, err)
			tt.check(t, cfg)
			assert.Equal(t, []string{configPath}, report.LoadedFiles)
		})
	}
}

func TestLoad_NamingTOML(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "grove.toml")
	content := `[naming]
lowercase = false
max_length = 42
strip_prefixes = ["topic/", "bug/"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))

	cfg, report, err := LoadFiles([]string{configPath})
	require.NoError(t, err)
	assert.Equal(t, NamingConfig{
		Lowercase:     false,
		MaxLength:     42,
		StripPrefixes: []string{"topic/", "bug/"},
	}, cfg.Naming)
	assert.Equal(t, []string{configPath}, report.LoadedFiles)
}

func TestLoad_SequentialOverlayAndProvenance(t *testing.T) {
	tmpDir := t.TempDir()
	lowPath := filepath.Join(tmpDir, "low.toml")
	highPath := filepath.Join(tmpDir, "high.toml")
	require.NoError(t, os.WriteFile(lowPath, []byte(`[naming]
max_length = 80
strip_prefixes = ["low/"]

[local_branch]
branch_template = "low/{{.PhraseSlug}}"
`), 0644))
	require.NoError(t, os.WriteFile(highPath, []byte(`[naming]
strip_prefixes = ["high/"]

[local_branch]
branch_template = "high/{{.PhraseSlug}}"
`), 0644))

	cfg, report, err := LoadFiles([]string{lowPath, highPath})
	require.NoError(t, err)
	assert.Equal(t, 80, cfg.Naming.MaxLength)
	assert.Equal(t, []string{"high/"}, cfg.Naming.StripPrefixes)
	assert.Equal(t, "high/{{.PhraseSlug}}", cfg.LocalBranch.BranchTemplate)
	assert.Equal(t, []string{lowPath, highPath}, report.LoadedFiles)
	assert.Equal(t, lowPath, report.Updates["naming.maxlength"])
	assert.Equal(t, highPath, report.Updates["naming.stripprefixes"])
	assert.Equal(t, highPath, report.Updates["localbranch.branchtemplate"])
	assert.Equal(t, configloader.SourceDefault, report.Updates["naming.lowercase"])
}

func TestLoad_ZeroValueOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	firstPath := filepath.Join(tmpDir, "first.toml")
	secondPath := filepath.Join(tmpDir, "second.toml")
	require.NoError(t, os.WriteFile(firstPath, []byte(`[naming]
lowercase = true
max_length = 100
strip_prefixes = ["topic/"]
`), 0644))
	require.NoError(t, os.WriteFile(secondPath, []byte(`[naming]
lowercase = false
max_length = 0
strip_prefixes = []
`), 0644))

	cfg, _, err := LoadFiles([]string{firstPath, secondPath})
	require.NoError(t, err)
	assert.False(t, cfg.Naming.Lowercase)
	assert.Zero(t, cfg.Naming.MaxLength)
	assert.Empty(t, cfg.Naming.StripPrefixes)
}

func TestLoad_InvalidTOML(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "grove.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[naming\nmax_length = 10"), 0644))

	_, _, err := LoadFiles([]string{configPath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), configPath)
}

func TestLoad_UnknownKeys(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "grove.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[git]\nunknown = true\n"), 0644))

	_, _, err := LoadFiles([]string{configPath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown keys")
	assert.Contains(t, err.Error(), "git.unknown")
}

func TestLoad_InvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "negative git timeout",
			content: `[git]
timeout = "-5s"
`,
			wantErr: "git.timeout cannot be negative",
		},
		{
			name: "negative GitHub preview cache TTL",
			content: `[github]
preview_cache_ttl = "-1s"
`,
			wantErr: "github.preview_cache_ttl cannot be negative",
		},
		{
			name: "negative naming max length",
			content: `[naming]
max_length = -1
`,
			wantErr: "naming.max_length cannot be negative",
		},
		{
			name: "explicit empty file template",
			content: `[issue]
branch_template = ""
`,
			wantErr: "issue.branch_template cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "grove.toml")
			require.NoError(t, os.WriteFile(configPath, []byte(tt.content), 0644))
			_, _, err := LoadFiles([]string{configPath})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoad_ReturnsLoadedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "one.toml")
	path2 := filepath.Join(tmpDir, "two.toml")
	missingPath := filepath.Join(tmpDir, "missing.toml")
	require.NoError(t, os.WriteFile(path1, []byte("[naming]\nmax_length = 40\n"), 0644))
	require.NoError(t, os.WriteFile(path2, []byte("[naming]\nmax_length = 50\n"), 0644))

	_, report, err := LoadFiles([]string{path1, missingPath, path2})
	require.NoError(t, err)
	assert.Equal(t, []string{path1, path2}, report.LoadedFiles)
}

func TestLoad_PathIsDirectory(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "grove.toml")
	require.NoError(t, os.Mkdir(dirPath, 0755))

	cfg, report, err := LoadFiles([]string{dirPath})
	require.NoError(t, err)
	assert.Equal(t, DefaultConfig(), cfg)
	assert.Empty(t, report.LoadedFiles)
}

func TestLoadFilesWithFlags_Precedence(t *testing.T) {
	tests := []struct {
		name       string
		fileValue  *string
		args       []string
		want       string
		wantSource any
	}{
		{
			name:       "default used without file or flag",
			want:       "wt-{{.BranchSlug}}",
			wantSource: configloader.SourceDefault,
		},
		{
			name:      "file overrides default",
			fileValue: stringPtr("tree-{{.BranchSlug}}"),
			want:      "tree-{{.BranchSlug}}",
		},
		{
			name:      "unset flag keeps file value",
			fileValue: stringPtr("tree-{{.BranchSlug}}"),
			args:      []string{},
			want:      "tree-{{.BranchSlug}}",
		},
		{
			name:       "flag overrides file",
			fileValue:  stringPtr("tree-{{.BranchSlug}}"),
			args:       []string{"--worktree-template", "flag-{{.BranchSlug}}"},
			want:       "flag-{{.BranchSlug}}",
			wantSource: pflagloader.SourcePFlag,
		},
		{
			name:       "flag overrides default",
			args:       []string{"--worktree-template", "flag-{{.BranchSlug}}"},
			want:       "flag-{{.BranchSlug}}",
			wantSource: pflagloader.SourcePFlag,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var paths []string
			if tt.fileValue != nil {
				configPath := filepath.Join(t.TempDir(), "grove.toml")
				content := "[local_branch]\nworktree_template = \"" + *tt.fileValue + "\"\n"
				require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))
				paths = []string{configPath}
			}

			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			require.NoError(t, RegisterFlags(flags))
			require.NoError(t, flags.Parse(tt.args))
			cfg, report, err := LoadFilesWithFlags(paths, flags)
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.LocalBranch.WorktreeTemplate)
			if tt.wantSource != nil {
				assert.Equal(t, tt.wantSource, report.Updates["localbranch.worktreetemplate"])
			}
		})
	}
}

func TestLoadFilesWithFlags_ExplicitEmptyIsInvalid(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	require.NoError(t, RegisterFlags(flags))
	require.NoError(t, flags.Parse([]string{"--worktree-template", ""}))

	_, _, err := LoadFilesWithFlags(nil, flags)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local_branch.worktree_template cannot be empty")
}

func TestLoadFilesWithFlags_NilFlagSetMatchesLoadFiles(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "grove.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("[local_branch]\nworktree_template = \"tree-{{.BranchSlug}}\"\n"), 0644))

	fromFiles, _, err := LoadFiles([]string{configPath})
	require.NoError(t, err)
	fromNilFlags, _, err := LoadFilesWithFlags([]string{configPath}, nil)
	require.NoError(t, err)
	assert.Equal(t, fromFiles, fromNilFlags)
}

func stringPtr(value string) *string {
	return &value
}
