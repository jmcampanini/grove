package cmd

import "github.com/spf13/cobra"

func newExitCodesTopicCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "exit-codes",
		Short: "Exit codes and error categories",
		Args:  cobra.NoArgs,
		// Help topics need a RunE so Cobra validates Args instead of
		// silently ignoring operands on non-runnable commands.
		RunE: runHelpTopic,
		Long: `Grove uses the conventional two-value exit scheme:

  0  success
  1  any error (bad arguments, git failures, config errors, I/O, etc.)

Error detail is reported on stderr. When an error is not self-explanatory,
inspect the log file at $XDG_STATE_HOME/grove/grove.log
(~/.local/state/grove/grove.log when XDG_STATE_HOME is unset).`,
	}
}

func runHelpTopic(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
