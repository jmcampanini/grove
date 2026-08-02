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
	"github.com/spf13/pflag"
)

// Version is set at build time via ldflags.
var Version = "n/a"

const (
	diagnosticDebugFlag = "debug"
	diagnosticQuietFlag = "quiet"
)

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

	registerDiagnosticFlags(root.PersistentFlags())
	root.MarkFlagsMutuallyExclusive(diagnosticDebugFlag, diagnosticQuietFlag)
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

	root.PersistentPreRunE = applyDiagnosticLevel

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

func registerDiagnosticFlags(flags *pflag.FlagSet) {
	flags.Bool(diagnosticDebugFlag, false, "Set diagnostic log level to debug")
	flags.Bool(diagnosticQuietFlag, false, "Set diagnostic log level to error")
}

func resolveDiagnosticLevel(flags *pflag.FlagSet) (log.Level, error) {
	debug, err := flags.GetBool(diagnosticDebugFlag)
	if err != nil {
		return log.InfoLevel, err
	}
	quiet, err := flags.GetBool(diagnosticQuietFlag)
	if err != nil {
		return log.InfoLevel, err
	}
	if debug && quiet {
		return log.InfoLevel, errors.New("--debug and --quiet cannot be used together; choose one diagnostic level")
	}

	switch {
	case debug:
		return log.DebugLevel, nil
	case quiet:
		return log.ErrorLevel, nil
	default:
		return log.InfoLevel, nil
	}
}

func applyDiagnosticLevel(cmd *cobra.Command, _ []string) error {
	level, err := resolveDiagnosticLevel(cmd.Root().PersistentFlags())
	if err != nil {
		return err
	}
	log.SetLevel(level)
	return nil
}

func preparseDiagnosticLevel(args []string) log.Level {
	flags := pflag.NewFlagSet("diagnostic", pflag.ContinueOnError)
	flags.ParseErrorsAllowlist.UnknownFlags = true
	flags.Usage = func() {}
	registerDiagnosticFlags(flags)
	flags.BoolP("help", "h", false, "")
	_ = flags.Parse(args)
	level, err := resolveDiagnosticLevel(flags)
	if err != nil {
		return log.InfoLevel
	}
	return level
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

func executeRoot(root *cobra.Command, args []string) error {
	root.SetArgs(args)
	log.SetLevel(preparseDiagnosticLevel(args))
	configureLogStyles()

	if err := logging.Setup(); err != nil {
		log.Warn("failed to set up file logging", "error", err)
	}
	defer logging.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return root.ExecuteContext(ctx)
}

// Execute runs the root command.
func Execute() error {
	return executeRoot(newRootCmd(), os.Args[1:])
}
