package cmd

import (
	"context"
	"errors"
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

func newRootCmd() *cobra.Command {
	log.SetLevel(log.InfoLevel)

	root := &cobra.Command{
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
  See all worktrees:    grove status

Logs are appended to $XDG_STATE_HOME/grove/grove.log
(~/.local/state/grove/grove.log when XDG_STATE_HOME is unset).
Diagnostic logging defaults to info. Pass --debug for debug logging or --quiet
to show only errors. The flags are mutually exclusive and do not change stdout.`,
		Version: Version,
	}

	root.PersistentFlags().Bool("debug", false, "Set diagnostic log level to debug")
	root.PersistentFlags().Bool("quiet", false, "Set diagnostic log level to error")
	root.MarkFlagsMutuallyExclusive("debug", "quiet")
	cobra.CheckErr(config.RegisterFlags(root.PersistentFlags()))

	root.AddGroup(
		&cobra.Group{ID: "worktree", Title: "Worktree Commands:"},
		&cobra.Group{ID: "git", Title: "Git Commands:"},
		&cobra.Group{ID: "pr", Title: "Pull Request Commands:"},
		&cobra.Group{ID: "issue", Title: "Issue Commands:"},
		&cobra.Group{ID: "config", Title: "Configuration Commands:"},
		&cobra.Group{ID: "utility", Title: "Utility Commands:"},
	)
	root.SetHelpCommandGroupID("config")
	root.SetCompletionCommandGroupID("config")

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return applyDiagnosticLevel(cmd)
	}

	root.AddCommand(
		newCacheCmd(),
		newCatchupCmd(),
		newCheckoutCmd(),
		newConfigCmd(),
		newCreateCmd(),
		newDocsCmd(),
		newExitCodesTopicCmd(),
		newIssueCmd(),
		newListCmd(),
		newNamerCmd(),
		newPRCmd(),
		newPruneCmd(),
		newRemoveCmd(),
		newResolveCmd(),
		newStatusCmd(),
		newSyncCmd(),
		newWorkspaceTopicCmd(),
	)

	return root
}

func applyDiagnosticLevel(cmd *cobra.Command) error {
	flags := cmd.Root().PersistentFlags()
	debug, err := flags.GetBool("debug")
	if err != nil {
		return err
	}
	quiet, err := flags.GetBool("quiet")
	if err != nil {
		return err
	}
	if debug && quiet {
		return errors.New("--debug and --quiet cannot be used together; choose one diagnostic level")
	}

	switch {
	case debug:
		log.SetLevel(log.DebugLevel)
	case quiet:
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
	return nil
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

func executeRoot(root *cobra.Command) error {
	configureLogStyles()

	setupErr := logging.Setup()
	defer logging.Close()

	setupWarningPending := setupErr != nil
	reportSetupWarning := func() {
		if setupWarningPending {
			log.Warn("failed to set up file logging", "error", setupErr)
			setupWarningPending = false
		}
	}
	if setupWarningPending {
		preRun := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if preRun != nil {
				if err := preRun(cmd, args); err != nil {
					return err
				}
			}
			reportSetupWarning()
			return nil
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := root.ExecuteContext(ctx)

	if setupWarningPending {
		if levelErr := applyDiagnosticLevel(root); levelErr == nil {
			reportSetupWarning()
		}
	}
	return err
}

// Execute runs the root command.
func Execute() error {
	return executeRoot(newRootCmd())
}
