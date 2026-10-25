package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func handlePreviewError(cmd *cobra.Command, err error, fzf bool) error {
	if fzf {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Error: %v\n", err)
		return nil
	}
	return err
}
