package config

import "time"

// DefaultConfig returns sensible defaults for all configuration.
func DefaultConfig() Config {
	return Config{
		Git: GitConfig{
			Timeout: 5 * time.Second,
		},
		GitHub: GitHubConfig{
			PreviewCacheTTL: 5 * time.Minute,
		},
		Issue: IssueConfig{
			BranchTemplate:   "issue/{{.Number}}-{{.TitleSlug}}",
			WorktreeTemplate: "is-{{.Number}}-{{.TitleSlug}}",
		},
		LocalBranch: LocalBranchConfig{
			BranchTemplate:   "feature/{{.PhraseSlug}}",
			WorktreeTemplate: "wt-{{.BranchSlug}}",
		},
		Naming: NamingConfig{
			Lowercase:     true,
			MaxLength:     30,
			StripPrefixes: []string{"feature/", "fix/", "issue/"},
		},
		PullRequest: PullRequestConfig{
			BranchTemplate:   "{{.Branch}}",
			WorktreeTemplate: "pr-{{.Number}}-{{.TitleSlug}}",
		},
		Workspace: WorkspaceConfig{
			PrimaryBranches: []string{"main", "develop", "master"},
		},
	}
}
