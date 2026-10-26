package cli

import "github.com/spf13/cobra"

func newIssueCmd(services commandServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "issue",
		Short:   "Browse GitHub issues and start work on them",
		GroupID: "issue",
		// Args and RunE make the group a grammar boundary; see runHelpTopic.
		Args: cobra.NoArgs,
		RunE: runHelpTopic,
		Long: `Browse the open issues of the current repository and start work on one in
a local worktree. Every subcommand runs gh, which must already be
authenticated. A bare grove issue prints this help and exits 0.

Unlike a pull request, an issue has no branch to check out: starting work
on one creates a branch from the latest primary branch of the default
remote.`,
	}
	cmd.AddCommand(
		newIssueListCmd(services),
		newIssuePreviewCmd(services),
		newIssueStartCmd(services),
	)
	return cmd
}
