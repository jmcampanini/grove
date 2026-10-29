package cli

import "github.com/spf13/cobra"

func newCompletionFishCmd() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "fish",
		Short: "Generate the fish completion script",
		Long: `Print the fish completion script on stdout.

` + helpCompletion + "\n\n" + helpFailure,
		Example: `  # Load completions into the current shell.
  grove completion fish | source

  # Generate a script.
  grove completion fish
  grove completion fish --no-descriptions`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), !noDescriptions)
		},
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Omit descriptions from completion suggestions")
	return cmd
}
