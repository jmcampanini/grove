package cmd

import (
	"bytes"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print current configuration in TOML format",
	Long: `Print the current effective configuration in TOML format.

This outputs the merged configuration (defaults with any user overrides applied).
The output can be redirected to a file to create a new configuration:

  grove config > grove.toml`,
	Args: cobra.NoArgs,
	RunE: runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, _ []string) error {
	env, err := initFromEnv()
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(env.cfg); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	_, err = fmt.Fprint(cmd.OutOrStdout(), buf.String())
	return err
}
