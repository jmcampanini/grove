package config

import (
	"os"
	"path/filepath"
)

const configFileName = "grove.toml"

// ConfigPaths returns ordered list of config file paths to check.
// Paths are ordered from lowest to highest priority, so that when decoded
// sequentially, each subsequent file overrides values from previous files.
//
// Order (lowest to highest priority):
//  1. File in XDG config directory (~/.config/grove/grove.toml)
//  2. Files walking up from git root toward home directory
//  3. File in git repository root (main worktree)
//  4. File in current worktree root (if different from git root)
//  5. File in current working directory (if different from worktree root)
//
// The worktreeRoot and gitRoot may be the same directory if running from the main worktree.
// Empty strings for worktreeRoot or gitRoot are handled gracefully.
func ConfigPaths(cwd, worktreeRoot, gitRoot, homeDir string) []string {
	var paths []string
	seen := make(map[string]bool)

	addPath := func(dir string) {
		if dir == "" {
			return
		}
		path := filepath.Join(dir, configFileName)
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}

	if xdgConfigDir := os.Getenv("XDG_CONFIG_HOME"); xdgConfigDir != "" {
		addPath(filepath.Join(xdgConfigDir, "grove"))
	} else if homeDir != "" {
		addPath(filepath.Join(homeDir, ".config", "grove"))
	}

	if gitRoot != "" && homeDir != "" {
		// Collect ancestors from gitRoot's parent up to home
		var ancestors []string
		current := filepath.Dir(gitRoot)
		for current != "" && len(current) >= len(homeDir) {
			ancestors = append(ancestors, current)
			if current == homeDir {
				break
			}
			parent := filepath.Dir(current)
			if parent == current {
				break // reached filesystem root
			}
			current = parent
		}

		// Add in reverse order: home first (lowest priority), closest to gitRoot last
		for i := len(ancestors) - 1; i >= 0; i-- {
			addPath(ancestors[i])
		}
	}

	addPath(gitRoot)
	addPath(worktreeRoot)
	addPath(cwd)

	return paths
}

// BootstrapConfigPaths returns config paths that don't require git context.
// This is used during workspace root detection, before git context is available.
//
// Order (lowest to highest priority):
//  1. File in XDG config directory (~/.config/grove/grove.toml)
//  2. File in current working directory
func BootstrapConfigPaths(cwd, homeDir string) []string {
	return ConfigPaths(cwd, "", "", homeDir)
}
