package cli

import "github.com/spf13/cobra"

func newNamingTopicCmd(_ commandServices) *cobra.Command {
	return newHelpTopicCmd("naming", "How branch and worktree names are rendered",
		`Each flow renders a branch name and a worktree directory name from its own
pair of Go templates in the configuration; run grove config --help for the
keys, their defaults, and their constraints. The variables each template
may use, written as {{.Name}}:

  Flow          branch_template     worktree_template
  local_branch  PhraseSlug          BranchSlug
  issue         Number, TitleSlug   Number, TitleSlug, BranchSlug
  pull_request  Number, Branch      Number, TitleSlug, BranchSlug

Number is the issue or pull request number. Branch is the pull request's
head branch as named on GitHub and may contain a slash. PhraseSlug comes
from the phrase given to grove create, TitleSlug from the issue or pull
request title, and BranchSlug from the branch name after the first
matching naming.strip_prefixes entry is removed.

Every variable ending in Slug is normalized the same way: each run of
characters outside ASCII letters and digits becomes one dash, leading and
trailing dashes are removed, and letters are lowercased when
naming.lowercase is true. Literal text in a template is kept as written,
and the rendered result is not normalized again, so the separators and
casing in a template survive.

naming.max_length caps each rendered branch and worktree name; zero
disables the cap. Truncation removes runes from the end without splitting
one, then removes trailing dashes. Pull request branch names are exempt
because they must match the remote branch. Put {{.Number}} early in a
custom issue or pull request template so it survives truncation; issue
branch generation fails when the cap removes any part of the number.
Truncated names get no hash suffix, so different inputs can collide, and
grove then reports the existing branch or worktree instead of
disambiguating.

The namer commands print the names these rules produce for a phrase
without creating anything; run grove namer --help.`)
}
