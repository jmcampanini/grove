package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/246859/tail"
	"github.com/spf13/cobra"
)

var logsTailLinesFlag int

var logsTailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Print the last lines of the log file",
	Args:  cobra.NoArgs,
	RunE:  runLogsTail,
}

func init() {
	logsTailCmd.Flags().IntVarP(&logsTailLinesFlag, "lines", "n", 25, "Number of lines to display")
	logsCmd.AddCommand(logsTailCmd)
}

func runLogsTail(cmd *cobra.Command, _ []string) error {
	if err := validateTailLines(logsTailLinesFlag); err != nil {
		return err
	}

	p, err := resolveLogPath()
	if err != nil {
		return err
	}

	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("log file does not exist: %s", p)
		}
		return fmt.Errorf("could not open log file %s: %w", p, err)
	}
	defer func() { _ = f.Close() }()

	data, err := readTail(f, logsTailLinesFlag)
	if err != nil {
		return err
	}

	_, err = cmd.OutOrStdout().Write(data)
	return err
}

func validateTailLines(n int) error {
	if n <= 0 {
		return fmt.Errorf("--lines must be a positive integer, got %d", n)
	}
	if n > 10000 {
		return fmt.Errorf("--lines must be at most 10000, got %d", n)
	}
	return nil
}

func readTail(f *os.File, n int) ([]byte, error) {
	// tail.Tail treats \n as a separator, not a terminator, so a file ending
	// in \n has an empty trailing segment that consumes one of the N lines.
	// Over-fetch by one and trim back to exactly n lines.
	data, err := tail.Tail(f, n+1)
	if err != nil {
		return nil, fmt.Errorf("reading log file tail: %w", err)
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return bytes.Join(lines, nil), nil
}
