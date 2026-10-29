package cli

import "github.com/spf13/cobra"

func newCompletionBashCmd() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "bash",
		Short: "Generate the bash completion script",
		Long: `Print the bash completion script on stdout.

Install bash-completion using your package manager before loading the
script into the current shell.

` + helpCompletion + "\n\n" + helpFailure,
		Example: `  # Load completions into the current shell.
  source <(grove completion bash)

  # Generate a script.
  grove completion bash
  grove completion bash --no-descriptions`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), !noDescriptions)
		},
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Omit descriptions from completion suggestions")
	return cmd
}
