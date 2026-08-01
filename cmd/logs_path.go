package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var logsPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the log file path",
	Args:  cobra.NoArgs,
	RunE:  runLogsPath,
}

func init() {
	logsCmd.AddCommand(logsPathCmd)
}

func runLogsPath(cmd *cobra.Command, _ []string) error {
	return executeLogsPath(cmd.OutOrStdout())
}

func executeLogsPath(w io.Writer) error {
	p, err := resolveLogPath()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, p)
	return err
}
