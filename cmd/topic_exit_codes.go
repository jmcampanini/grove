package cmd

import "github.com/spf13/cobra"

var exitCodesTopicCmd = &cobra.Command{
	Use:   "exit-codes",
	Short: "Exit codes and error categories",
	Long: `Grove uses the conventional two-value exit scheme:

  0  success
  1  any error (bad arguments, git failures, config errors, I/O, etc.)

Error detail is reported on stderr. When an error is not self-explanatory,
inspect the log file at $XDG_STATE_HOME/grove/grove.log
(~/.local/state/grove/grove.log when XDG_STATE_HOME is unset).`,
}

func init() {
	rootCmd.AddCommand(exitCodesTopicCmd)
}
