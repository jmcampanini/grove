package cli

import "github.com/spf13/cobra"

func newCacheCmd(services commandServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cache",
		Short:   "Manage the grove cache",
		GroupID: "config",
		// Args and RunE make the group a grammar boundary; see runHelpTopic.
		Args: cobra.NoArgs,
		RunE: runHelpTopic,
		Long: `Manage the on-disk cache of gh command output that grove pr list, grove pr
preview, grove issue list, grove issue preview, and grove status read.

The cache lives in the user cache directory under grove/v1: on macOS
~/Library/Caches/grove/v1, on Linux $XDG_CACHE_HOME/grove/v1 or
~/.cache/grove/v1 when XDG_CACHE_HOME is unset. Entries expire after
github.preview_cache_ttl; zero disables caching. A bare grove cache prints
this help and exits 0.`,
	}
	cmd.AddCommand(newCacheClearCmd(services))
	return cmd
}
