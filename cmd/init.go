package cmd

import (
	"fmt"

	"github.com/jmcampanini/grove-cli/internal/shell"
	"github.com/spf13/cobra"
)

var (
	initBashFlag bool
	initFishFlag bool
	initZshFlag  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate shell integration functions",
	Long: `Generate shell integration functions.

Usage:
  grove init --<shell>

Examples:
  Zsh:
    grove init --zsh > "${XDG_DATA_HOME:-$HOME/.local/share}/grove/init.zsh"
    # Then add to .zshrc:  source "${XDG_DATA_HOME:-$HOME/.local/share}/grove/init.zsh"

  Bash:
    grove init --bash > "${XDG_DATA_HOME:-$HOME/.local/share}/grove/init.bash"
    # Then add to .bashrc:  source "${XDG_DATA_HOME:-$HOME/.local/share}/grove/init.bash"

  Fish:
    grove init --fish > ~/.config/fish/conf.d/grove.fish
    # Fish sources conf.d automatically — no config changes needed.`,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initBashFlag, "bash", false, "Output bash shell functions")
	initCmd.Flags().BoolVar(&initFishFlag, "fish", false, "Output fish shell functions")
	initCmd.Flags().BoolVar(&initZshFlag, "zsh", false, "Output zsh shell functions")
	initCmd.MarkFlagsMutuallyExclusive("bash", "fish", "zsh")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	gen := shell.NewFunctionGenerator()

	var output string
	switch {
	case initBashFlag:
		output = gen.GenerateBash()
	case initFishFlag:
		output = gen.GenerateFish()
	case initZshFlag:
		output = gen.GenerateZsh()
	default:
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), cmd.Long)
		return fmt.Errorf("specify a shell flag: --bash, --fish, or --zsh")
	}

	_, err := fmt.Fprint(cmd.OutOrStdout(), output)
	return err
}
