package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

// numberArgs is the positional grammar of the commands that take one issue
// or pull request number: exactly one operand that is a positive integer.
// noun names the operand in the diagnostic, as in
// `invalid pull request number "abc"`. The check reads only the operand, so
// a bad number is rejected before any command work.
func numberArgs(noun string) cobra.PositionalArgs {
	return cobra.MatchAll(cobra.ExactArgs(1), func(_ *cobra.Command, args []string) error {
		if _, err := parseNumberOperand(args[0]); err != nil {
			return fmt.Errorf("invalid %s number %q", noun, args[0])
		}
		return nil
	})
}

// parseNumberOperand converts an operand numberArgs accepted.
func parseNumberOperand(operand string) (int, error) {
	n, err := strconv.Atoi(operand)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("number %d is not positive", n)
	}
	return n, nil
}
