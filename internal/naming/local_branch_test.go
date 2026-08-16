package naming

import (
	"errors"
	"strings"
	"testing"
	"text/template"

	"github.com/jmcampanini/grove/internal/config"
)

func testNamingConfig() config.NamingConfig {
	return config.NamingConfig{
		Lowercase:     true,
		StripPrefixes: []string{"feature/", "fix/", "issue/"},
	}
}

func testLocalConfig() config.LocalBranchConfig {
	return config.LocalBranchConfig{
		BranchTemplate:   "feature/{{.PhraseSlug}}",
		WorktreeTemplate: "wt-{{.BranchSlug}}",
	}
}

func requireLocalNamer(t *testing.T, localCfg config.LocalBranchConfig, namingCfg config.NamingConfig) *LocalBranchNamer {
	t.Helper()
	namer, err := NewLocalBranchNamer(localCfg, namingCfg)
	if err != nil {
		t.Fatalf("NewLocalBranchNamer() error = %v", err)
	}
	return namer
}

func TestNewLocalBranchNamerValidatesTemplates(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		worktree  string
		wantError string
	}{
		{name: "valid variables", branch: "feature/{{.PhraseSlug}}", worktree: "wt-{{.BranchSlug}}"},
		{name: "worktree double dot within name", branch: "feature/{{.PhraseSlug}}", worktree: "wt..{{.BranchSlug}}"},
		{name: "builtins remain available", branch: `{{printf "feature/%s" .PhraseSlug}}`, worktree: `{{printf "wt-%s" .BranchSlug}}`},
		{name: "variables remain available", branch: `{{$slug := .PhraseSlug}}feature/{{$slug}}`, worktree: `{{$slug := .BranchSlug}}wt-{{$slug}}`},
		{name: "root variable direct fields remain available", branch: `{{with .PhraseSlug}}feature/{{$.PhraseSlug}}{{end}}`, worktree: `{{with .BranchSlug}}wt-{{$.BranchSlug}}{{end}}`},
		{name: "control structures remain available", branch: `{{if .PhraseSlug}}feature/{{with .PhraseSlug}}{{.}}{{end}}{{end}}`, worktree: `{{if .BranchSlug}}wt-{{.BranchSlug}}{{end}}`},
		{name: "fields in definitions remain available", branch: `{{define "name"}}feature/{{.PhraseSlug}}{{end}}{{template "name" .}}`, worktree: `{{define "name"}}wt-{{.BranchSlug}}{{end}}{{template "name" .}}`},
		{name: "branch parse error", branch: "{{.PhraseSlug", worktree: "wt-{{.BranchSlug}}", wantError: "branch_template"},
		{name: "branch field unavailable", branch: "{{.BranchSlug}}", worktree: "wt-{{.BranchSlug}}", wantError: "BranchSlug"},
		{name: "field unavailable in dormant if branch", branch: `{{if .PhraseSlug}}feature/{{.PhraseSlug}}{{else}}{{.BranchSlug}}{{end}}`, worktree: "wt-{{.BranchSlug}}", wantError: "BranchSlug"},
		{name: "field unavailable in dormant range", branch: `{{if .PhraseSlug}}feature/{{.PhraseSlug}}{{else}}{{range .BranchSlug}}{{.}}{{end}}{{end}}`, worktree: "wt-{{.BranchSlug}}", wantError: "BranchSlug"},
		{name: "field unavailable in dormant with", branch: `{{if .PhraseSlug}}feature/{{.PhraseSlug}}{{else}}{{with .BranchSlug}}{{.}}{{end}}{{end}}`, worktree: "wt-{{.BranchSlug}}", wantError: "BranchSlug"},
		{name: "field unavailable in definition", branch: `{{define "hidden"}}{{.BranchSlug}}{{end}}feature/{{.PhraseSlug}}`, worktree: "wt-{{.BranchSlug}}", wantError: "BranchSlug"},
		{name: "nested direct field rejected", branch: `{{if .PhraseSlug}}feature/{{.PhraseSlug}}{{else}}{{.PhraseSlug.Value}}{{end}}`, worktree: "wt-{{.BranchSlug}}", wantError: "nested field access"},
		{name: "nested variable field rejected", branch: `{{$slug := .PhraseSlug}}{{if $slug}}feature/{{$slug}}{{else}}{{$slug.Value}}{{end}}`, worktree: "wt-{{.BranchSlug}}", wantError: "nested field access"},
		{name: "unavailable root variable field rejected", branch: `{{if .PhraseSlug}}feature/{{.PhraseSlug}}{{else}}{{$.BranchSlug}}{{end}}`, worktree: "wt-{{.BranchSlug}}", wantError: "BranchSlug"},
		{name: "nested root variable field rejected", branch: `{{if .PhraseSlug}}feature/{{.PhraseSlug}}{{else}}{{$.PhraseSlug.Value}}{{end}}`, worktree: "wt-{{.BranchSlug}}", wantError: "nested field access"},
		{name: "parenthesized nested field rejected", branch: `{{if .PhraseSlug}}feature/{{.PhraseSlug}}{{else}}{{(.PhraseSlug).Value}}{{end}}`, worktree: "wt-{{.BranchSlug}}", wantError: "nested field access"},
		{name: "empty branch", branch: "", worktree: "wt-{{.BranchSlug}}", wantError: "empty"},
		{name: "branch leading dash", branch: "-{{.PhraseSlug}}", worktree: "wt-{{.BranchSlug}}", wantError: "starts with"},
		{name: "branch double dot", branch: "bad..{{.PhraseSlug}}", worktree: "wt-{{.BranchSlug}}", wantError: "contains '..'"},
		{name: "branch control", branch: "bad\t{{.PhraseSlug}}", worktree: "wt-{{.BranchSlug}}", wantError: "control character"},
		{name: "worktree parse error", branch: "feature/{{.PhraseSlug}}", worktree: "{{.BranchSlug", wantError: "worktree_template"},
		{name: "worktree field unavailable", branch: "feature/{{.PhraseSlug}}", worktree: "{{.PhraseSlug}}", wantError: "PhraseSlug"},
		{name: "empty worktree", branch: "feature/{{.PhraseSlug}}", worktree: "", wantError: "worktree name is empty"},
		{name: "worktree slash", branch: "feature/{{.PhraseSlug}}", worktree: "dir/{{.BranchSlug}}", wantError: "contains '/'"},
		{name: "worktree leading dash", branch: "feature/{{.PhraseSlug}}", worktree: "-{{.BranchSlug}}", wantError: "starts with"},
		{name: "worktree dot", branch: "feature/{{.PhraseSlug}}", worktree: ".", wantError: `is "."`},
		{name: "worktree dot dot", branch: "feature/{{.PhraseSlug}}", worktree: "..", wantError: `is ".."`},
		{name: "worktree control", branch: "feature/{{.PhraseSlug}}", worktree: "wt-\u0085{{.BranchSlug}}", wantError: "control character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLocalBranchNamer(
				config.LocalBranchConfig{BranchTemplate: tt.branch, WorktreeTemplate: tt.worktree},
				testNamingConfig(),
			)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("NewLocalBranchNamer() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("NewLocalBranchNamer() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestLocalBranchNamerGenerate(t *testing.T) {
	tests := []struct {
		name         string
		localCfg     config.LocalBranchConfig
		namingCfg    config.NamingConfig
		phrase       string
		wantBranch   string
		wantWorktree string
	}{
		{
			name:         "fixed slug pipeline",
			localCfg:     testLocalConfig(),
			namingCfg:    testNamingConfig(),
			phrase:       "Add user!!! Auth",
			wantBranch:   "feature/add-user-auth",
			wantWorktree: "wt-add-user-auth",
		},
		{
			name:     "preserve case",
			localCfg: testLocalConfig(),
			namingCfg: config.NamingConfig{
				StripPrefixes: []string{"feature/"},
			},
			phrase:       "Add User",
			wantBranch:   "feature/Add-User",
			wantWorktree: "wt-Add-User",
		},
		{
			name: "cap final rendered names",
			localCfg: config.LocalBranchConfig{
				BranchTemplate:   "feature/{{.PhraseSlug}}",
				WorktreeTemplate: "worktree-{{.BranchSlug}}",
			},
			namingCfg: config.NamingConfig{
				Lowercase:     true,
				MaxLength:     10,
				StripPrefixes: []string{"feature/"},
			},
			phrase:       "abcdefghijk",
			wantBranch:   "feature/ab",
			wantWorktree: "worktree-a",
		},
		{
			name: "rune safe template literal",
			localCfg: config.LocalBranchConfig{
				BranchTemplate:   "éé{{.PhraseSlug}}",
				WorktreeTemplate: "éé{{.BranchSlug}}",
			},
			namingCfg:    config.NamingConfig{Lowercase: true, MaxLength: 3},
			phrase:       "abc",
			wantBranch:   "ééa",
			wantWorktree: "ééa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := requireLocalNamer(t, tt.localCfg, tt.namingCfg)
			branch, err := namer.GenerateBranchName(tt.phrase)
			if err != nil {
				t.Fatalf("GenerateBranchName() error = %v", err)
			}
			if branch != tt.wantBranch {
				t.Fatalf("GenerateBranchName() = %q, want %q", branch, tt.wantBranch)
			}

			worktree, err := namer.GenerateWorktreeName(branch)
			if err != nil {
				t.Fatalf("GenerateWorktreeName() error = %v", err)
			}
			if worktree != tt.wantWorktree {
				t.Fatalf("GenerateWorktreeName() = %q, want %q", worktree, tt.wantWorktree)
			}
		})
	}
}

func TestLocalBranchNamerStripsFirstMatchingPrefix(t *testing.T) {
	tests := []struct {
		name     string
		prefixes []string
		want     string
	}{
		{name: "short prefix first", prefixes: []string{"feature/", "feature/long/"}, want: "wt-long-topic"},
		{name: "long prefix first", prefixes: []string{"feature/long/", "feature/"}, want: "wt-topic"},
		{name: "first matching not first configured", prefixes: []string{"fix/", "feature/"}, want: "wt-long-topic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namingCfg := testNamingConfig()
			namingCfg.StripPrefixes = tt.prefixes
			namer := requireLocalNamer(t, testLocalConfig(), namingCfg)
			got, err := namer.GenerateWorktreeName("feature/long/topic")
			if err != nil {
				t.Fatalf("GenerateWorktreeName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("GenerateWorktreeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocalBranchNamerEmptySlug(t *testing.T) {
	namer := requireLocalNamer(t, testLocalConfig(), testNamingConfig())

	if _, err := namer.GenerateBranchName("***"); !errors.Is(err, ErrEmptySlug) {
		t.Fatalf("GenerateBranchName() error = %v, want ErrEmptySlug", err)
	}
	if _, err := namer.GenerateWorktreeName("---"); !errors.Is(err, ErrEmptySlug) {
		t.Fatalf("GenerateWorktreeName() error = %v, want ErrEmptySlug", err)
	}
}

func TestLocalBranchNamerValidatesActualFinalOutput(t *testing.T) {
	tests := []struct {
		name      string
		localCfg  config.LocalBranchConfig
		generate  func(*LocalBranchNamer) error
		wantError string
	}{
		{
			name: "branch invalid after render and truncate",
			localCfg: config.LocalBranchConfig{
				BranchTemplate:   `{{if eq .PhraseSlug "bad"}}-x{{else}}ok-{{.PhraseSlug}}{{end}}`,
				WorktreeTemplate: "wt-{{.BranchSlug}}",
			},
			generate: func(n *LocalBranchNamer) error {
				_, err := n.GenerateBranchName("bad")
				return err
			},
			wantError: "empty",
		},
		{
			name: "worktree invalid after render and truncate",
			localCfg: config.LocalBranchConfig{
				BranchTemplate:   "feature/{{.PhraseSlug}}",
				WorktreeTemplate: `{{if eq .BranchSlug "bad"}}.x{{else}}ok-{{.BranchSlug}}{{end}}`,
			},
			generate: func(n *LocalBranchNamer) error {
				_, err := n.GenerateWorktreeName("bad")
				return err
			},
			wantError: `worktree name is "."`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namingCfg := testNamingConfig()
			namingCfg.MaxLength = 1
			namer := requireLocalNamer(t, tt.localCfg, namingCfg)
			err := tt.generate(namer)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("generation error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestLocalBranchNamerLiteralPrefix(t *testing.T) {
	localCfg := testLocalConfig()
	localCfg.WorktreeTemplate = "subagent-{{if .BranchSlug}}{{.BranchSlug}}{{end}}"
	namer := requireLocalNamer(t, localCfg, testNamingConfig())

	if got := namer.WorktreeLiteralPrefix(); got != "subagent-" {
		t.Fatalf("WorktreeLiteralPrefix() = %q, want %q", got, "subagent-")
	}
	if !namer.HasPrefix("subagent-topic") {
		t.Fatal("HasPrefix() = false, want true")
	}
	if namer.HasPrefix("wt-topic") {
		t.Fatal("HasPrefix() = true, want false")
	}
	if got := namer.ExtractFromAbsolutePath("/workspace/subagent-topic"); got != "topic" {
		t.Fatalf("ExtractFromAbsolutePath() = %q, want %q", got, "topic")
	}
}

func TestLeadingLiteral(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{source: "prefix-{{.PhraseSlug}}-suffix", want: "prefix-"},
		{source: "before-{{if .PhraseSlug}}inside{{end}}", want: "before-"},
		{source: "{{.PhraseSlug}}-suffix", want: ""},
		{source: "literal-only", want: "literal-only"},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			tmpl, err := template.New("test").Parse(tt.source)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got := leadingLiteral(tmpl); got != tt.want {
				t.Fatalf("leadingLiteral() = %q, want %q", got, tt.want)
			}
		})
	}
}
