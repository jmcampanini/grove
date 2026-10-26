package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// handlePreviewError applies the --fzf presentation mode: a failure that
// occurs after the invocation was accepted is printed on stdout in the same
// "grove: <message>" form stderr would carry, and the command succeeds so
// the preview pane shows the message. Only a failure to write that line is
// returned. Outside --fzf the error is returned unchanged.
func handlePreviewError(cmd *cobra.Command, err error, fzf bool) error {
	if !fzf {
		return err
	}
	_, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "grove: %v\n", err)
	return writeErr
}
