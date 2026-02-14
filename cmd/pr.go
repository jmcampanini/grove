package cmd

import "github.com/spf13/cobra"

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Browse and check out GitHub pull requests",
	Long: `Browse and check out GitHub pull requests into local worktrees.

Subcommands:
  create    Check out a pull request into a local worktree
  list      List open pull requests
  preview   Preview a pull request

To start new local work (not from a PR), use 'grove create' instead.`,
}

func init() {
	prCmd.GroupID = "pr"
	rootCmd.AddCommand(prCmd)
}
