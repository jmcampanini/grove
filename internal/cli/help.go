package cli

import (
	"github.com/spf13/cobra"
)

// helpTopicAnnotation marks a command built by newHelpTopicCmd, so tests
// find every help topic without a hand-kept list.
const helpTopicAnnotation = "grove.help-topic"

// Shared contract text, written once and composed into the long
// descriptions of the commands that share it.
const (
	// helpFailure states the stderr and exit status contract every
	// application command shares.
	helpFailure = `Diagnostics go to stderr. A failure prints one line there as
"grove: <message>" and exits nonzero; run grove help exit-codes for the
statuses.`

	// helpWorktreeCreate states where the commands that create a worktree
	// put it.
	helpWorktreeCreate = `The worktree directory is created under worktree.root at the path
worktree.layout renders (run grove help layout); the global --space flag
selects the space segment for one invocation. Nothing is created when a
worktree for the branch already exists.`

	// helpGH states the gh dependency and the cache the read-only GitHub
	// commands consult.
	helpGH = `The command runs gh, which must already be authenticated, and reads the
gh output cache described in grove cache --help before querying GitHub.`

	// helpGHUncached states the gh dependency of the commands that bypass
	// the cache.
	helpGHUncached = `The command runs gh, which must already be authenticated, and always
queries GitHub; it does not read the gh output cache.`

	// helpFzfPreview states the preview presentation mode shared by the pr
	// and issue preview commands.
	helpFzfPreview = `--fzf is the presentation mode for an fzf preview pane. It changes only
how a failure is shown: once the invocation is accepted, an error such as
an unknown number or a failed gh call is printed on stdout as
"grove: <message>" and the command exits 0, so the pane shows it. A
rejected invocation still exits 2.`
)

// newHelpTopicCmd builds a help topic: an application-owned command that
// does no command work and prints its help whether invoked directly or
// through grove help.
func newHelpTopicCmd(use, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:         use,
		Short:       short,
		Long:        long,
		Annotations: map[string]string{helpTopicAnnotation: "true"},
		Args:        cobra.NoArgs,
		GroupID:     "topics",
		// A RunE makes Cobra validate Args instead of printing help for a
		// non-runnable command before it looks at the operands.
		RunE: runHelpTopic,
	}
}

func isHelpTopic(cmd *cobra.Command) bool {
	return cmd.Annotations[helpTopicAnnotation] == "true"
}

func runHelpTopic(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
