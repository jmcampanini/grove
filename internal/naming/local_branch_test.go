package naming

import (
	"testing"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLocalBranchNamer_GenerateBranchName(t *testing.T) {
	tests := []struct {
		name       string
		branchCfg  config.LocalBranchConfig
		slugifyCfg config.SlugifyConfig
		phrase     string
		want       string
	}{
		{
			name:       "simple phrase with feature prefix",
			branchCfg:  config.LocalBranchConfig{BranchPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "add user auth",
			want:       "feature/add-user-auth",
		},
		{
			name:       "phrase with fix prefix",
			branchCfg:  config.LocalBranchConfig{BranchPrefix: "fix/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "login bug",
			want:       "fix/login-bug",
		},
		{
			name:       "empty prefix",
			branchCfg:  config.LocalBranchConfig{BranchPrefix: ""},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "my feature",
			want:       "my-feature",
		},
		{
			name:       "phrase with special characters",
			branchCfg:  config.LocalBranchConfig{BranchPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "add @auth & login!",
			want:       "feature/add-auth-login",
		},
		{
			name:       "phrase with numbers",
			branchCfg:  config.LocalBranchConfig{BranchPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "fix issue 123",
			want:       "feature/fix-issue-123",
		},
		{
			name:       "phrase with uppercase",
			branchCfg:  config.LocalBranchConfig{BranchPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "Add User AUTH",
			want:       "feature/add-user-auth",
		},
		{
			name:       "empty phrase returns empty",
			branchCfg:  config.LocalBranchConfig{BranchPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "",
			want:       "",
		},
		{
			name:       "phrase with only special chars returns empty",
			branchCfg:  config.LocalBranchConfig{BranchPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "@#$%^&*()",
			want:       "",
		},
		{
			name:      "long phrase gets truncated with hash",
			branchCfg: config.LocalBranchConfig{BranchPrefix: "feature/"},
			slugifyCfg: config.SlugifyConfig{
				CollapseDashes:     true,
				HashLength:         4,
				Lowercase:          true,
				MaxLength:          20,
				ReplaceNonAlphanum: true,
				TrimDashes:         true,
			},
			phrase: "this is a very long feature description that should be truncated",
			want:   "feature/this-is-a-very-xolx", // prefix + truncated slug with hash
		},
		{
			name:       "phrase with unicode",
			branchCfg:  config.LocalBranchConfig{BranchPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "add emoji support",
			want:       "feature/add-emoji-support",
		},
		{
			name:       "phrase with dashes already",
			branchCfg:  config.LocalBranchConfig{BranchPrefix: "feature/"},
			slugifyCfg: defaultSlugifyConfig(),
			phrase:     "my-feature-name",
			want:       "feature/my-feature-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := NewLocalBranchNamer(tt.branchCfg, tt.slugifyCfg)
			got := namer.GenerateBranchName(tt.phrase)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLocalBranchNamer_GenerateWorktreeName(t *testing.T) {
	tests := []struct {
		name        string
		worktreeCfg config.LocalBranchConfig
		slugifyCfg  config.SlugifyConfig
		branchName  string
		want        string
	}{
		{
			name: "strip feature prefix",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/add-user-auth",
			want:       "wt-add-user-auth",
		},
		{
			name: "strip fix prefix",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{"feature/", "fix/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "fix/login-bug",
			want:       "wt-login-bug",
		},
		{
			name: "first matching prefix stripped",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{"fix/", "feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/add-auth",
			want:       "wt-add-auth",
		},
		{
			name: "no matching prefix",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{"feature/", "fix/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "chore/update-deps",
			want:       "wt-chore-update-deps",
		},
		{
			name: "empty strip prefix list",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/add-auth",
			want:       "wt-feature-add-auth",
		},
		{
			name: "different worktree prefix",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "work-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/my-feature",
			want:       "work-my-feature",
		},
		{
			name: "empty worktree prefix",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/add-auth",
			want:       "add-auth",
		},
		{
			name: "empty branch name returns empty",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "",
			want:       "",
		},
		{
			name: "branch name with uppercase gets lowercased",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/ADD-USER-AUTH",
			want:       "wt-add-user-auth",
		},
		{
			name: "branch name with special chars gets slugified",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/add@user#auth",
			want:       "wt-add-user-auth",
		},
		{
			name: "branch name only has prefix returns empty",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/",
			want:       "",
		},
		{
			name: "main branch without prefix",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{"feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "main",
			want:       "wt-main",
		},
		{
			name: "nested prefix pattern",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix:    "wt-",
				StripBranchPrefix: []string{"feature/jcamp/", "feature/"},
			},
			slugifyCfg: defaultSlugifyConfig(),
			branchName: "feature/jcamp/add-auth",
			want:       "wt-add-auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := NewLocalBranchNamer(tt.worktreeCfg, tt.slugifyCfg)
			got := namer.GenerateWorktreeName(tt.branchName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewLocalBranchNamer(t *testing.T) {
	cfg := config.LocalBranchConfig{
		BranchPrefix:      "feat/",
		StripBranchPrefix: []string{"a/", "b/"},
		WorktreePrefix:    "test-",
	}
	slugCfg := config.SlugifyConfig{
		CollapseDashes:     false,
		HashLength:         8,
		Lowercase:          false,
		MaxLength:          75,
		ReplaceNonAlphanum: false,
		TrimDashes:         false,
	}

	namer := NewLocalBranchNamer(cfg, slugCfg)

	assert.Equal(t, "feat/", namer.branchPrefix)
	assert.Equal(t, "test-", namer.worktreePrefix)
	assert.Equal(t, []string{"a/", "b/"}, namer.stripBranchPrefix)
	assert.Equal(t, 8, namer.slugifyOpts.HashLength)
	assert.False(t, namer.slugifyOpts.Lowercase)
	assert.Equal(t, 75, namer.slugifyOpts.MaxLength)
}

func TestLocalBranchNamer_ExtractFromAbsolutePath(t *testing.T) {
	tests := []struct {
		name        string
		worktreeCfg config.LocalBranchConfig
		absPath     string
		want        string
	}{
		{
			name: "standard prefix stripping",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix: "wt-",
			},
			absPath: "/workspace/wt-add-auth",
			want:    "add-auth",
		},
		{
			name: "no prefix match",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix: "wt-",
			},
			absPath: "/workspace/main",
			want:    "main",
		},
		{
			name: "empty prefix config",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix: "",
			},
			absPath: "/workspace/add-auth",
			want:    "add-auth",
		},
		{
			name: "empty input",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix: "wt-",
			},
			absPath: "",
			want:    ".",
		},
		{
			name: "deep nested path",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix: "wt-",
			},
			absPath: "/deep/nested/path/wt-feature",
			want:    "feature",
		},
		{
			name: "partial prefix match not at start",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix: "wt-",
			},
			absPath: "/workspace/foo-wt-bar",
			want:    "foo-wt-bar",
		},
		{
			name: "different prefix",
			worktreeCfg: config.LocalBranchConfig{
				WorktreePrefix: "work-",
			},
			absPath: "/workspace/work-feature",
			want:    "feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := NewLocalBranchNamer(tt.worktreeCfg, defaultSlugifyConfig())
			got := namer.ExtractFromAbsolutePath(tt.absPath)
			assert.Equal(t, tt.want, got)
		})
	}
}

func defaultSlugifyConfig() config.SlugifyConfig {
	return config.SlugifyConfig{
		CollapseDashes:     true,
		HashLength:         4,
		Lowercase:          true,
		MaxLength:          50,
		ReplaceNonAlphanum: true,
		TrimDashes:         true,
	}
}
