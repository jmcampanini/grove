package cmd

import "github.com/spf13/cobra"

func newPRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr",
		Short:   "Browse and check out GitHub pull requests",
		GroupID: "pr",
		Long: `Browse and check out GitHub pull requests into local worktrees.

Subcommands:
  checkout  Check out a pull request into a local worktree
  list      List open pull requests
  preview   Preview a pull request

To start new local work (not from a PR), use 'grove create' instead.`,
	}
	cmd.AddCommand(
		newPRCheckoutCmd(),
		newPRListCmd(),
		newPRPreviewCmd(),
	)
	return cmd
}
