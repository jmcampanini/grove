package cli

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// Exit statuses. Every failure that is not a usage error exits exitFailure;
// the exit-codes help topic documents both.
const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

// usageError marks a failure Cobra raised while validating the invocation:
// an unknown flag or flag value, a positional grammar failure, or flags that
// cannot be combined. Execute maps it to exitUsage.
type usageError struct {
	err error
}

func (e usageError) Error() string { return e.err.Error() }

func (e usageError) Unwrap() error { return e.err }

// markUsageErrors routes every invocation failure Cobra reports through
// usageError: flag parse errors via the flag error func, and operand grammar
// failures by wrapping the Args validator of every command below root.
// Cobra's own root-level unknown command error is untyped and is classified
// by text in exitStatus.
func markUsageErrors(root *cobra.Command) {
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError{err: err}
	})
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			if validate := child.Args; validate != nil {
				child.Args = func(cmd *cobra.Command, args []string) error {
					if err := validate(cmd, args); err != nil {
						return usageError{err: err}
					}
					return nil
				}
			}
			visit(child)
		}
	}
	visit(root)
}

// unknownCommandText matches the untyped unknown-command error Cobra
// reports from command discovery at the root. The command path after
// "for" is deliberately not anchored.
var unknownCommandText = regexp.MustCompile(`^unknown command ".*" for "`)

// exitStatus maps the error an execution returned to the process exit
// status.
func exitStatus(err error) int {
	if err == nil {
		return exitSuccess
	}
	var usage usageError
	if errors.As(err, &usage) || unknownCommandText.MatchString(err.Error()) {
		return exitUsage
	}
	return exitFailure
}

var lineBreaks = regexp.MustCompile(`\s*\n\s*`)

// reportFailure writes the one diagnostic line a failure produces, in the
// form "grove: <message>". Line breaks inside the message, such as the
// stderr of a failed git command, are folded into spaces so the diagnostic
// stays one line.
func reportFailure(w io.Writer, err error) {
	message := lineBreaks.ReplaceAllString(strings.TrimSpace(err.Error()), " ")
	_, _ = fmt.Fprintf(w, "grove: %s\n", message)
}
