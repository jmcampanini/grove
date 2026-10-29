package cli

import (
	"fmt"
	"strings"

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
gh output cache described in grove cache --help before querying GitHub; a
miss stores the fresh gh output there.`

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

// helpTopicContract states the stream contract every help topic shares.
const helpTopicContract = `This topic prints on stdout and needs no repository, terminal, or network.`

func newHelpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "help [command...]",
		Short: "Show help for a command or topic",
		Long: `Print command or topic help on stdout. With no command, print grove's
root help. A command path names each level with a separate word.
Unknown command paths are usage errors.

Help is never styled, never prompts, and needs no repository or network.
It does not load configuration or create the log file.

` + helpFailure,
		Example: `  grove help
  grove help pr list
  grove help exit-codes`,
		GroupID: "config",
		Args: func(cmd *cobra.Command, args []string) error {
			_, err := helpTarget(cmd.Root(), args)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := helpTarget(cmd.Root(), args)
			if err != nil {
				return err
			}
			target.InitDefaultHelpFlag()
			return target.Help()
		},
		ValidArgsFunction: completeHelpCommand,
	}
}

func completeHelpCommand(cmd *cobra.Command, args []string, prefix string) ([]cobra.Completion, cobra.ShellCompDirective) {
	target, err := helpTarget(cmd.Root(), args)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var completions []cobra.Completion
	for _, child := range target.Commands() {
		if (child.IsAvailableCommand() || child == cmd) && strings.HasPrefix(child.Name(), prefix) {
			completions = append(completions, cobra.CompletionWithDesc(child.Name(), child.Short))
		}
	}
	return completions, cobra.ShellCompDirectiveNoFileComp
}

func helpTarget(root *cobra.Command, args []string) (*cobra.Command, error) {
	target := root
	for _, name := range args {
		var match *cobra.Command
		for _, child := range target.Commands() {
			if child.Name() == name || child.HasAlias(name) {
				match = child
				break
			}
		}
		if match == nil {
			return nil, fmt.Errorf("unknown command %q for %q", name, target.CommandPath())
		}
		target = match
	}
	return target, nil
}

// newHelpTopicCmd builds a help topic: an application-owned command that
// does no command work and prints its help whether invoked directly or
// through grove help.
func newHelpTopicCmd(use, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:         use,
		Short:       short,
		Long:        long + "\n\n" + helpTopicContract,
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
