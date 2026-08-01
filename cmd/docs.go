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
- grove issue start NUMBER: create a branch and worktree to work on an issue.
- grove status: list workspace worktrees and branch state.
- grove sync: update local branch metadata and prune stale refs.
- grove config: print the effective TOML configuration.
- grove config --provenance: print field-level configuration sources.

Topic help pages:

- grove help workspace: workspace layout and requirements.
- grove help exit-codes: exit code and error conventions.

## Configuration loading

Grove loads TOML config files from lowest to highest priority. Later files override earlier values.

1. XDG config: ${XDG_CONFIG_HOME}/grove/grove.toml, or ~/.config/grove/grove.toml.
2. Ancestor grove.toml files from the home directory toward the repository or workspace.
3. The main worktree grove.toml.
4. The current worktree grove.toml when it differs from the main worktree.
5. The current directory grove.toml when it differs from the worktree root.

Run grove config to inspect the merged effective configuration. Run grove config --provenance to see which file supplied each value.

Some values can also be set with global CLI flags, which take priority over all config files:

- --worktree-prefix: overrides local_branch.worktree_prefix.

## TOML schema

    [git]
    timeout = "5s"

    [github]
    preview_cache_ttl = "5m"

    [issue]
    branch_template = "issue/{{.Number}}-{{.TitleSlug}}"
    strip_branch_prefix = ["issue/"]
    title_slug_max_length = 40
    worktree_prefix = "is-"

    [local_branch]
    branch_prefix = "feature/"
    strip_branch_prefix = ["feature/"]
    worktree_prefix = "wt-"

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

## External command safety

- git.timeout applies to both git and gh commands. Zero means no configured deadline; commands remain cancellable when Grove is interrupted.
- Captured stdout and stderr are each limited to 8 MiB. Exceeding either limit stops the command and returns an error without echoing the captured content.
- Git and GitHub commands disable interactive credential prompts.

## Validation notes

- git.timeout and github.preview_cache_ttl must be zero or positive durations.
- issue.branch_template must reference {{.Number}}; issue matching is anchored on the issue number.
- issue.strip_branch_prefix removes the first matching prefix before generating a worktree directory name.
- issue.title_slug_max_length must be zero or positive; the slug is truncated cleanly with no hash suffix.
- issue.worktree_prefix cannot be empty.
- pull_request.worktree_prefix cannot be empty.
- slugify.hash_length and slugify.max_length must be zero or positive.
- slugify.hash_length must leave at least two characters when slugify.max_length is set.
- workspace.primary_branches must include at least one branch name.

## Logging

Grove appends ANSI-free logs for every invocation to a fixed path:

    $XDG_STATE_HOME/grove/grove.log
    ~/.local/state/grove/grove.log   (fallback when XDG_STATE_HOME is unset)

Pass --debug on any command to raise the log level to debug for that invocation.

If grove cannot determine the home directory or open the log file, it prints a
warning to stderr and continues without file logging. Standard output is
unaffected.

Inspect the file with standard tools:

    tail -n 50 ~/.local/state/grove/grove.log
    tail -f ~/.local/state/grove/grove.log
`

func runDocs(cmd *cobra.Command, _ []string) error {
	_, err := io.WriteString(cmd.OutOrStdout(), docsMarkdown)
	return err
}
