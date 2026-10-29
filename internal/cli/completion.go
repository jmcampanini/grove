package cli

import "github.com/spf13/cobra"

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate a shell completion script",
		Long: `Print a shell completion script on stdout. Choose bash, zsh, fish, or
powershell; run grove completion <shell> --help for loading instructions.
With no subcommand, print this help on stdout.

The command needs no repository, terminal, configuration, or network.

` + helpFailure,
		Example: `  grove completion bash
  grove completion zsh --no-descriptions`,
		Args:                  cobra.NoArgs,
		GroupID:               "config",
		RunE:                  runHelpTopic,
		ValidArgsFunction:     cobra.NoFileCompletions,
		DisableFlagsInUseLine: true,
	}
	cmd.AddCommand(newCompletionBashCmd(), newCompletionZshCmd(), newCompletionFishCmd(), newCompletionPowerShellCmd())
	return cmd
}

const helpCompletion = `--no-descriptions omits descriptions from completion suggestions. The
command needs no repository, terminal, configuration, or network.`
