package cmd

import (
	"fmt"

	"github.com/jmcampanini/grove/internal/cache"
	"github.com/spf13/cobra"
)

func newCacheClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove all cached data",
		Long: `Remove every entry in the grove cache directory and print "Cache cleared."
on stdout. A missing cache directory is not an error. When an entry cannot
be removed, the remaining entries are still tried and the command fails
with exit status 1.`,
		Args: cobra.NoArgs,
		RunE: runCacheClear,
	}
}

func runCacheClear(cmd *cobra.Command, _ []string) error {
	dir, err := cache.DefaultDir()
	if err != nil {
		return err
	}

	c := cache.New(dir, 0, commandLogger(cmd))
	if err := c.Clear(); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cache cleared.")
	return nil
}
