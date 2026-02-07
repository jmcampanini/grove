package naming

import (
	"testing"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLocalBranchNamer_Generate(t *testing.T) {
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
			got := namer.Generate(tt.branchName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewLocalBranchNamer(t *testing.T) {
	worktreeCfg := config.LocalBranchConfig{
		WorktreePrefix:    "test-",
		StripBranchPrefix: []string{"a/", "b/"},
	}
	slugCfg := config.SlugifyConfig{
		CollapseDashes:     false,
		HashLength:         8,
		Lowercase:          false,
		MaxLength:          75,
		ReplaceNonAlphanum: false,
		TrimDashes:         false,
	}

	namer := NewLocalBranchNamer(worktreeCfg, slugCfg)

	assert.Equal(t, "test-", namer.prefix)
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
