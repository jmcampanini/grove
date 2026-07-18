package naming

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultIssueConfig() config.IssueConfig {
	return config.IssueConfig{
		BranchTemplate:     "issue/{{.Number}}-{{.TitleSlug}}",
		StripBranchPrefix:  []string{"issue/"},
		TitleSlugMaxLength: 40,
		WorktreePrefix:     "is-",
	}
}

func newIssueNamer(t *testing.T, cfg config.IssueConfig) *IssueNamer {
	t.Helper()
	namer, err := NewIssueNamer(cfg, defaultSlugifyConfig())
	require.NoError(t, err)
	return namer
}

func TestNewIssueNamer_InvalidTemplates(t *testing.T) {
	tests := []struct {
		name           string
		branchTemplate string
		wantErrContain string
	}{
		{
			name:           "syntax error",
			branchTemplate: "issue/{{.Number",
			wantErrContain: "invalid branch_template",
		},
		{
			name:           "unknown field",
			branchTemplate: "issue/{{.Bogus}}",
			wantErrContain: "branch_template uses invalid field",
		},
		{
			name:           "produces branch starting with dash",
			branchTemplate: "-{{.Number}}",
			wantErrContain: "branch_template produces invalid branch name",
		},
		{
			name:           "produces empty branch",
			branchTemplate: "",
			wantErrContain: "branch_template produces invalid branch name",
		},
		{
			name:           "missing number field",
			branchTemplate: "issue/{{.TitleSlug}}",
			wantErrContain: "branch_template must reference {{.Number}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultIssueConfig()
			cfg.BranchTemplate = tt.branchTemplate
			_, err := NewIssueNamer(cfg, defaultSlugifyConfig())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrContain)
		})
	}
}

func TestIssueNamer_TitleSlug(t *testing.T) {
	tests := []struct {
		name      string
		maxLength int
		title     string
		want      string
	}{
		{
			name:      "simple title",
			maxLength: 40,
			title:     "Fix login crash",
			want:      "fix-login-crash",
		},
		{
			name:      "punctuation collapses",
			maxLength: 40,
			title:     "Fix: login!! crash (regression)",
			want:      "fix-login-crash-regression",
		},
		{
			name:      "truncated at cap without hash suffix",
			maxLength: 20,
			title:     "Fix login crash when the password field is empty",
			want:      "fix-login-crash-when",
		},
		{
			name:      "trailing dash trimmed after truncation",
			maxLength: 16,
			title:     "Fix login crash when the password field is empty",
			want:      "fix-login-crash",
		},
		{
			name:      "zero cap disables truncation",
			maxLength: 0,
			title:     "Fix login crash when the password field is empty",
			want:      "fix-login-crash-when-the-password-field-is-empty",
		},
		{
			name:      "empty title",
			maxLength: 40,
			title:     "",
			want:      "",
		},
		{
			name:      "punctuation-only title",
			maxLength: 40,
			title:     "!!!",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultIssueConfig()
			cfg.TitleSlugMaxLength = tt.maxLength
			namer := newIssueNamer(t, cfg)
			assert.Equal(t, tt.want, namer.TitleSlug(tt.title))
		})
	}
}

func TestIssueNamer_TitleSlug_MultibyteTruncation(t *testing.T) {
	// With replace_non_alphanum disabled, multibyte characters survive
	// slugification; truncation must count runes and never split one.
	slugCfg := defaultSlugifyConfig()
	slugCfg.ReplaceNonAlphanum = false

	cfg := defaultIssueConfig()
	cfg.TitleSlugMaxLength = 40
	namer, err := NewIssueNamer(cfg, slugCfg)
	require.NoError(t, err)

	got := namer.TitleSlug(strings.Repeat("é", 45))
	assert.True(t, utf8.ValidString(got), "truncation split a rune: %q", got)
	assert.Equal(t, strings.Repeat("é", 40), got)
}

func TestIssueNamer_GenerateBranchName(t *testing.T) {
	tests := []struct {
		name           string
		branchTemplate string
		number         int
		title          string
		want           string
	}{
		{
			name:           "default template",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			number:         123,
			title:          "Fix login crash",
			want:           "issue/123-fix-login-crash",
		},
		{
			name:           "number-only template",
			branchTemplate: "issue/{{.Number}}",
			number:         123,
			title:          "Fix login crash",
			want:           "issue/123",
		},
		{
			name:           "empty title leaves trailing dash",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			number:         123,
			title:          "!!!",
			want:           "issue/123-",
		},
		{
			name:           "long title capped at 40",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			number:         7,
			title:          "Fix login crash when the password field is empty (regression!!)",
			want:           "issue/7-fix-login-crash-when-the-password-field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultIssueConfig()
			cfg.BranchTemplate = tt.branchTemplate
			namer := newIssueNamer(t, cfg)
			got, err := namer.GenerateBranchName(tt.number, tt.title)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssueNamer_GenerateWorktreeName(t *testing.T) {
	tests := []struct {
		name              string
		branchName        string
		stripBranchPrefix []string
		worktreePrefix    string
		want              string
	}{
		{
			name:              "default issue namespace is replaced",
			branchName:        "issue/123-fix-login",
			stripBranchPrefix: []string{"issue/"},
			worktreePrefix:    "is-",
			want:              "is-123-fix-login",
		},
		{
			name:              "first matching branch prefix is stripped",
			branchName:        "ticket/123-fix-login",
			stripBranchPrefix: []string{"issue/", "ticket/"},
			worktreePrefix:    "is-",
			want:              "is-123-fix-login",
		},
		{
			name:              "earlier overlapping branch prefix wins",
			branchName:        "issue/123-fix-login",
			stripBranchPrefix: []string{"issue/", "issue/123-"},
			worktreePrefix:    "is-",
			want:              "is-123-fix-login",
		},
		{
			name:              "reversing overlapping prefixes changes the result",
			branchName:        "issue/123-fix-login",
			stripBranchPrefix: []string{"issue/123-", "issue/"},
			worktreePrefix:    "is-",
			want:              "is-fix-login",
		},
		{
			name:              "only one branch prefix is stripped",
			branchName:        "feature/issue/123-fix-login",
			stripBranchPrefix: []string{"feature/", "issue/"},
			worktreePrefix:    "is-",
			want:              "is-issue-123-fix-login",
		},
		{
			name:              "unmatched branch prefix remains",
			branchName:        "ticket/123-fix-login",
			stripBranchPrefix: []string{"issue/"},
			worktreePrefix:    "is-",
			want:              "is-ticket-123-fix-login",
		},
		{
			name:              "empty branch prefix list strips nothing",
			branchName:        "issue/123-fix-login",
			stripBranchPrefix: nil,
			worktreePrefix:    "is-",
			want:              "is-issue-123-fix-login",
		},
		{
			name:              "stripping the entire branch returns empty",
			branchName:        "issue/",
			stripBranchPrefix: []string{"issue/"},
			worktreePrefix:    "is-",
			want:              "",
		},
		{
			name:              "smart prefix detection avoids doubling",
			branchName:        "issue/is-123-fix-login",
			stripBranchPrefix: []string{"issue/"},
			worktreePrefix:    "is-",
			want:              "is-123-fix-login",
		},
		{
			name:              "custom worktree prefix",
			branchName:        "issue/123-fix-login",
			stripBranchPrefix: []string{"issue/"},
			worktreePrefix:    "task-",
			want:              "task-123-fix-login",
		},
		{
			name:              "trailing dash branch slugs cleanly",
			branchName:        "issue/123-",
			stripBranchPrefix: []string{"issue/"},
			worktreePrefix:    "is-",
			want:              "is-123",
		},
		{
			name:              "empty branch",
			branchName:        "",
			stripBranchPrefix: []string{"issue/"},
			worktreePrefix:    "is-",
			want:              "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultIssueConfig()
			cfg.StripBranchPrefix = tt.stripBranchPrefix
			cfg.WorktreePrefix = tt.worktreePrefix
			namer := newIssueNamer(t, cfg)
			assert.Equal(t, tt.want, namer.GenerateWorktreeName(tt.branchName))
		})
	}
}

func TestIssueNamer_MatchesIssueNumber(t *testing.T) {
	tests := []struct {
		name           string
		branchTemplate string
		branchName     string
		number         int
		title          string
		want           bool
	}{
		{
			name:           "anchored match on number and boundary",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			branchName:     "issue/123-fix-login",
			number:         123,
			title:          "Fix login",
			want:           true,
		},
		{
			name:           "anchored match survives title edits",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			branchName:     "issue/123-old-title",
			number:         123,
			title:          "Completely renamed title",
			want:           true,
		},
		{
			name:           "longer number does not match",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			branchName:     "issue/1234-fix-login",
			number:         123,
			title:          "Fix login",
			want:           false,
		},
		{
			name:           "shorter number does not match",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			branchName:     "issue/12-fix-login",
			number:         123,
			title:          "Fix login",
			want:           false,
		},
		{
			name:           "bare number branch matches",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			branchName:     "issue/123",
			number:         123,
			title:          "Fix login",
			want:           true,
		},
		{
			name:           "unrelated branch does not match",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			branchName:     "feature/add-auth",
			number:         123,
			title:          "Fix login",
			want:           false,
		},
		{
			name:           "unrelated branch sharing numeric prefix does not match",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			branchName:     "issue/2fa-improvements",
			number:         2,
			title:          "Add 2FA",
			want:           false,
		},
		{
			name:           "non-separator boundary does not match",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			branchName:     "issue/123abc",
			number:         123,
			title:          "Fix login",
			want:           false,
		},
		{
			name:           "slash separator template matches its own shape",
			branchTemplate: "issue/{{.Number}}/{{.TitleSlug}}",
			branchName:     "issue/123/fix-login",
			number:         123,
			title:          "Fix login",
			want:           true,
		},
		{
			name:           "slash separator template rejects dash-separated branch",
			branchTemplate: "issue/{{.Number}}/{{.TitleSlug}}",
			branchName:     "issue/123-fix-login",
			number:         123,
			title:          "Fix login",
			want:           false,
		},
		{
			name:           "number-only template matches its own output",
			branchTemplate: "issue/{{.Number}}",
			branchName:     "issue/123",
			number:         123,
			title:          "Fix login",
			want:           true,
		},
		{
			name:           "number-only template accepts non-digit continuation",
			branchTemplate: "issue/{{.Number}}",
			branchName:     "issue/123-extra",
			number:         123,
			title:          "Fix login",
			want:           true,
		},
		{
			name:           "number-only template rejects digit continuation",
			branchTemplate: "issue/{{.Number}}",
			branchName:     "issue/1234",
			number:         123,
			title:          "Fix login",
			want:           false,
		},
		{
			name:           "title-first template falls back to exact regeneration",
			branchTemplate: "{{.TitleSlug}}-{{.Number}}",
			branchName:     "fix-login-123",
			number:         123,
			title:          "Fix login",
			want:           true,
		},
		{
			name:           "title-first template misses after title edit",
			branchTemplate: "{{.TitleSlug}}-{{.Number}}",
			branchName:     "fix-login-123",
			number:         123,
			title:          "Renamed title",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultIssueConfig()
			cfg.BranchTemplate = tt.branchTemplate
			namer := newIssueNamer(t, cfg)
			assert.Equal(t, tt.want, namer.MatchesIssueNumber(tt.branchName, tt.number, tt.title))
		})
	}
}

func TestNumberAnchorPrefix(t *testing.T) {
	tests := []struct {
		name           string
		branchTemplate string
		wantPrefix     string
		wantBoundary   string
		wantOK         bool
	}{
		{
			name:           "literal prefix before number",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			wantPrefix:     "issue/",
			wantBoundary:   "-",
			wantOK:         true,
		},
		{
			name:           "number first",
			branchTemplate: "{{.Number}}-{{.TitleSlug}}",
			wantPrefix:     "",
			wantBoundary:   "-",
			wantOK:         true,
		},
		{
			name:           "spaced action still anchors",
			branchTemplate: "issue/{{ .Number }}-{{.TitleSlug}}",
			wantPrefix:     "issue/",
			wantBoundary:   "-",
			wantOK:         true,
		},
		{
			name:           "slash separator",
			branchTemplate: "issue/{{.Number}}/{{.TitleSlug}}",
			wantPrefix:     "issue/",
			wantBoundary:   "/",
			wantOK:         true,
		},
		{
			name:           "number last has empty boundary",
			branchTemplate: "issue/{{.Number}}",
			wantPrefix:     "issue/",
			wantBoundary:   "",
			wantOK:         true,
		},
		{
			name:           "title before number disables anchor",
			branchTemplate: "{{.TitleSlug}}-{{.Number}}",
			wantOK:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := defaultIssueConfig()
			cfg.BranchTemplate = tt.branchTemplate
			namer := newIssueNamer(t, cfg)
			assert.Equal(t, tt.wantOK, namer.numberAnchorOK)
			if tt.wantOK {
				assert.Equal(t, tt.wantPrefix, namer.numberAnchor)
				assert.Equal(t, tt.wantBoundary, namer.numberBoundary)
			}
		})
	}
}
