package logging

import (
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

var logFile *os.File

type teeWriter struct {
	file     io.Writer
	terminal io.Writer
}

func (t *teeWriter) Write(p []byte) (n int, err error) {
	n, err = t.terminal.Write(p)
	if err != nil {
		return n, err
	}
	_, _ = t.file.Write([]byte(ansi.Strip(string(p[:n]))))
	return n, nil
}

// Setup tees the default logger's output to both stderr and the given file path.
// The color profile is captured before swapping the output so that stderr
// retains its native color support after the tee is installed.
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

	profile := termenv.NewOutput(os.Stderr).ColorProfile()
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
