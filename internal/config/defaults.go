package config

import (
	"os"
	"path/filepath"
	"time"
)

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
			BranchTemplate:     "issue/{{.Number}}-{{.TitleSlug}}",
			TitleSlugMaxLength: 40,
			WorktreePrefix:     "issue-",
		},
		LocalBranch: LocalBranchConfig{
			BranchPrefix:      "feature/",
			StripBranchPrefix: []string{"feature/"},
			WorktreePrefix:    "wt-",
		},
		Log: LogConfig{
			File: DefaultLogFilePath(),
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
		Workspace: WorkspaceConfig{
			PrimaryBranches: []string{"main", "develop", "master"},
		},
	}
}

// DefaultLogFilePath returns the default log file path following the XDG Base
// Directory Specification. It uses $XDG_STATE_HOME/grove/grove.log, falling
// back to ~/.local/state/grove/grove.log. Returns empty string if the home
// directory cannot be determined, which disables file logging.
func DefaultLogFilePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "grove", "grove.log")
}
