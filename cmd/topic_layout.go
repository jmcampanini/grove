package cmd

import "github.com/spf13/cobra"

func newLayoutTopicCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "layout",
		Short: "Where worktrees are placed on disk",
		Args:  cobra.NoArgs,
		RunE:  runHelpTopic,
		Long: `Grove runs inside a git repository: the primary worktree (the clone) or
any linked worktree, at any depth. The primary worktree can live anywhere.
New worktrees are placed under worktree.root at the path rendered by
worktree.layout.

Defaults:

  [worktree]
  root = "$CODE_DIR/.worktrees"
  layout = "{{.Space}}/{{.Host}}/{{.Owner}}/{{.Repo}}/{{.Name}}"
  space = "grove"

Example with CODE_DIR=~/Code and the clone at ~/Code/github.com/acme/app:

  ~/Code/.worktrees/grove/github.com/acme/app/wt-add-auth
  ~/Code/.worktrees/grove/github.com/acme/app/pr-456-fix-login
  ~/Code/.worktrees/claude/github.com/acme/app/wt-run-tests

Root: a plain path. $VAR and ${VAR} expand from the environment and a
leading ~ expands to the home directory. A referenced variable that is
unset or empty fails the command; set it or configure a root that does not
use it. The expanded root must be absolute.

Layout: a Go template rendered relative to the root. Variables:

  {{.Space}}  the space: worktree.space, or the --space flag
  {{.Host}}   the default remote's host, e.g. github.com
  {{.Owner}}  the default remote's owner; nested groups keep their slashes
  {{.Repo}}   the default remote's repository name without .git
  {{.Name}}   the worktree name from the branch, issue, or pull request
              worktree_template

Host, owner, and repository come from the URL of the default remote
(remote.pushDefault, otherwise origin) and require a network remote. A
layout that uses none of them works in a repository without remotes. The
rendered layout must be relative and must not contain "..".

Space: one directory segment that partitions worktrees by who created them.
It defaults to "grove"; launchers pass --space claude, --space codex, or
--space pi so their worktrees sit apart from yours. Space only decides
where a new worktree goes. Every command still operates on all worktrees of
the repository, wherever they are.

Worktrees created before a layout change stay where they are; git tracks
them, so grove keeps listing, syncing, and pruning them.`,
	}
}
