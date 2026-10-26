package cli

import (
	"errors"
	"io"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/jmcampanini/grove/internal/logging"
	"github.com/jmcampanini/grove/internal/render"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// commandLogger builds the diagnostic logger for one command execution,
// writing to the command's stderr at the level selected by the diagnostic
// flags and with the color profile the --color mode resolves for stderr.
func commandLogger(cmd *cobra.Command) *log.Logger {
	level, err := resolveDiagnosticLevel(cmd.Root().PersistentFlags())
	if err != nil {
		level = log.InfoLevel
	}
	return newDiagnosticLogger(cmd.ErrOrStderr(), level, colorMode(cmd))
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

// newDiagnosticLogger builds a logger on w. The profile is the one resolved
// for the terminal behind a Tee, and otherwise the one mode resolves for w
// itself, so the logger never runs its own detection.
func newDiagnosticLogger(w io.Writer, level log.Level, mode render.ColorMode) *log.Logger {
	logger := log.NewWithOptions(w, log.Options{Level: level, ReportTimestamp: true})
	profile := render.ResolveProfile(mode, w)
	if tee, ok := w.(*logging.Tee); ok {
		profile = tee.Profile
	}
	logger.SetColorProfile(profile)
	logger.SetStyles(diagnosticLogStyles())
	return logger
}

func logHasDarkBackground() bool {
	if dark, ok := render.DetectDarkBackgroundFromEnv(); ok {
		return dark
	}
	return true
}

func diagnosticLogStyles() *log.Styles {
	styles := log.DefaultStyles()

	// Catppuccin Latte (light) / Mocha (dark) palette.
	hasDarkBackground := logHasDarkBackground()
	muted := render.LightDarkColor(hasDarkBackground, "#6c6f85", "#7f849c")
	styles.Caller = lipgloss.NewStyle().Foreground(muted)
	styles.Key = lipgloss.NewStyle().Foreground(muted)
	styles.Prefix = lipgloss.NewStyle().Foreground(muted).Bold(true)
	styles.Separator = lipgloss.NewStyle().Foreground(muted)

	styles.Levels[log.DebugLevel] = styles.Levels[log.DebugLevel].
		Foreground(render.LightDarkColor(hasDarkBackground, "#8c8fa1", "#7f849c"))
	styles.Levels[log.InfoLevel] = styles.Levels[log.InfoLevel].
		Foreground(render.LightDarkColor(hasDarkBackground, "#179299", "#94e2d5"))
	styles.Levels[log.WarnLevel] = styles.Levels[log.WarnLevel].
		Foreground(render.LightDarkColor(hasDarkBackground, "#df8e1d", "#f9e2af"))
	styles.Levels[log.ErrorLevel] = styles.Levels[log.ErrorLevel].
		Foreground(render.LightDarkColor(hasDarkBackground, "#d20f39", "#f38ba8"))
	styles.Levels[log.FatalLevel] = styles.Levels[log.FatalLevel].
		Foreground(render.LightDarkColor(hasDarkBackground, "#8839ef", "#cba6f7"))

	return styles
}
