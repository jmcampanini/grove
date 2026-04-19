package cmd

import "github.com/spf13/cobra"

var exitCodesTopicCmd = &cobra.Command{
	Use:   "exit-codes",
	Short: "Exit codes and error categories",
	Long: `Grove uses the conventional two-value exit scheme:

  0  success
  1  any error (bad arguments, git failures, config errors, I/O, etc.)

Error detail is reported on stderr. Run 'grove logs tail' to inspect the
most recent log output when an error is not self-explanatory.`,
}

func init() {
	rootCmd.AddCommand(exitCodesTopicCmd)
}
