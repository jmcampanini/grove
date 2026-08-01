package cmd

import (
	"errors"

	"github.com/jmcampanini/grove-cli/internal/logging"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View grove log information",
	Long: `Grove writes file logs to a fixed path following the XDG Base Directory
Specification:

  $XDG_STATE_HOME/grove/grove.log
  ~/.local/state/grove/grove.log   (fallback when XDG_STATE_HOME is unset)

If grove cannot determine the fallback home directory or open the log file for
an invocation, it prints a warning to stderr and continues without file
logging. Standard output is unaffected.

Pass --debug on any grove command to raise the log level to debug for that
invocation.

Subcommands:
  grove logs path   Print the log file path
  grove logs tail   Print the last lines of the log file`,
}

func init() {
	logsCmd.GroupID = "config"
	rootCmd.AddCommand(logsCmd)
}

func resolveLogPath() (string, error) {
	p := logging.DefaultLogFilePath()
	if p == "" {
		return "", errors.New("could not determine log file path: user home directory is unavailable")
	}
	return p, nil
}
