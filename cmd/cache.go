package cmd

import "github.com/spf13/cobra"

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cache",
		Short:   "Manage the grove cache",
		GroupID: "config",
		Long: `Manage the on-disk cache of gh command output that grove pr list, grove pr
preview, grove issue list, grove issue preview, and grove status read.

The cache lives in the user cache directory under grove/v1: on macOS
~/Library/Caches/grove/v1, on Linux $XDG_CACHE_HOME/grove/v1 or
~/.cache/grove/v1 when XDG_CACHE_HOME is unset. Entries expire after
github.preview_cache_ttl; zero disables caching.

Subcommands:
  clear  Remove all cached data`,
	}
	cmd.AddCommand(newCacheClearCmd())
	return cmd
}
