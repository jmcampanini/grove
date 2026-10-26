package cli

import "github.com/spf13/cobra"

func newPRCmd(services commandServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr",
		Short:   "Browse and check out GitHub pull requests",
		GroupID: "pr",
		// Args and RunE make the group a grammar boundary; see runHelpTopic.
		Args: cobra.NoArgs,
		RunE: runHelpTopic,
		Long: `Browse the open pull requests of the current repository and check one out
into a local worktree. Every subcommand runs gh, which must already be
authenticated. A bare grove pr prints this help and exits 0.

To start new local work that has no pull request yet, use grove create.`,
	}
	cmd.AddCommand(
		newPRCheckoutCmd(services),
		newPRListCmd(services),
		newPRPreviewCmd(services),
	)
	return cmd
}
