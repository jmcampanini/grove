package cmd

import "github.com/spf13/cobra"

// Version is set at build time via ldflags.
var Version = "n/a"

var rootCmd = &cobra.Command{
	Use:   "grove",
	Short: "Git worktree workspace manager",
	Long: `Grove manages git worktrees in a workspace structure.

Common workflows:
  Start new work:       grove create "add user auth"
  Check out a PR:       grove pr create 42
  See all worktrees:    grove status`,
}

func init() {
	rootCmd.Version = Version

	rootCmd.AddGroup(
		&cobra.Group{ID: "worktree", Title: "Worktree Commands:"},
		&cobra.Group{ID: "pr", Title: "Pull Request Commands:"},
		&cobra.Group{ID: "config", Title: "Configuration Commands:"},
	)
	rootCmd.SetHelpCommandGroupID("config")
	rootCmd.SetCompletionCommandGroupID("config")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
