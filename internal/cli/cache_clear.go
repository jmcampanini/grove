package cli

import (
	"fmt"

	"github.com/jmcampanini/grove/internal/cache"
	"github.com/spf13/cobra"
)

func newCacheClearCmd(_ commandServices) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove all cached data",
		Long: `Delete every entry under the grove cache directory (run grove cache --help
for its location) and print "Cache cleared." on stdout. The directory
itself is kept, and a missing directory is not an error. When an entry
cannot be removed, the remaining entries are still tried and the command
fails. Nothing prompts.

` + helpFailure,
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

	_, err = fmt.Fprintln(cmd.OutOrStdout(), "Cache cleared.")
	return err
}
