package cmd

import (
	"fmt"

	"github.com/jmcampanini/grove-cli/internal/cache"
	"github.com/spf13/cobra"
)

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Remove all cached data",
	RunE:  runCacheClear,
}

func init() {
	cacheCmd.AddCommand(cacheClearCmd)
}

func runCacheClear(cmd *cobra.Command, _ []string) error {
	dir, err := cache.DefaultDir()
	if err != nil {
		return err
	}

	c := cache.New(dir, 0)
	if err := c.Clear(); err != nil {
		return err
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cache cleared.")
	return nil
}
