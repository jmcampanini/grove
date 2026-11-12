package cli

import "github.com/spf13/cobra"

func newIssueCmd(services commandServices) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "issue",
		Short:   "Start work on GitHub issues",
		GroupID: "issue",
		// Args and RunE make the group a grammar boundary; see runHelpTopic.
		Args: cobra.NoArgs,
		RunE: runHelpTopic,
		Long: `Start work on an issue of the current repository in a local worktree.
Every subcommand runs gh, which must already be authenticated. A bare
grove issue prints this help and exits 0.

Unlike a pull request, an issue has no branch to check out: starting work
on one creates a branch from the latest primary branch of the default
remote.`,
	}
	cmd.AddCommand(newIssueStartCmd(services))
	return cmd
}
