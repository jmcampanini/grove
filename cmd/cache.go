package cmd

import "github.com/spf13/cobra"

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the grove cache",
}

func init() {
	cacheCmd.GroupID = "config"
	rootCmd.AddCommand(cacheCmd)
}
