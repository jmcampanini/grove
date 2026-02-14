package cmd

import (
	"fmt"

	"github.com/jmcampanini/grove-cli/internal/shell"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init <shell>",
	Short: "Generate shell integration functions",
	Long: `Init outputs shell functions for integration with your shell.

Add to your shell config:
  Fish:  grove init fish | source
  Zsh:   eval "$(grove init zsh)"
  Bash:  eval "$(grove init bash)"`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"fish", "zsh", "bash"},
	RunE:      runInit,
}

func init() {
	initCmd.GroupID = "config"
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	shellName := args[0]
	gen := shell.NewFunctionGenerator()

	var output string
	switch shellName {
	case "fish":
		output = gen.GenerateFish()
	case "zsh":
		output = gen.GenerateZsh()
	case "bash":
		output = gen.GenerateBash()
	default:
		return fmt.Errorf("unsupported shell: %s (supported: fish, zsh, bash)", shellName)
	}

	_, err := fmt.Fprint(cmd.OutOrStdout(), output)
	return err
}
