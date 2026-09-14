package cmd

import (
	"io"

	"github.com/spf13/cobra"
)

func newDocsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "docs",
		Short: "Print Grove reference documentation",
		Long: `Print a Markdown reference for Grove commands, configuration loading, and
the TOML config schema. Command help is the canonical contract for every
command; this reference supplements it.`,
		Args:    cobra.NoArgs,
		GroupID: "utility",
		RunE:    runDocs,
	}
}

const docsMarkdown = `# grove reference

Grove manages git worktrees: one directory per branch, placed under a
configurable root. Generated command help remains the canonical reference for
flags and examples.

## Command reference

Common commands:

- grove create PHRASE: create a branch and worktree from a phrase.
- grove checkout BRANCH: switch to an existing branch worktree or create it.
- grove pr checkout NUMBER: check out a pull request as a worktree.
- grove issue start NUMBER: create a branch and worktree to work on an issue.
- grove status: list the repository's worktrees and branch state.
- grove sync: fetch and hard-reset the current branch to its upstream, discarding local changes after confirmation.
- grove config: print the effective TOML configuration.
- grove config --provenance: print field-level configuration sources.

Topic help pages:

- grove help layout: where worktrees are placed and how to change it.
- grove help exit-codes: exit code and error conventions.

## Configuration loading

Grove loads TOML config files from lowest to highest priority. Later files override earlier values.

1. XDG config: ${XDG_CONFIG_HOME}/grove/grove.toml, or ~/.config/grove/grove.toml.
2. Ancestor grove.toml files from the home directory toward the main worktree.
3. The main worktree grove.toml.
4. The current worktree grove.toml when it differs from the main worktree.
5. The current directory grove.toml when it differs from the worktree root.

XDG_CONFIG_HOME must be an absolute path. A relative value is ignored and the
default ~/.config location is used; it is never resolved against the current
directory.

Every discovered path is optional: a missing file or a directory at a
candidate path is skipped, and symbolic links are followed. A candidate that
exists but cannot be read or parsed fails with an error.

Outside a repository, grove config and grove namer fall back to defaults plus
the XDG, home, and ancestor files. Inside a repository, every command resolves
the same effective configuration from the same files. Steps 2 and 3 follow the
main worktree, so a worktree placed under worktree.root reads the same
repository files as the main worktree.

Run grove config to inspect the merged effective configuration. Run grove config --provenance to see which file supplied each value.

Some values can also be set with global CLI flags, which take priority over all config files:

- --space: overrides worktree.space.
- --worktree-template: overrides local_branch.worktree_template.

## TOML schema

    [git]
    timeout = "5s"

    [github]
    preview_cache_ttl = "5m"

    [issue]
    branch_template = "issue/{{.Number}}-{{.TitleSlug}}"
    worktree_template = "is-{{.Number}}-{{.TitleSlug}}"

    [local_branch]
    branch_template = "feature/{{.PhraseSlug}}"
    worktree_template = "wt-{{.BranchSlug}}"

    [naming]
    lowercase = true
    max_length = 30
    strip_prefixes = ["feature/", "fix/", "issue/"]

    [pull_request]
    branch_template = "{{.Branch}}"
    worktree_template = "pr-{{.Number}}-{{.TitleSlug}}"

    [worktree]
    layout = "{{.Space}}/{{.Host}}/{{.Owner}}/{{.Repo}}/{{.Name}}"
    root = "$CODE_DIR/.worktrees"
    space = "grove"

## Worktree placement

New worktrees are created at worktree.root joined with worktree.layout. Grove
runs from the primary worktree or any linked worktree; the primary worktree
can live anywhere. Run grove help layout for the full description.

worktree.root is a plain path. $VAR and ${VAR} expand from the environment and
a leading ~ expands to the home directory. A referenced variable that is unset
or empty fails the command that needs the path. The expanded root must be
absolute.

worktree.layout is a Go template rendered relative to the root:

| Variable | Value |
|---|---|
| {{.Space}} | worktree.space, or the --space flag |
| {{.Host}} | origin remote host, e.g. github.com |
| {{.Owner}} | origin remote owner; nested groups keep their slashes |
| {{.Repo}} | origin remote repository name without .git |
| {{.Name}} | the rendered worktree_template name |

Host, owner, and repository come from the URL of the remote named origin. A
layout that uses none of them works in a repository without an origin. The layout must use {{.Name}} so each worktree
has its own path, and the rendered layout must be relative and must not
contain "..".

worktree.space is one directory segment, default "grove". Launchers pass
--space claude, --space codex, or --space pi to keep their worktrees apart.
Space only decides where a new worktree goes; every command operates on all
worktrees of the repository. Worktrees created under an earlier layout stay
where they are and remain managed.

## Naming templates

Each flow has separate branch and worktree templates. Variables are available as follows:

| Flow | branch_template | worktree_template |
|---|---|---|
| local_branch | {{.PhraseSlug}} | {{.BranchSlug}} |
| issue | {{.Number}}, {{.TitleSlug}} | {{.Number}}, {{.TitleSlug}}, {{.BranchSlug}} |
| pull_request | {{.Number}}, {{.Branch}} | {{.Number}}, {{.TitleSlug}}, {{.BranchSlug}} |

{{.Number}} is the issue or pull request number. {{.Branch}} is the raw branch name and can contain a slash.

Every variable whose name ends in Slug uses the same safety normalization: each run of characters outside ASCII letters and digits becomes one dash, surrounding dashes are removed, and letters are lowercased when naming.lowercase is true. Before {{.BranchSlug}} is normalized, the first matching naming.strip_prefixes entry is removed. {{.PhraseSlug}} comes from the phrase passed to grove create, and {{.TitleSlug}} comes from the issue or pull request title.

Template literal text is rendered verbatim. Variables are normalized before rendering, and the rendered result is not slugged again. This preserves literal separators and casing exactly as written in the template.

naming.max_length caps each final rendered branch and worktree name. Zero disables the cap. Truncation removes runes from the end without splitting a rune, then removes trailing dashes. Pull request branch names are exempt from this cap because they must match the remote branch.

Put {{.Number}} early in a custom issue or pull request template when the number must survive truncation, because the cap removes content from the end. Issue branch generation fails if the cap removes the complete runtime issue number. Truncated names receive no hash suffix, so different inputs can collide; Grove reports the existing branch or worktree conflict instead of disambiguating it.

The grove namer slug command performs safety normalization only. It does not apply naming.max_length because that cap belongs to final branch and worktree generation.

## External command safety

- git.timeout applies to both git and gh commands. Zero means no configured deadline; commands remain cancellable when Grove is interrupted.
- Captured stdout and stderr are each limited to 8 MiB. Exceeding either limit stops the command and returns an error without echoing the captured content.
- Git and GitHub commands disable interactive credential prompts.

## Validation notes

- git.timeout and github.preview_cache_ttl must be zero or positive durations.
- All branch_template and worktree_template values must be non-empty. Commands that use a template parse it and reject unavailable variables when constructing the corresponding namer.
- issue.branch_template must directly render {{.Number}} because issue matching requires the complete issue number.
- After rendering and truncation, Grove rejects empty branch names, leading dashes, double dots, and control characters. Git reports additional invalid-ref edge cases during branch creation.
- Final worktree names must be non-empty, contain no slash or control character, not begin with a dash, and not equal to **.** or **..**.
- naming.max_length must be zero or positive.
- worktree.root and worktree.layout must be non-empty. The root is expanded and the layout is parsed when a command places a worktree.
- worktree.space must be one path segment: non-empty, no slash, not "." or "..", not starting with a dash, no control characters.

## Logging

Grove appends ANSI-free logs to a fixed path once the command line has been
validated; --help, --version, and rejected input do not create the file:

    $XDG_STATE_HOME/grove/grove.log
    ~/.local/state/grove/grove.log   (fallback when XDG_STATE_HOME is unset)

Grove uses the same diagnostic level for stderr and the log file:

- By default, Grove emits info, warning, and error diagnostics.
- Pass --debug to also emit debug diagnostics.
- Pass --quiet to emit only error diagnostics.

The --debug and --quiet flags are mutually exclusive. Neither flag changes
standard output or machine-readable formats. Command failures remain visible on
stderr in quiet mode.

If grove cannot determine the home directory or open the log file, it emits a
warning unless --quiet is active and continues without file logging. Standard
output is unaffected.

Inspect the file with standard tools:

    tail -n 50 ~/.local/state/grove/grove.log
    tail -f ~/.local/state/grove/grove.log
`

func runDocs(cmd *cobra.Command, _ []string) error {
	_, err := io.WriteString(cmd.OutOrStdout(), docsMarkdown)
	return err
}
