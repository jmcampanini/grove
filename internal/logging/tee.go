package logging

import (
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

// Setup tees the default logger's output to both stderr and the given file
// path. If filePath is empty, Setup is a no-op. If a previous log file is
// open, it is closed before the new one is opened. The color profile is
// captured before swapping the output so that stderr retains its native color
// support after the tee is installed.
func Setup(filePath string) error {
	if filePath == "" {
		return nil
	}

	Close()

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
