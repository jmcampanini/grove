package config

import (
	"errors"
	"time"
)

// Config represents the complete grove configuration.
type Config struct {
	Branch   BranchConfig   `toml:"branch"`
	Git      GitConfig      `toml:"git"`
	PR       PRConfig       `toml:"pr"`
	Slugify  SlugifyConfig  `toml:"slugify"`
	Worktree WorktreeConfig `toml:"worktree"`
}

// Validate checks that all config values are valid.
// Returns an error describing the first invalid value found.
func (c Config) Validate() error {
	if c.Git.Timeout < 0 {
		return errors.New("git.timeout cannot be negative")
	}
	if c.PR.WorktreePrefix == "" {
		return errors.New("pr.worktree_prefix cannot be empty")
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
	return nil
}

// BranchConfig configures branch naming.
type BranchConfig struct {
	NewPrefix string `toml:"new_prefix"` // e.g., "feature/"
}

// GitConfig configures git command execution.
type GitConfig struct {
	Timeout time.Duration `toml:"timeout"` // Timeout for git commands (e.g., "5s")
}

// PRConfig configures pull request worktree naming.
type PRConfig struct {
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

// WorktreeConfig configures worktree naming.
type WorktreeConfig struct {
	NewPrefix string `toml:"new_prefix"` // e.g., "wt-"
	// StripBranchPrefix is a list of prefixes to strip from branch names.
	// Only the first matching prefix is stripped (checked in list order).
	// e.g., branch "feature/add-auth" with ["fix/", "feature/"] -> "add-auth"
	StripBranchPrefix []string `toml:"strip_branch_prefix"`
}
