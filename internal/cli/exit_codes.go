package cli

import "github.com/spf13/cobra"

func newExitCodesTopicCmd(_ commandServices) *cobra.Command {
	return newHelpTopicCmd("exit-codes", "Exit statuses and the failure line",
		`Grove exits with one of these statuses:

  0  success
  1  any failure that is not a usage error: git or gh failed, the
     configuration is invalid, a path or branch already exists, a prompt
     cannot run, a file cannot be read or written
  2  usage error: an unknown command, flag, or operand, a missing or extra
     operand, an operand that fails its grammar (such as a pull request
     number that is not a positive integer), an invalid flag value, or
     flags that cannot be combined (such as --debug with --quiet)

A failure prints exactly one line on stderr, without usage text:

  grove: <message>

When the message is not self-explanatory, inspect the log file at
$XDG_STATE_HOME/grove/grove.log (~/.local/state/grove/grove.log when
XDG_STATE_HOME is unset).

These outcomes keep status 0:

  - grove pr preview --fzf and grove issue preview --fzf print a failure
    that occurs after the invocation was accepted as "grove: <message>"
    on stdout, so an fzf preview pane shows it. A rejected invocation
    still exits 2, and a failure to write that line exits 1.
  - Declining a prompt: grove sync prints "Sync aborted." and grove prune
    prints "Aborted." or nothing, both on stdout.
  - A bare command group (grove pr, grove issue, grove cache, grove namer)
    prints its help.
  - grove help <unknown> prints "Unknown help topic" and the root usage on
    stdout.`)
}
