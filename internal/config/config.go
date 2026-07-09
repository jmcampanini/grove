package config

import (
	"errors"
	"time"
)

// Config represents the complete grove configuration.
type Config struct {
	Git         GitConfig         `toml:"git"`
	GitHub      GitHubConfig      `toml:"github"`
	LocalBranch LocalBranchConfig `toml:"local_branch"`
	Log         LogConfig         `toml:"log"`
	PullRequest PullRequestConfig `toml:"pull_request"`
	Slugify     SlugifyConfig     `toml:"slugify"`
	Workspace   WorkspaceConfig   `toml:"workspace"`
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
	if c.PullRequest.WorktreePrefix == "" {
		return errors.New("pull_request.worktree_prefix cannot be empty")
	}
	if c.Slugify.HashLength < 0 {
		return errors.New("slugify.hash_length cannot be negative")
	}
	if c.Slugify.MaxLength < 0 {
		return errors.New("slugify.max_length cannot be negative")
	}
	if c.Slugify.MaxLength > 0 && c.Slugify.HashLength > c.Slugify.MaxLength-2 {
		return errors.New("slugify.hash_length must be at least 2 less than slugify.max_length")
	}
	if len(c.Workspace.PrimaryBranches) == 0 {
		return errors.New("workspace.primary_branches cannot be empty")
	}
	return nil
}

// GitConfig configures git command execution.
type GitConfig struct {
	Timeout time.Duration `toml:"timeout"` // Timeout for git commands (e.g., "5s")
}

// GitHubConfig configures GitHub-related behavior.
type GitHubConfig struct {
	PreviewCacheTTL time.Duration `toml:"preview_cache_ttl"` // TTL for FZF preview cache (e.g., "5m"); 0 disables
}

// LocalBranchConfig configures local branch worktree naming.
type LocalBranchConfig struct {
	BranchPrefix string `toml:"branch_prefix"` // e.g., "feature/"

	// StripBranchPrefix is a list of prefixes to strip from branch names.
	// Only the first matching prefix is stripped (checked in list order).
	// e.g., branch "feature/add-auth" with ["fix/", "feature/"] -> "add-auth"
	StripBranchPrefix []string `toml:"strip_branch_prefix"`

	WorktreePrefix string `toml:"worktree_prefix" config:"worktree-prefix" help:"Override the local branch worktree directory prefix (local_branch.worktree_prefix)"` // e.g., "wt-"
}

// LogConfig configures file logging.
type LogConfig struct {
	File string `toml:"file"` // Path to log file; empty string disables file logging
}

// PullRequestConfig configures pull request worktree naming.
type PullRequestConfig struct {
	BranchTemplate string `toml:"branch_template"` // Template for local branch name (e.g., "{{.BranchName}}")
	WorktreePrefix string `toml:"worktree_prefix"` // Prefix for PR worktree directories (e.g., "pr-")
}

// SlugifyConfig configures slug generation.
type SlugifyConfig struct {
	CollapseDashes     bool `toml:"collapse_dashes"`
	HashLength         int  `toml:"hash_length"`
	Lowercase          bool `toml:"lowercase"`
	MaxLength          int  `toml:"max_length"`
	ReplaceNonAlphanum bool `toml:"replace_non_alphanum"`
	TrimDashes         bool `toml:"trim_dashes"`
}

// WorkspaceConfig configures workspace root detection.
type WorkspaceConfig struct {
	PrimaryBranches []string `toml:"primary_branches"`
}
