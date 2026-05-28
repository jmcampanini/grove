package cmd

import (
	"io"

	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Print Grove reference documentation",
	Long:  "Print a Markdown reference for Grove commands, configuration loading, and the TOML config schema.",
	Args:  cobra.NoArgs,
	RunE:  runDocs,
}

func init() {
	docsCmd.GroupID = "utility"
	rootCmd.AddCommand(docsCmd)
}

const docsMarkdown = `# grove reference

Grove manages git worktrees as a workspace: one directory per branch, all siblings.
Generated command help remains the canonical reference for flags and examples.

## Command reference

Common commands:

- grove create PHRASE: create a branch and sibling worktree from a phrase.
- grove checkout BRANCH: switch to an existing branch worktree or create it.
- grove pr checkout NUMBER: check out a pull request as a worktree.
- grove status: list workspace worktrees and branch state.
- grove sync: update local branch metadata and prune stale refs.
- grove config: print the effective TOML configuration.
- grove config --provenance: print field-level configuration sources.
- grove logs path: print the configured log file path.
- grove logs tail: print recent log lines.
- grove init --bash, --fish, or --zsh: generate shell integration functions.

Topic help pages:

- grove help workspace: workspace layout and requirements.
- grove help exit-codes: exit code and error conventions.
- grove help logs: log file location and troubleshooting notes.

## Configuration loading

Grove loads TOML config files from lowest to highest priority. Later files override earlier values.

1. XDG config: ${XDG_CONFIG_HOME}/grove/grove.toml, or ~/.config/grove/grove.toml.
2. Ancestor grove.toml files from the home directory toward the repository or workspace.
3. The main worktree grove.toml.
4. The current worktree grove.toml when it differs from the main worktree.
5. The current directory grove.toml when it differs from the worktree root.

Run grove config to inspect the merged effective configuration. Run grove config --provenance to see which file supplied each value.

## TOML schema

    [git]
    timeout = "5s"

    [github]
    preview_cache_ttl = "5m"

    [local_branch]
    branch_prefix = "feature/"
    strip_branch_prefix = ["feature/"]
    worktree_prefix = "wt-"

    [log]
    file = "~/.local/state/grove/grove.log"

    [pull_request]
    branch_template = "{{.BranchName}}"
    worktree_prefix = "pr-"

    [slugify]
    collapse_dashes = true
    hash_length = 4
    lowercase = true
    max_length = 50
    replace_non_alphanum = true
    trim_dashes = true

    [workspace]
    primary_branches = ["main", "develop", "master"]

## Validation notes

- git.timeout and github.preview_cache_ttl must be zero or positive durations.
- pull_request.worktree_prefix cannot be empty.
- slugify.hash_length and slugify.max_length must be zero or positive.
- slugify.hash_length must leave at least two characters when slugify.max_length is set.
- workspace.primary_branches must include at least one branch name.
`

func runDocs(cmd *cobra.Command, _ []string) error {
	_, err := io.WriteString(cmd.OutOrStdout(), docsMarkdown)
	return err
}
