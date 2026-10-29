package cli

import "github.com/spf13/cobra"

func newCompletionZshCmd() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "zsh",
		Short: "Generate the zsh completion script",
		Long: `Print the zsh completion script on stdout.

Enable shell completion before loading the script into the current shell.

` + helpCompletion + "\n\n" + helpFailure,
		Example: `  # Enable and load completions into the current shell.
  autoload -U compinit; compinit
  source <(grove completion zsh)

  # Generate a script.
  grove completion zsh
  grove completion zsh --no-descriptions`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if noDescriptions {
				return cmd.Root().GenZshCompletionNoDesc(cmd.OutOrStdout())
			}
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		},
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Omit descriptions from completion suggestions")
	return cmd
}
