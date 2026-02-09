package cmd

import "github.com/spf13/cobra"

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Manage pull request worktrees",
}

func init() {
	rootCmd.AddCommand(prCmd)
}
