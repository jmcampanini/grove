package cmd

import (
	"context"
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
	return executeLogsPath(cmd.Context(), cmd.OutOrStdout())
}

func executeLogsPath(ctx context.Context, w io.Writer) error {
	p, err := resolveLogPath(ctx)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, p)
	return err
}
