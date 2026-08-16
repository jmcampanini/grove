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
		Args:  cobra.NoArgs,
		RunE:  runCacheClear,
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
