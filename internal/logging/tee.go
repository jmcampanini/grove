package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

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

// Tee forwards diagnostic writes to a terminal writer while appending an
// ANSI-stripped copy to the grove log file.
type Tee struct {
	// Profile is the color profile of the terminal writer, captured at
	// construction so loggers writing through the tee can keep the
	// terminal's native color support.
	Profile colorprofile.Profile

	file *os.File
	tee  *teeWriter
}

func (t *Tee) Write(p []byte) (int, error) {
	return t.tee.Write(p)
}

// Close releases the log file.
func (t *Tee) Close() error {
	return t.file.Close()
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

// NewTee opens the default log file for appending and returns a Tee that
// forwards writes to terminal while copying them, ANSI-stripped, to the file.
func NewTee(terminal io.Writer) (*Tee, error) {
	filePath := DefaultLogFilePath()
	if filePath == "" {
		return nil, errors.New("could not determine log file path: user home directory is unavailable")
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}

	return &Tee{
		Profile: colorprofile.Detect(terminal, os.Environ()),
		file:    f,
		tee:     &teeWriter{file: f, terminal: terminal},
	}, nil
}
