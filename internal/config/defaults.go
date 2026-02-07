package config

import "time"

// DefaultConfig returns sensible defaults for all configuration.
func DefaultConfig() Config {
	return Config{
		Branch: BranchConfig{
			NewPrefix: "feature/",
		},
		Git: GitConfig{
			Timeout: 5 * time.Second,
		},
		LocalBranch: LocalBranchConfig{
			WorktreePrefix:    "wt-",
			StripBranchPrefix: []string{"feature/"},
		},
		PullRequest: PullRequestConfig{
			BranchTemplate: "{{.BranchName}}",
			WorktreePrefix: "pr-",
		},
		Slugify: SlugifyConfig{
			CollapseDashes:     true,
			HashLength:         4,
			Lowercase:          true,
			MaxLength:          50,
			ReplaceNonAlphanum: true,
			TrimDashes:         true,
		},
	}
}
