package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

var logFile *os.File

type teeWriter struct {
	file          io.Writer
	fileErrLogged bool
	terminal      io.Writer
}

func (t *teeWriter) Write(p []byte) (n int, err error) {
	n, err = t.terminal.Write(p)

	stripped := []byte(ansi.Strip(string(p)))
	if _, fileErr := t.file.Write(stripped); fileErr != nil && !t.fileErrLogged {
		t.fileErrLogged = true
		_, _ = fmt.Fprintf(t.terminal, "WARN file logging failed: %v\n", fileErr)
	}

	return n, err
}

// DefaultLogFilePath returns the log file path following the XDG Base
// Directory Specification. It uses $XDG_STATE_HOME/grove/grove.log, falling
// back to ~/.local/state/grove/grove.log. It returns an empty string if the
// home directory cannot be determined.
func DefaultLogFilePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "grove", "grove.log")
}

// Setup tees the default logger's output to stderr and the fixed log file. If
// a previous log file is open, it is closed before the new one is opened. The
// color profile is captured before swapping the output so that stderr retains
// its native color support after the tee is installed.
func Setup() error {
	filePath := DefaultLogFilePath()
	if filePath == "" {
		return errors.New("could not determine log file path: user home directory is unavailable")
	}

	Close()

	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		return err
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	logFile = f

	profile := colorprofile.Detect(os.Stderr, os.Environ())
	log.SetOutput(&teeWriter{
		terminal: os.Stderr,
		file:     f,
	})
	log.SetColorProfile(profile)

	return nil
}

// Close releases the log file opened by Setup, if any, and restores the
// default logger's output to stderr.
func Close() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
		log.SetOutput(os.Stderr)
	}
}
