package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/logging"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "n/a"

var rootCmd = &cobra.Command{
	Use:           "grove",
	Short:         "Git worktree workspace manager",
	SilenceErrors: true,
	SilenceUsage:  true,
	Long: `Grove manages git worktrees in a workspace structure.

Common workflows:
  Start new work:       grove create "add user auth"
  Check out a branch:   grove checkout feature/fix-login
  Check out a PR:       grove pr checkout 42
  Work on an issue:     grove issue start 17
  See all worktrees:    grove status`,
}

func init() {
	rootCmd.Version = Version
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	cobra.CheckErr(config.RegisterFlags(rootCmd.PersistentFlags()))

	rootCmd.AddGroup(
		&cobra.Group{ID: "worktree", Title: "Worktree Commands:"},
		&cobra.Group{ID: "git", Title: "Git Commands:"},
		&cobra.Group{ID: "pr", Title: "Pull Request Commands:"},
		&cobra.Group{ID: "issue", Title: "Issue Commands:"},
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

func logHasDarkBackground() bool {
	if dark, ok := detectDarkBackgroundFromEnv(); ok {
		return dark
	}
	return true
}

func configureLogStyles() {
	styles := log.DefaultStyles()

	// Catppuccin Latte (light) / Mocha (dark) palette.
	hasDarkBackground := logHasDarkBackground()
	muted := lightDarkColor(hasDarkBackground, "#6c6f85", "#7f849c")
	styles.Caller = lipgloss.NewStyle().Foreground(muted)
	styles.Key = lipgloss.NewStyle().Foreground(muted)
	styles.Prefix = lipgloss.NewStyle().Foreground(muted).Bold(true)
	styles.Separator = lipgloss.NewStyle().Foreground(muted)

	styles.Levels[log.DebugLevel] = styles.Levels[log.DebugLevel].
		Foreground(lightDarkColor(hasDarkBackground, "#8c8fa1", "#7f849c"))
	styles.Levels[log.InfoLevel] = styles.Levels[log.InfoLevel].
		Foreground(lightDarkColor(hasDarkBackground, "#179299", "#94e2d5"))
	styles.Levels[log.WarnLevel] = styles.Levels[log.WarnLevel].
		Foreground(lightDarkColor(hasDarkBackground, "#df8e1d", "#f9e2af"))
	styles.Levels[log.ErrorLevel] = styles.Levels[log.ErrorLevel].
		Foreground(lightDarkColor(hasDarkBackground, "#d20f39", "#f38ba8"))
	styles.Levels[log.FatalLevel] = styles.Levels[log.FatalLevel].
		Foreground(lightDarkColor(hasDarkBackground, "#8839ef", "#cba6f7"))

	log.SetStyles(styles)
}

// Execute runs the root command.
func Execute() error {
	if err := logging.Setup(); err != nil {
		log.Warn("failed to set up file logging", "error", err)
	}
	defer logging.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return rootCmd.ExecuteContext(ctx)
}
