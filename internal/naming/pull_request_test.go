package naming

import (
	"strings"
	"testing"

	"github.com/jmcampanini/grove/internal/config"
)

func testPullRequestConfig() config.PullRequestConfig {
	return config.PullRequestConfig{
		BranchTemplate:   "{{.Branch}}",
		WorktreeTemplate: "pr-{{.Number}}-{{.TitleSlug}}",
	}
}

func requirePullRequestNamer(t *testing.T, prCfg config.PullRequestConfig, namingCfg config.NamingConfig) *PullRequestNamer {
	t.Helper()
	namer, err := NewPullRequestNamer(prCfg, namingCfg)
	if err != nil {
		t.Fatalf("NewPullRequestNamer() error = %v", err)
	}
	return namer
}

func TestNewPullRequestNamerValidatesTemplates(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		worktree  string
		wantError string
	}{
		{name: "valid variables", branch: "review/{{.Number}}/{{.Branch}}", worktree: "pr-{{.Number}}-{{.TitleSlug}}-{{.BranchSlug}}"},
		{name: "branch parse error", branch: "{{.Branch", worktree: "pr-{{.Number}}", wantError: "branch_template"},
		{name: "title slug unavailable to branch", branch: "{{.Number}}-{{.TitleSlug}}", worktree: "pr-{{.Number}}", wantError: "TitleSlug"},
		{name: "invalid branch", branch: "-{{.Branch}}", worktree: "pr-{{.Number}}", wantError: "starts with"},
		{name: "worktree parse error", branch: "{{.Branch}}", worktree: "{{.Number", wantError: "worktree_template"},
		{name: "raw branch unavailable to worktree", branch: "{{.Branch}}", worktree: "{{.Branch}}", wantError: "Branch"},
		{name: "worktree slash", branch: "{{.Branch}}", worktree: "pr/{{.Number}}", wantError: "contains '/'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPullRequestNamer(
				config.PullRequestConfig{BranchTemplate: tt.branch, WorktreeTemplate: tt.worktree},
				testNamingConfig(),
			)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("NewPullRequestNamer() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("NewPullRequestNamer() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestPullRequestNamerGenerate(t *testing.T) {
	tests := []struct {
		name         string
		prCfg        config.PullRequestConfig
		namingCfg    config.NamingConfig
		data         PullRequestTemplateData
		title        string
		wantBranch   string
		wantWorktree string
	}{
		{
			name: "all template variables",
			prCfg: config.PullRequestConfig{
				BranchTemplate:   "review/{{.Number}}/{{.Branch}}",
				WorktreeTemplate: "pr-{{.Number}}-{{.TitleSlug}}-{{.BranchSlug}}",
			},
			namingCfg:    testNamingConfig(),
			data:         PullRequestTemplateData{Branch: "feature/Add Auth", Number: 117},
			title:        "Issue & PR Numbers",
			wantBranch:   "review/117/feature/Add Auth",
			wantWorktree: "pr-117-issue-pr-numbers-review-117-feature-add-auth",
		},
		{
			name:      "branch is exempt from cap",
			prCfg:     testPullRequestConfig(),
			namingCfg: config.NamingConfig{Lowercase: true, MaxLength: 12},
			data: PullRequestTemplateData{
				Branch: "feature/a-very-long-remote-branch",
				Number: 42,
			},
			title:        "a very long title",
			wantBranch:   "feature/a-very-long-remote-branch",
			wantWorktree: "pr-42-a-very",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := requirePullRequestNamer(t, tt.prCfg, tt.namingCfg)
			branch, err := namer.GenerateBranchName(tt.data)
			if err != nil {
				t.Fatalf("GenerateBranchName() error = %v", err)
			}
			if branch != tt.wantBranch {
				t.Fatalf("GenerateBranchName() = %q, want %q", branch, tt.wantBranch)
			}

			worktree, err := namer.GenerateWorktreeName(tt.data.Number, tt.title, branch)
			if err != nil {
				t.Fatalf("GenerateWorktreeName() error = %v", err)
			}
			if worktree != tt.wantWorktree {
				t.Fatalf("GenerateWorktreeName() = %q, want %q", worktree, tt.wantWorktree)
			}
		})
	}
}

func TestPullRequestNamerStripsFirstMatchingPrefix(t *testing.T) {
	prCfg := testPullRequestConfig()
	prCfg.WorktreeTemplate = "pr-{{.BranchSlug}}"
	tests := []struct {
		name     string
		prefixes []string
		want     string
	}{
		{name: "short first", prefixes: []string{"review/", "review/long/"}, want: "pr-long-topic"},
		{name: "long first", prefixes: []string{"review/long/", "review/"}, want: "pr-topic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namingCfg := testNamingConfig()
			namingCfg.StripPrefixes = tt.prefixes
			namer := requirePullRequestNamer(t, prCfg, namingCfg)
			got, err := namer.GenerateWorktreeName(1, "title", "review/long/topic")
			if err != nil {
				t.Fatalf("GenerateWorktreeName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("GenerateWorktreeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRequestNamerValidatesActualFinalOutput(t *testing.T) {
	tests := []struct {
		name      string
		prCfg     config.PullRequestConfig
		generate  func(*PullRequestNamer) error
		namingCfg config.NamingConfig
		wantError string
	}{
		{
			name:  "raw branch invalid at runtime",
			prCfg: testPullRequestConfig(),
			generate: func(n *PullRequestNamer) error {
				_, err := n.GenerateBranchName(PullRequestTemplateData{Branch: "-bad", Number: 1})
				return err
			},
			namingCfg: testNamingConfig(),
			wantError: "starts with",
		},
		{
			name: "worktree invalid after truncation",
			prCfg: config.PullRequestConfig{
				BranchTemplate:   "{{.Branch}}",
				WorktreeTemplate: `{{if lt .Number 0}}.x{{else}}pr-{{.Number}}{{end}}`,
			},
			generate: func(n *PullRequestNamer) error {
				_, err := n.GenerateWorktreeName(-1, "title", "feature/topic")
				return err
			},
			namingCfg: config.NamingConfig{Lowercase: true, MaxLength: 1},
			wantError: `is "."`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := requirePullRequestNamer(t, tt.prCfg, tt.namingCfg)
			err := tt.generate(namer)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("generation error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestPullRequestNamerWorktreeLiteralPrefix(t *testing.T) {
	prCfg := testPullRequestConfig()
	prCfg.WorktreeTemplate = "pull-{{.Number}}-{{.TitleSlug}}"
	namer := requirePullRequestNamer(t, prCfg, testNamingConfig())

	if got := namer.WorktreeLiteralPrefix(); got != "pull-" {
		t.Fatalf("WorktreeLiteralPrefix() = %q, want %q", got, "pull-")
	}
}
