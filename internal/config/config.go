package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Config represents the complete grove configuration.
type Config struct {
	Git         GitConfig         `toml:"git"`
	GitHub      GitHubConfig      `toml:"github"`
	Issue       IssueConfig       `toml:"issue"`
	LocalBranch LocalBranchConfig `toml:"local_branch"`
	Naming      NamingConfig      `toml:"naming"`
	PullRequest PullRequestConfig `toml:"pull_request"`
	Worktree    WorktreeConfig    `toml:"worktree"`
}

// Validate checks that all config values are valid.
// Returns an error describing the first invalid value found.
func (c Config) Validate() error {
	if c.Git.Timeout < 0 {
		return errors.New("git.timeout cannot be negative")
	}
	if c.GitHub.PreviewCacheTTL < 0 {
		return errors.New("github.preview_cache_ttl cannot be negative")
	}
	if c.Issue.BranchTemplate == "" {
		return errors.New("issue.branch_template cannot be empty")
	}
	if c.Issue.WorktreeTemplate == "" {
		return errors.New("issue.worktree_template cannot be empty")
	}
	if c.LocalBranch.BranchTemplate == "" {
		return errors.New("local_branch.branch_template cannot be empty")
	}
	if c.LocalBranch.WorktreeTemplate == "" {
		return errors.New("local_branch.worktree_template cannot be empty")
	}
	if c.Naming.MaxLength < 0 {
		return errors.New("naming.max_length cannot be negative")
	}
	if c.PullRequest.BranchTemplate == "" {
		return errors.New("pull_request.branch_template cannot be empty")
	}
	if c.PullRequest.WorktreeTemplate == "" {
		return errors.New("pull_request.worktree_template cannot be empty")
	}
	if c.Worktree.Layout == "" {
		return errors.New("worktree.layout cannot be empty")
	}
	if c.Worktree.Root == "" {
		return errors.New("worktree.root cannot be empty")
	}
	if err := ValidateSpace(c.Worktree.Space); err != nil {
		return fmt.Errorf("worktree.space %w", err)
	}
	return nil
}

// ValidateSpace checks that a space is a single path segment: non-empty, no
// slash, not "." or "..", not starting with "-", and free of control
// characters. The same rules apply to worktree names.
func ValidateSpace(space string) error {
	switch {
	case space == "":
		return errors.New("cannot be empty")
	case strings.ContainsRune(space, '/'):
		return errors.New("cannot contain '/'")
	case strings.HasPrefix(space, "-"):
		return errors.New("cannot start with '-'")
	case space == "." || space == "..":
		return fmt.Errorf("cannot be %q", space)
	}
	for _, r := range space {
		if unicode.IsControl(r) {
			return errors.New("cannot contain control characters")
		}
	}
	return nil
}

// GitConfig configures git command execution.
type GitConfig struct {
	Timeout time.Duration `toml:"timeout"` // Timeout for git and gh commands; zero disables the deadline
}

// GitHubConfig configures GitHub-related behavior.
type GitHubConfig struct {
	PreviewCacheTTL time.Duration `toml:"preview_cache_ttl"` // TTL for FZF preview cache (e.g., "5m"); 0 disables
}

// IssueConfig configures issue naming.
type IssueConfig struct {
	BranchTemplate   string `toml:"branch_template"`
	WorktreeTemplate string `toml:"worktree_template"`
}

// LocalBranchConfig configures local branch naming.
type LocalBranchConfig struct {
	BranchTemplate   string `toml:"branch_template"`
	WorktreeTemplate string `toml:"worktree_template" config:"worktree-template" help:"Override the local branch worktree directory template (local_branch.worktree_template)"`
}

// NamingConfig configures generated names.
type NamingConfig struct {
	Lowercase     bool     `toml:"lowercase"`
	MaxLength     int      `toml:"max_length"`
	StripPrefixes []string `toml:"strip_prefixes"`
}

// PullRequestConfig configures pull request naming.
type PullRequestConfig struct {
	BranchTemplate   string `toml:"branch_template"`
	WorktreeTemplate string `toml:"worktree_template"`
}

// WorktreeConfig configures where new worktrees are placed.
type WorktreeConfig struct {
	Layout string `toml:"layout"` // Go template rendered under Root; see the layout help topic
	Root   string `toml:"root"`   // Base directory; $VAR, ${VAR}, and a leading ~ are expanded
	Space  string `toml:"space" config:"space" help:"Override the space segment of new worktree paths (worktree.space)"`
}
