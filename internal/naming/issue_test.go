package naming

import (
	"strings"
	"testing"

	"github.com/jmcampanini/grove/internal/config"
)

func testIssueConfig() config.IssueConfig {
	return config.IssueConfig{
		BranchTemplate:   "issue/{{.Number}}-{{.TitleSlug}}",
		WorktreeTemplate: "is-{{.Number}}-{{.TitleSlug}}",
	}
}

func requireIssueNamer(t *testing.T, issueCfg config.IssueConfig, namingCfg config.NamingConfig) *IssueNamer {
	t.Helper()
	namer, err := NewIssueNamer(issueCfg, namingCfg)
	if err != nil {
		t.Fatalf("NewIssueNamer() error = %v", err)
	}
	return namer
}

func TestNewIssueNamerValidatesTemplates(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		worktree  string
		wantError string
	}{
		{name: "valid variables", branch: "issue/{{.Number}}-{{.TitleSlug}}", worktree: "is-{{.Number}}-{{.TitleSlug}}-{{.BranchSlug}}"},
		{name: "branch parse error", branch: "issue/{{.Number", worktree: "is-{{.Number}}", wantError: "branch_template"},
		{name: "raw title unavailable", branch: "issue/{{.Number}}-{{.Title}}", worktree: "is-{{.Number}}", wantError: "Title"},
		{name: "branch slug unavailable", branch: "issue/{{.Number}}-{{.BranchSlug}}", worktree: "is-{{.Number}}", wantError: "BranchSlug"},
		{name: "invalid branch", branch: "-{{.Number}}", worktree: "is-{{.Number}}", wantError: "starts with"},
		{name: "direct number required", branch: "issue/{{.TitleSlug}}", worktree: "is-{{.Number}}", wantError: "must directly render {{.Number}}"},
		{name: "indirect number rejected", branch: "{{with .Number}}issue/{{.}}{{end}}", worktree: "is-{{.Number}}", wantError: "must directly render {{.Number}}"},
		{name: "number in control structure accepted", branch: "{{if .TitleSlug}}issue/{{.Number}}{{end}}", worktree: "is-{{.Number}}"},
		{name: "number in definition accepted", branch: `{{define "branch"}}issue/{{.Number}}{{end}}{{template "branch" .}}`, worktree: "is-{{.Number}}"},
		{name: "worktree parse error", branch: "issue/{{.Number}}", worktree: "is-{{.Number", wantError: "worktree_template"},
		{name: "phrase slug unavailable", branch: "issue/{{.Number}}", worktree: "{{.PhraseSlug}}", wantError: "PhraseSlug"},
		{name: "worktree slash", branch: "issue/{{.Number}}", worktree: "is/{{.Number}}", wantError: "contains '/'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIssueNamer(
				config.IssueConfig{BranchTemplate: tt.branch, WorktreeTemplate: tt.worktree},
				testNamingConfig(),
			)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("NewIssueNamer() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("NewIssueNamer() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestIssueNamerGenerate(t *testing.T) {
	tests := []struct {
		name         string
		issueCfg     config.IssueConfig
		namingCfg    config.NamingConfig
		number       int
		title        string
		branchInput  string
		wantBranch   string
		wantWorktree string
	}{
		{
			name: "all template variables",
			issueCfg: config.IssueConfig{
				BranchTemplate:   "ticket/{{.Number}}-{{.TitleSlug}}",
				WorktreeTemplate: "is-{{.Number}}-{{.TitleSlug}}-{{.BranchSlug}}",
			},
			namingCfg:    testNamingConfig(),
			number:       117,
			title:        "Issue & PR Numbers",
			branchInput:  "issue/117-issue-pr-numbers",
			wantBranch:   "ticket/117-issue-pr-numbers",
			wantWorktree: "is-117-issue-pr-numbers-117-issue-pr-numbers",
		},
		{
			name:     "cap branch and worktree after rendering",
			issueCfg: testIssueConfig(),
			namingCfg: config.NamingConfig{
				Lowercase: true,
				MaxLength: 12,
			},
			number:       42,
			title:        "abcdefgh",
			branchInput:  "issue/42-abcdefgh",
			wantBranch:   "issue/42-abc",
			wantWorktree: "is-42-abcdef",
		},
		{
			name: "title slug is not independently capped",
			issueCfg: config.IssueConfig{
				BranchTemplate:   "{{.Number}}-{{.TitleSlug}}",
				WorktreeTemplate: "is-{{.Number}}",
			},
			namingCfg:    testNamingConfig(),
			number:       9,
			title:        "abcdefghijklmnopqrstuvwxyz ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			branchInput:  "issue/9",
			wantBranch:   "9-abcdefghijklmnopqrstuvwxyz-abcdefghijklmnopqrstuvwxyz",
			wantWorktree: "is-9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namer := requireIssueNamer(t, tt.issueCfg, tt.namingCfg)
			branch, err := namer.GenerateBranchName(tt.number, tt.title)
			if err != nil {
				t.Fatalf("GenerateBranchName() error = %v", err)
			}
			if branch != tt.wantBranch {
				t.Fatalf("GenerateBranchName() = %q, want %q", branch, tt.wantBranch)
			}

			worktree, err := namer.GenerateWorktreeName(tt.number, tt.title, tt.branchInput)
			if err != nil {
				t.Fatalf("GenerateWorktreeName() error = %v", err)
			}
			if worktree != tt.wantWorktree {
				t.Fatalf("GenerateWorktreeName() = %q, want %q", worktree, tt.wantWorktree)
			}
		})
	}
}

func TestIssueNamerGenerateBranchNameRequiresCompleteNumber(t *testing.T) {
	tests := []struct {
		name           string
		branchTemplate string
		maxLength      int
		number         int
		title          string
		want           string
		wantError      string
	}{
		{
			name:           "truncation cuts runtime number",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			maxLength:      8,
			number:         123,
			title:          "title",
			wantError:      "complete issue number 123",
		},
		{
			name:           "truncation keeps complete runtime number",
			branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}",
			maxLength:      9,
			number:         123,
			title:          "title",
			want:           "issue/123",
		},
		{
			name:           "adjacent digit does not satisfy anchor boundary",
			branchTemplate: "issue/{{.Number}}{{.TitleSlug}}",
			number:         123,
			title:          "4 title",
			wantError:      "complete issue number 123",
		},
		{
			name:           "truncation removes number from title first template",
			branchTemplate: "{{.TitleSlug}}-{{.Number}}",
			maxLength:      5,
			number:         123,
			title:          "login",
			wantError:      "complete issue number 123",
		},
		{
			name:           "title digits do not substitute for truncated number field",
			branchTemplate: "{{.TitleSlug}}-{{.Number}}",
			maxLength:      9,
			number:         123,
			title:          "login 123",
			wantError:      "complete issue number 123",
		},
		{
			name:           "partial number expansion does not combine with title digits",
			branchTemplate: "{{.TitleSlug}}-{{.Number}}",
			maxLength:      6,
			number:         199,
			title:          "199",
			wantError:      "complete issue number 199",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issueCfg := testIssueConfig()
			issueCfg.BranchTemplate = tt.branchTemplate
			namingCfg := testNamingConfig()
			namingCfg.MaxLength = tt.maxLength
			namer := requireIssueNamer(t, issueCfg, namingCfg)

			got, err := namer.GenerateBranchName(tt.number, tt.title)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("GenerateBranchName() error = %v, want containing %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("GenerateBranchName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("GenerateBranchName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIssueNamerNonAnchorUsesExactRegenerationWhenNumberSurvives(t *testing.T) {
	issueCfg := testIssueConfig()
	issueCfg.BranchTemplate = "{{.TitleSlug}}-{{.Number}}"
	namingCfg := testNamingConfig()
	namingCfg.MaxLength = 10
	namer := requireIssueNamer(t, issueCfg, namingCfg)

	branch, err := namer.GenerateBranchName(123, "abcdef")
	if err != nil {
		t.Fatalf("GenerateBranchName() error = %v", err)
	}
	if branch != "abcdef-123" {
		t.Fatalf("GenerateBranchName() = %q, want %q", branch, "abcdef-123")
	}
	if !namer.MatchesIssueNumber("abcdef-123", 123, "abcdef") {
		t.Fatal("MatchesIssueNumber() = false for exact regenerated branch")
	}
	if namer.MatchesIssueNumber("abcdeg-123", 123, "abcdef") {
		t.Fatal("MatchesIssueNumber() = true for non-matching branch")
	}
}

func TestIssueNamerTitleSlugIsUncapped(t *testing.T) {
	namingCfg := testNamingConfig()
	namingCfg.MaxLength = 5
	namer := requireIssueNamer(t, testIssueConfig(), namingCfg)

	if got := namer.TitleSlug("A very long issue title"); got != "a-very-long-issue-title" {
		t.Fatalf("TitleSlug() = %q, want uncapped slug", got)
	}
}

func TestIssueNamerStripsFirstMatchingPrefix(t *testing.T) {
	issueCfg := testIssueConfig()
	issueCfg.WorktreeTemplate = "is-{{.BranchSlug}}"
	tests := []struct {
		name     string
		prefixes []string
		want     string
	}{
		{name: "short first", prefixes: []string{"issue/", "issue/long/"}, want: "is-long-topic"},
		{name: "long first", prefixes: []string{"issue/long/", "issue/"}, want: "is-topic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namingCfg := testNamingConfig()
			namingCfg.StripPrefixes = tt.prefixes
			namer := requireIssueNamer(t, issueCfg, namingCfg)
			got, err := namer.GenerateWorktreeName(1, "title", "issue/long/topic")
			if err != nil {
				t.Fatalf("GenerateWorktreeName() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("GenerateWorktreeName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIssueNamerValidatesActualFinalOutput(t *testing.T) {
	tests := []struct {
		name      string
		issueCfg  config.IssueConfig
		generate  func(*IssueNamer) error
		maxLength int
		wantError string
	}{
		{
			name: "branch",
			issueCfg: config.IssueConfig{
				BranchTemplate:   `{{if lt .Number 0}}-bad{{else}}issue/{{.Number}}{{end}}`,
				WorktreeTemplate: "is-{{.Number}}",
			},
			generate: func(n *IssueNamer) error {
				_, err := n.GenerateBranchName(-1, "title")
				return err
			},
			wantError: "starts with",
		},
		{
			name: "worktree after truncation",
			issueCfg: config.IssueConfig{
				BranchTemplate:   "issue/{{.Number}}",
				WorktreeTemplate: `{{if lt .Number 0}}.x{{else}}is-{{.Number}}{{end}}`,
			},
			generate: func(n *IssueNamer) error {
				_, err := n.GenerateWorktreeName(-1, "title", "issue/1")
				return err
			},
			maxLength: 1,
			wantError: `is "."`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namingCfg := testNamingConfig()
			namingCfg.MaxLength = tt.maxLength
			namer := requireIssueNamer(t, tt.issueCfg, namingCfg)
			err := tt.generate(namer)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("generation error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestIssueNamerWorktreeLiteralPrefix(t *testing.T) {
	issueCfg := testIssueConfig()
	issueCfg.WorktreeTemplate = "issue-dir-{{.Number}}-{{.TitleSlug}}"
	namer := requireIssueNamer(t, issueCfg, testNamingConfig())

	if got := namer.WorktreeLiteralPrefix(); got != "issue-dir-" {
		t.Fatalf("WorktreeLiteralPrefix() = %q, want %q", got, "issue-dir-")
	}
}

func TestIssueNamerRejectsTruncatedTitleFirstCollision(t *testing.T) {
	issueCfg := testIssueConfig()
	issueCfg.BranchTemplate = "{{.TitleSlug}}-{{.Number}}"
	namingCfg := testNamingConfig()
	namingCfg.MaxLength = 6
	namer := requireIssueNamer(t, issueCfg, namingCfg)

	if namer.MatchesIssueNumber("199-19", 199, "199") {
		t.Fatal("MatchesIssueNumber() matched a branch with a partial issue number")
	}
	if !namer.MatchesIssueNumber("199-19", 19, "199") {
		t.Fatal("MatchesIssueNumber() rejected the issue whose complete number survives")
	}
}

func TestIssueNamerMatchesIssueNumber(t *testing.T) {
	tests := []struct {
		name           string
		branchTemplate string
		branch         string
		number         int
		title          string
		want           bool
	}{
		{name: "anchored match survives title edit", branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}", branch: "issue/123-old-title", number: 123, title: "new title", want: true},
		{name: "anchored exact number", branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}", branch: "issue/123", number: 123, title: "title", want: true},
		{name: "reject larger number", branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}", branch: "issue/1234-title", number: 123, title: "title", want: false},
		{name: "enforce configured boundary", branchTemplate: "issue/{{.Number}}-{{.TitleSlug}}", branch: "issue/123x-title", number: 123, title: "title", want: false},
		{name: "slash boundary", branchTemplate: "issue/{{.Number}}/{{.TitleSlug}}", branch: "issue/123/old-title", number: 123, title: "new title", want: true},
		{name: "number last permits nondigit suffix", branchTemplate: "issue/{{.Number}}", branch: "issue/123-old", number: 123, title: "title", want: true},
		{name: "number last rejects digit suffix", branchTemplate: "issue/{{.Number}}", branch: "issue/1234", number: 123, title: "title", want: false},
		{name: "field before number falls back to exact match", branchTemplate: "{{.TitleSlug}}-{{.Number}}", branch: "current-title-123", number: 123, title: "current title", want: true},
		{name: "fallback does not survive title edit", branchTemplate: "{{.TitleSlug}}-{{.Number}}", branch: "old-title-123", number: 123, title: "current title", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issueCfg := testIssueConfig()
			issueCfg.BranchTemplate = tt.branchTemplate
			namer := requireIssueNamer(t, issueCfg, testNamingConfig())
			if got := namer.MatchesIssueNumber(tt.branch, tt.number, tt.title); got != tt.want {
				t.Fatalf("MatchesIssueNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}
