package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Version is set at build time via ldflags.
var Version = "n/a"

const (
	diagnosticDebugFlag = "debug"
	diagnosticQuietFlag = "quiet"
)

// NewRootCommand builds a fresh grove command tree wired to the given
// streams. Every execution gets its own tree; no state is shared across
// constructions.
func NewRootCommand(in io.Reader, out, errOut io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:   "grove",
		Short: "Git worktree workspace manager",
		// Groups never suggest corrections for an unknown subcommand, so the
		// root does not either; an unknown command reports only its name.
		DisableSuggestions: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		Long: `Grove manages git worktrees in a workspace structure.

Common workflows:
  Start new work:       grove create "add user auth"
  Check out a branch:   grove checkout feature/fix-login
  Check out a PR:       grove pr checkout 42
  Work on an issue:     grove issue start 17
  See all worktrees:    grove status

Configuration comes from grove.toml files and the --worktree-template flag;
run grove config --help for the file locations and precedence.

Logs are appended to $XDG_STATE_HOME/grove/grove.log
(~/.local/state/grove/grove.log when XDG_STATE_HOME is unset). Logging starts
after argument validation: --help, --version, and rejected input do not create
the log file. Diagnostic logging defaults to info. Pass --debug for debug
logging or --quiet to show only errors. The flags are mutually exclusive and
do not change stdout.`,
		Version: Version,
	}
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)

	registerDiagnosticFlags(root.PersistentFlags())
	root.MarkFlagsMutuallyExclusive(diagnosticDebugFlag, diagnosticQuietFlag)
	if err := config.RegisterFlags(root.PersistentFlags()); err != nil {
		panic(err)
	}
	// Cobra's Find strips flags before Execute registers --help and
	// --version, so without these a value flag after either one is read as
	// an operand and "grove --help --worktree-template x" fails with unknown
	// command "x". Registering them here keeps that ordering correct on a
	// root that has no Args validator.
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()

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

// commandLogger builds the diagnostic logger for one command execution,
// writing to the command's stderr at the level selected by the diagnostic
// flags.
func commandLogger(cmd *cobra.Command) *log.Logger {
	level, err := resolveDiagnosticLevel(cmd.Root().PersistentFlags())
	if err != nil {
		level = log.InfoLevel
	}
	return newDiagnosticLogger(cmd.ErrOrStderr(), level)
}

func newDiagnosticLogger(w io.Writer, level log.Level) *log.Logger {
	logger := log.NewWithOptions(w, log.Options{Level: level, ReportTimestamp: true})
	if tee, ok := w.(*logging.Tee); ok {
		logger.SetColorProfile(tee.Profile)
	}
	logger.SetStyles(diagnosticLogStyles())
	return logger
}

func logHasDarkBackground() bool {
	if dark, ok := detectDarkBackgroundFromEnv(); ok {
		return dark
	}
	return true
}

func diagnosticLogStyles() *log.Styles {
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

	return styles
}

func executeRoot(root *cobra.Command, args []string) error {
	root.SetArgs(args)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return root.ExecuteContext(ctx)
}

// executeWithFileLogging builds a fresh tree on the given streams and runs
// it, teeing diagnostics (and everything else written to stderr) into the
// grove log file. The log file is opened in the root's persistent pre-run,
// after Cobra has parsed flags, handled --help and --version, and validated
// operands, so rejected input never creates the state directory or the log.
// When the log file cannot be opened, a warning is logged and execution
// proceeds with stderr alone.
func executeWithFileLogging(in io.Reader, out, stderr io.Writer, args []string) error {
	root := NewRootCommand(in, out, stderr)

	var tee *logging.Tee
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// Cobra checks flag groups after the persistent pre-run; check them
		// first so conflicting --debug/--quiet never opens the log file.
		if err := cmd.ValidateFlagGroups(); err != nil {
			return err
		}

		opened, err := logging.NewTee(stderr)
		if err != nil {
			commandLogger(cmd).Warn("failed to set up file logging", "error", err)
			return nil
		}
		tee = opened
		cmd.Root().SetErr(tee)
		return nil
	}
	defer func() {
		if tee != nil {
			_ = tee.Close()
		}
	}()

	return executeRoot(root, args)
}

// Execute runs the root command on the process streams.
func Execute() error {
	return executeWithFileLogging(os.Stdin, os.Stdout, os.Stderr, os.Args[1:])
}
