package naming

import (
	"testing"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPRWorktreeNamer(t *testing.T) {
	tests := []struct {
		name      string
		prCfg     config.PRConfig
		slugCfg   config.SlugifyConfig
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid template with BranchName",
			prCfg: config.PRConfig{
				BranchTemplate: "{{.BranchName}}",
				WorktreePrefix: "pr-",
			},
			slugCfg: defaultSlugifyConfig(),
			wantErr: false,
		},
		{
			name: "valid template with Number",
			prCfg: config.PRConfig{
				BranchTemplate: "pr/{{.Number}}",
				WorktreePrefix: "pr-",
			},
			slugCfg: defaultSlugifyConfig(),
			wantErr: false,
		},
		{
			name: "valid template with both fields",
			prCfg: config.PRConfig{
				BranchTemplate: "pr-{{.Number}}-{{.BranchName}}",
				WorktreePrefix: "pr-",
			},
			slugCfg: defaultSlugifyConfig(),
			wantErr: false,
		},
		{
			name: "invalid template syntax",
			prCfg: config.PRConfig{
				BranchTemplate: "{{.BranchName",
				WorktreePrefix: "pr-",
			},
			slugCfg:   defaultSlugifyConfig(),
			wantErr:   true,
			errSubstr: "invalid branch_template",
		},
		{
			name: "template with unknown field",
			prCfg: config.PRConfig{
				BranchTemplate: "{{.UnknownField}}",
				WorktreePrefix: "pr-",
			},
			slugCfg:   defaultSlugifyConfig(),
			wantErr:   true,
			errSubstr: "invalid field",
		},
		{
			name: "template produces leading dash",
			prCfg: config.PRConfig{
				BranchTemplate: "-{{.BranchName}}",
				WorktreePrefix: "pr-",
			},
			slugCfg:   defaultSlugifyConfig(),
			wantErr:   true,
			errSubstr: "invalid branch name",
		},
		{
			name: "template produces double dots",
			prCfg: config.PRConfig{
				BranchTemplate: "{{.BranchName}}..test",
				WorktreePrefix: "pr-",
			},
			slugCfg:   defaultSlugifyConfig(),
			wantErr:   true,
			errSubstr: "invalid branch name",
		},
		{
			name: "empty template produces empty output",
			prCfg: config.PRConfig{
				BranchTemplate: "",
				WorktreePrefix: "pr-",
			},
			slugCfg:   defaultSlugifyConfig(),
			wantErr:   true,
			errSubstr: "invalid branch name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer, err := NewPRWorktreeNamer(tt.prCfg, tt.slugCfg)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errSubstr)
				assert.Nil(t, namer)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, namer)
			}
		})
	}
}

func TestPRWorktreeNamer_GenerateBranchName(t *testing.T) {
	tests := []struct {
		name           string
		branchTemplate string
		prData         PRTemplateData
		want           string
	}{
		{
			name:           "simple BranchName template",
			branchTemplate: "{{.BranchName}}",
			prData:         PRTemplateData{BranchName: "feature/add-auth", Number: 123},
			want:           "feature/add-auth",
		},
		{
			name:           "simple Number template",
			branchTemplate: "pr/{{.Number}}",
			prData:         PRTemplateData{BranchName: "feature/add-auth", Number: 123},
			want:           "pr/123",
		},
		{
			name:           "combined template",
			branchTemplate: "pr-{{.Number}}-{{.BranchName}}",
			prData:         PRTemplateData{BranchName: "feature/add-auth", Number: 456},
			want:           "pr-456-feature/add-auth",
		},
		{
			name:           "static prefix template",
			branchTemplate: "review/{{.BranchName}}",
			prData:         PRTemplateData{BranchName: "fix/bug", Number: 789},
			want:           "review/fix/bug",
		},
		{
			name:           "zero PR number",
			branchTemplate: "pr/{{.Number}}",
			prData:         PRTemplateData{BranchName: "main", Number: 0},
			want:           "pr/0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prCfg := config.PRConfig{
				BranchTemplate: tt.branchTemplate,
				WorktreePrefix: "pr-",
			}
			namer, err := NewPRWorktreeNamer(prCfg, defaultSlugifyConfig())
			require.NoError(t, err)

			got, err := namer.GenerateBranchName(tt.prData)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPRWorktreeNamer_GenerateWorktreeName(t *testing.T) {
	tests := []struct {
		name           string
		worktreePrefix string
		branchName     string
		want           string
	}{
		{
			name:           "standard branch with prefix",
			worktreePrefix: "pr-",
			branchName:     "feature/add-auth",
			want:           "pr-feature-add-auth",
		},
		{
			name:           "branch already starts with prefix",
			worktreePrefix: "pr-",
			branchName:     "pr/123",
			want:           "pr-123",
		},
		{
			name:           "branch slugifies to start with prefix",
			worktreePrefix: "pr-",
			branchName:     "pr-fix/bug",
			want:           "pr-fix-bug",
		},
		{
			name:           "simple branch name",
			worktreePrefix: "pr-",
			branchName:     "main",
			want:           "pr-main",
		},
		{
			name:           "empty branch name",
			worktreePrefix: "pr-",
			branchName:     "",
			want:           "",
		},
		{
			name:           "different prefix",
			worktreePrefix: "review-",
			branchName:     "feature/test",
			want:           "review-feature-test",
		},
		{
			name:           "branch with special characters",
			worktreePrefix: "pr-",
			branchName:     "feature/add@user#auth",
			want:           "pr-feature-add-user-auth",
		},
		{
			name:           "uppercase branch gets lowercased",
			worktreePrefix: "pr-",
			branchName:     "Feature/ADD-AUTH",
			want:           "pr-feature-add-auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prCfg := config.PRConfig{
				BranchTemplate: "{{.BranchName}}",
				WorktreePrefix: tt.worktreePrefix,
			}
			namer, err := NewPRWorktreeNamer(prCfg, defaultSlugifyConfig())
			require.NoError(t, err)

			got := namer.GenerateWorktreeName(tt.branchName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsValidBranchName(t *testing.T) {
	tests := []struct {
		name       string
		branchName string
		want       bool
		wantReason string
	}{
		{
			name:       "valid simple name",
			branchName: "main",
			want:       true,
		},
		{
			name:       "valid with slashes",
			branchName: "feature/add-auth",
			want:       true,
		},
		{
			name:       "valid with numbers",
			branchName: "pr/123",
			want:       true,
		},
		{
			name:       "valid with dashes",
			branchName: "feature-add-auth",
			want:       true,
		},
		{
			name:       "valid with underscores",
			branchName: "feature_add_auth",
			want:       true,
		},
		{
			name:       "invalid empty string",
			branchName: "",
			want:       false,
			wantReason: "empty",
		},
		{
			name:       "invalid leading dash",
			branchName: "-feature",
			want:       false,
			wantReason: "starts with '-'",
		},
		{
			name:       "invalid double dots",
			branchName: "feature..test",
			want:       false,
			wantReason: "contains '..'",
		},
		{
			name:       "invalid double dots at start",
			branchName: "..feature",
			want:       false,
			wantReason: "contains '..'",
		},
		{
			name:       "invalid double dots at end",
			branchName: "feature..",
			want:       false,
			wantReason: "contains '..'",
		},
		{
			name:       "invalid control character tab",
			branchName: "feature\ttest",
			want:       false,
			wantReason: "contains control character",
		},
		{
			name:       "invalid control character newline",
			branchName: "feature\ntest",
			want:       false,
			wantReason: "contains control character",
		},
		{
			name:       "invalid DEL character",
			branchName: "feature\x7ftest",
			want:       false,
			wantReason: "contains control character",
		},
		{
			name:       "valid single dot",
			branchName: "feature.test",
			want:       true,
		},
		{
			name:       "valid dash in middle",
			branchName: "feature-test",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := isValidBranchName(tt.branchName)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}

func TestPRWorktreeNamer_SmartPrefixDetection(t *testing.T) {
	// Specifically test the smart prefix detection feature
	tests := []struct {
		name           string
		branchTemplate string
		worktreePrefix string
		prData         PRTemplateData
		wantWorktree   string
	}{
		{
			name:           "template produces prefix pattern - skip duplicate",
			branchTemplate: "pr/{{.Number}}",
			worktreePrefix: "pr-",
			prData:         PRTemplateData{BranchName: "feature/add-auth", Number: 123},
			wantWorktree:   "pr-123",
		},
		{
			name:           "template does not produce prefix - add prefix",
			branchTemplate: "{{.BranchName}}",
			worktreePrefix: "pr-",
			prData:         PRTemplateData{BranchName: "feature/add-auth", Number: 123},
			wantWorktree:   "pr-feature-add-auth",
		},
		{
			name:           "branch already has prefix - skip duplicate",
			branchTemplate: "{{.BranchName}}",
			worktreePrefix: "pr-",
			prData:         PRTemplateData{BranchName: "pr-fix/bug", Number: 456},
			wantWorktree:   "pr-fix-bug",
		},
		{
			name:           "different prefix pattern",
			branchTemplate: "review/{{.Number}}",
			worktreePrefix: "review-",
			prData:         PRTemplateData{BranchName: "feature", Number: 789},
			wantWorktree:   "review-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prCfg := config.PRConfig{
				BranchTemplate: tt.branchTemplate,
				WorktreePrefix: tt.worktreePrefix,
			}
			namer, err := NewPRWorktreeNamer(prCfg, defaultSlugifyConfig())
			require.NoError(t, err)

			branchName, err := namer.GenerateBranchName(tt.prData)
			require.NoError(t, err)

			worktreeName := namer.GenerateWorktreeName(branchName)
			assert.Equal(t, tt.wantWorktree, worktreeName)
		})
	}
}
