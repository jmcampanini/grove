package cmd

import "github.com/spf13/cobra"

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cache",
		Short:   "Manage the grove cache",
		GroupID: "config",
	}
	cmd.AddCommand(newCacheClearCmd())
	return cmd
}
