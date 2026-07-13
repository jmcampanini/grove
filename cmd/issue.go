package cmd

import "github.com/spf13/cobra"

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Browse GitHub issues and start work on them",
	Long: `Browse GitHub issues and start work on them in local worktrees.

Subcommands:
  start    Create a branch and worktree to work on an issue
  list     List open issues
  preview  Preview an issue

Unlike a pull request, an issue has no branch to check out; 'grove issue start'
creates a new branch from the latest primary branch of the default remote.`,
}

func init() {
	issueCmd.GroupID = "issue"
	rootCmd.AddCommand(issueCmd)
}
