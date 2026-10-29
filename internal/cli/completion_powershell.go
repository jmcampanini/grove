package cli

import "github.com/spf13/cobra"

func newCompletionPowerShellCmd() *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   "powershell",
		Short: "Generate the powershell completion script",
		Long: `Print the powershell completion script on stdout.

` + helpCompletion + "\n\n" + helpFailure,
		Example: `  # Load completions into the current shell.
  grove completion powershell | Out-String | Invoke-Expression

  # Generate a script.
  grove completion powershell
  grove completion powershell --no-descriptions`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if noDescriptions {
				return cmd.Root().GenPowerShellCompletion(cmd.OutOrStdout())
			}
			return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		},
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "Omit descriptions from completion suggestions")
	return cmd
}
