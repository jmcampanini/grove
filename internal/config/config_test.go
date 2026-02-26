package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Git defaults
	assert.Equal(t, 5*time.Second, cfg.Git.Timeout)

	// GitHub defaults
	assert.Equal(t, 5*time.Minute, cfg.GitHub.PreviewCacheTTL)

	// Slugify defaults
	assert.True(t, cfg.Slugify.CollapseDashes)
	assert.Equal(t, 4, cfg.Slugify.HashLength)
	assert.True(t, cfg.Slugify.Lowercase)
	assert.Equal(t, 50, cfg.Slugify.MaxLength)
	assert.True(t, cfg.Slugify.ReplaceNonAlphanum)
	assert.True(t, cfg.Slugify.TrimDashes)

	// LocalBranch defaults
	assert.Equal(t, "feature/", cfg.LocalBranch.BranchPrefix)
	assert.Equal(t, []string{"feature/"}, cfg.LocalBranch.StripBranchPrefix)
	assert.Equal(t, "wt-", cfg.LocalBranch.WorktreePrefix)

	// PullRequest defaults
	assert.True(t, cfg.PullRequest.AutoRecreate)
	assert.Equal(t, "{{.BranchName}}", cfg.PullRequest.BranchTemplate)
	assert.Equal(t, "recreated-{{.Number}}-{{.BranchName}}", cfg.PullRequest.RecreatedBranchTemplate)
	assert.Equal(t, "pr-", cfg.PullRequest.WorktreePrefix)

	// Workspace defaults
	assert.Equal(t, []string{"main", "develop", "master"}, cfg.Workspace.PrimaryBranches)

	// Default config should be valid
	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr string
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: "",
		},
		// Git
		{
			name: "negative git timeout",
			modify: func(c *Config) {
				c.Git.Timeout = -1 * time.Second
			},
			wantErr: "git.timeout cannot be negative",
		},
		{
			name: "zero timeout is valid",
			modify: func(c *Config) {
				c.Git.Timeout = 0
			},
			wantErr: "",
		},
		// GitHub
		{
			name: "negative preview cache TTL",
			modify: func(c *Config) {
				c.GitHub.PreviewCacheTTL = -1 * time.Second
			},
			wantErr: "github.preview_cache_ttl cannot be negative",
		},
		{
			name: "zero preview cache TTL is valid",
			modify: func(c *Config) {
				c.GitHub.PreviewCacheTTL = 0
			},
			wantErr: "",
		},
		// PullRequest
		{
			name: "empty pull request worktree prefix",
			modify: func(c *Config) {
				c.PullRequest.WorktreePrefix = ""
			},
			wantErr: "pull_request.worktree_prefix cannot be empty",
		},
		// Slugify
		{
			name: "negative hash length",
			modify: func(c *Config) {
				c.Slugify.HashLength = -1
			},
			wantErr: "slugify.hash_length cannot be negative",
		},
		{
			name: "zero hash length is valid",
			modify: func(c *Config) {
				c.Slugify.HashLength = 0
			},
			wantErr: "",
		},
		{
			name: "negative max length",
			modify: func(c *Config) {
				c.Slugify.MaxLength = -1
			},
			wantErr: "slugify.max_length cannot be negative",
		},
		{
			name: "zero max length is valid",
			modify: func(c *Config) {
				c.Slugify.MaxLength = 0
			},
			wantErr: "",
		},
		{
			name: "hash length equals max length minus 2 is valid",
			modify: func(c *Config) {
				c.Slugify.MaxLength = 10
				c.Slugify.HashLength = 8
			},
			wantErr: "",
		},
		{
			name: "hash length greater than max length minus 2 is invalid",
			modify: func(c *Config) {
				c.Slugify.MaxLength = 10
				c.Slugify.HashLength = 9
			},
			wantErr: "slugify.hash_length must be at least 2 less than slugify.max_length",
		},
		{
			name: "hash length equals max length is invalid",
			modify: func(c *Config) {
				c.Slugify.MaxLength = 10
				c.Slugify.HashLength = 10
			},
			wantErr: "slugify.hash_length must be at least 2 less than slugify.max_length",
		},
		{
			name: "hash length greater than max length is invalid",
			modify: func(c *Config) {
				c.Slugify.MaxLength = 5
				c.Slugify.HashLength = 10
			},
			wantErr: "slugify.hash_length must be at least 2 less than slugify.max_length",
		},
		// Workspace
		{
			name: "empty primary branches",
			modify: func(c *Config) {
				c.Workspace.PrimaryBranches = []string{}
			},
			wantErr: "workspace.primary_branches cannot be empty",
		},
		{
			name: "nil primary branches",
			modify: func(c *Config) {
				c.Workspace.PrimaryBranches = nil
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
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
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

// fakeFileSystem is a test double for FileSystem
type fakeFileSystem struct {
	existingFiles map[string]bool
}

func (f *fakeFileSystem) Exists(path string) bool {
	return f.existingFiles[path]
}

func TestLoad_MissingFile(t *testing.T) {
	fs := &fakeFileSystem{existingFiles: map[string]bool{}}
	loader := NewLoader(fs)

	result, err := loader.Load([]string{"/nonexistent/grove.toml"})
	require.NoError(t, err)
	assert.Equal(t, DefaultConfig(), result.Config)
	assert.Empty(t, result.SourcePaths)
}

func TestLoad_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "grove.toml")

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
			name: "github preview cache TTL",
			content: `[github]
preview_cache_ttl = "10m"
`,
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, 10*time.Minute, cfg.GitHub.PreviewCacheTTL)
				assert.Equal(t, 5*time.Second, cfg.Git.Timeout)
			},
		},
		{
			name: "github preview cache TTL disabled",
			content: `[github]
preview_cache_ttl = "0s"
`,
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, time.Duration(0), cfg.GitHub.PreviewCacheTTL)
			},
		},
		{
			name: "local branch config",
			content: `[local_branch]
branch_prefix = "fix/"
worktree_prefix = "work-"
strip_branch_prefix = ["fix/", "feature/", "chore/"]
`,
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, "fix/", cfg.LocalBranch.BranchPrefix)
				assert.Equal(t, "work-", cfg.LocalBranch.WorktreePrefix)
				assert.Equal(t, []string{"fix/", "feature/", "chore/"}, cfg.LocalBranch.StripBranchPrefix)
			},
		},
		{
			name: "slugify options",
			content: `[slugify]
max_length = 30
hash_length = 6
lowercase = false
`,
			check: func(t *testing.T, cfg Config) {
				assert.Equal(t, 30, cfg.Slugify.MaxLength)
				assert.Equal(t, 6, cfg.Slugify.HashLength)
				assert.False(t, cfg.Slugify.Lowercase)
				assert.True(t, cfg.Slugify.CollapseDashes)
			},
		},
		{
			name: "workspace config",
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
			err := os.WriteFile(configPath, []byte(tt.content), 0644)
			require.NoError(t, err)

			loader := NewDefaultLoader()
			result, err := loader.Load([]string{configPath})
			require.NoError(t, err)

			tt.check(t, result.Config)
			assert.Equal(t, []string{configPath}, result.SourcePaths)
		})
	}
}

func TestLoad_SequentialOverlay(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two config files
	lowPriorityPath := filepath.Join(tmpDir, "low.toml")
	highPriorityPath := filepath.Join(tmpDir, "high.toml")

	lowPriorityContent := `[local_branch]
branch_prefix = "low/"

[slugify]
max_length = 100
`

	highPriorityContent := `[local_branch]
branch_prefix = "high/"
`

	require.NoError(t, os.WriteFile(lowPriorityPath, []byte(lowPriorityContent), 0644))
	require.NoError(t, os.WriteFile(highPriorityPath, []byte(highPriorityContent), 0644))

	loader := NewDefaultLoader()
	result, err := loader.Load([]string{lowPriorityPath, highPriorityPath})
	require.NoError(t, err)

	// High priority should override local_branch.branch_prefix
	assert.Equal(t, "high/", result.Config.LocalBranch.BranchPrefix)
	// Low priority should still apply for non-overridden fields
	assert.Equal(t, 100, result.Config.Slugify.MaxLength)
	// Both paths should be in source paths
	assert.Equal(t, []string{lowPriorityPath, highPriorityPath}, result.SourcePaths)
}

func TestLoad_ZeroValueOverwrite(t *testing.T) {
	tmpDir := t.TempDir()

	// First file sets a value
	firstPath := filepath.Join(tmpDir, "first.toml")
	firstContent := `[slugify]
max_length = 100
`
	require.NoError(t, os.WriteFile(firstPath, []byte(firstContent), 0644))

	// Second file explicitly sets it to 0
	secondPath := filepath.Join(tmpDir, "second.toml")
	secondContent := `[slugify]
max_length = 0
`
	require.NoError(t, os.WriteFile(secondPath, []byte(secondContent), 0644))

	loader := NewDefaultLoader()
	result, err := loader.Load([]string{firstPath, secondPath})
	require.NoError(t, err)

	// Zero value from second file should override
	assert.Equal(t, 0, result.Config.Slugify.MaxLength)
}

func TestLoad_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "grove.toml")

	invalidContent := `[local_branch
branch_prefix = "broken`
	require.NoError(t, os.WriteFile(configPath, []byte(invalidContent), 0644))

	loader := NewDefaultLoader()
	_, err := loader.Load([]string{configPath})
	require.Error(t, err)
	assert.Contains(t, err.Error(), configPath)
}

func TestLoad_InvalidConfigValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "grove.toml")

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "negative timeout",
			content: `[git]
timeout = "-5s"
`,
			wantErr: "git.timeout cannot be negative",
		},
		{
			name: "negative preview cache TTL",
			content: `[github]
preview_cache_ttl = "-1s"
`,
			wantErr: "github.preview_cache_ttl cannot be negative",
		},
		{
			name: "negative hash_length",
			content: `[slugify]
hash_length = -5
`,
			wantErr: "slugify.hash_length cannot be negative",
		},
		{
			name: "negative max_length",
			content: `[slugify]
max_length = -1
`,
			wantErr: "slugify.max_length cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.WriteFile(configPath, []byte(tt.content), 0644))

			loader := NewDefaultLoader()
			_, err := loader.Load([]string{configPath})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoad_ReturnsSourcePaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create three config files, but only two exist
	path1 := filepath.Join(tmpDir, "one.toml")
	path2 := filepath.Join(tmpDir, "two.toml")
	path3 := filepath.Join(tmpDir, "nonexistent.toml")

	require.NoError(t, os.WriteFile(path1, []byte("[local_branch]\nbranch_prefix = \"one/\""), 0644))
	require.NoError(t, os.WriteFile(path2, []byte("[local_branch]\nbranch_prefix = \"two/\""), 0644))

	loader := NewDefaultLoader()
	result, err := loader.Load([]string{path1, path3, path2})
	require.NoError(t, err)

	// Only existing files should be in source paths
	assert.Equal(t, []string{path1, path2}, result.SourcePaths)
}

func TestLoad_PathIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory where a config file might be expected
	dirPath := filepath.Join(tmpDir, "grove.toml")
	require.NoError(t, os.Mkdir(dirPath, 0755))

	loader := NewDefaultLoader()
	result, err := loader.Load([]string{dirPath})
	require.NoError(t, err)

	// Directory should be skipped
	assert.Empty(t, result.SourcePaths)
	assert.Equal(t, DefaultConfig(), result.Config)
}

func TestOSFileSystem_Exists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real file
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("test"), 0644))

	// Create a directory
	dirPath := filepath.Join(tmpDir, "testdir")
	require.NoError(t, os.Mkdir(dirPath, 0755))

	fs := OSFileSystem{}

	// File should exist
	assert.True(t, fs.Exists(filePath))

	// Directory should not count as existing file
	assert.False(t, fs.Exists(dirPath))

	// Nonexistent should not exist
	assert.False(t, fs.Exists(filepath.Join(tmpDir, "nonexistent")))
}

func TestNewDefaultLoader(t *testing.T) {
	loader := NewDefaultLoader()
	assert.NotNil(t, loader)
	assert.IsType(t, OSFileSystem{}, loader.fs)
}
