package cmd

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/jmcampanini/grove-cli/internal/logging"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "n/a"

var rootCmd = &cobra.Command{
	Use:   "grove",
	Short: "Git worktree workspace manager",
	Long: `Grove manages git worktrees in a workspace structure.

Common workflows:
  Start new work:       grove create "add user auth"
  Check out a branch:   grove checkout feature/fix-login
  Check out a PR:       grove pr checkout 42
  See all worktrees:    grove status`,
}

func init() {
	rootCmd.Version = Version
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")

	rootCmd.AddGroup(
		&cobra.Group{ID: "worktree", Title: "Worktree Commands:"},
		&cobra.Group{ID: "pr", Title: "Pull Request Commands:"},
		&cobra.Group{ID: "config", Title: "Configuration Commands:"},
		&cobra.Group{ID: "utility", Title: "Utility Commands:"},
	)
	rootCmd.SetHelpCommandGroupID("config")
	rootCmd.SetCompletionCommandGroupID("config")
	configureLogStyles()

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if debug, _ := cmd.Flags().GetBool("debug"); debug {
			log.SetLevel(log.DebugLevel)
		}
	}
}

func configureLogStyles() {
	styles := log.DefaultStyles()

	// Catppuccin Latte (light) / Mocha (dark) palette.
	muted := lipgloss.AdaptiveColor{Light: "#6c6f85", Dark: "#7f849c"}
	styles.Caller = lipgloss.NewStyle().Foreground(muted)
	styles.Key = lipgloss.NewStyle().Foreground(muted)
	styles.Prefix = lipgloss.NewStyle().Foreground(muted).Bold(true)
	styles.Separator = lipgloss.NewStyle().Foreground(muted)

	styles.Levels[log.DebugLevel] = styles.Levels[log.DebugLevel].
		Foreground(lipgloss.AdaptiveColor{Light: "#8c8fa1", Dark: "#7f849c"})
	styles.Levels[log.InfoLevel] = styles.Levels[log.InfoLevel].
		Foreground(lipgloss.AdaptiveColor{Light: "#179299", Dark: "#94e2d5"})
	styles.Levels[log.WarnLevel] = styles.Levels[log.WarnLevel].
		Foreground(lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"})
	styles.Levels[log.ErrorLevel] = styles.Levels[log.ErrorLevel].
		Foreground(lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"})
	styles.Levels[log.FatalLevel] = styles.Levels[log.FatalLevel].
		Foreground(lipgloss.AdaptiveColor{Light: "#8839ef", Dark: "#cba6f7"})

	log.SetStyles(styles)
}

// Execute runs the root command.
func Execute() error {
	defer logging.Close()
	return rootCmd.Execute()
}
