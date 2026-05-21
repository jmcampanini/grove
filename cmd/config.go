package cmd

import (
	"fmt"
	"io"

	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/spf13/cobra"
)

var configProvenance bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print current configuration in TOML format",
	Long: `Print the current effective configuration in TOML format.

This outputs the merged configuration (defaults with any user overrides applied).
The output can be redirected to a file to create a new configuration:

  grove config > grove.toml

Use --provenance to print the source that supplied each configuration value.`,
	Args: cobra.NoArgs,
	RunE: runConfig,
}

func init() {
	configCmd.GroupID = "config"
	configCmd.Flags().BoolVar(&configProvenance, "provenance", false, "Print field-level configuration provenance")
	configCmd.Flags().BoolVar(&configProvenance, "sources", false, "Alias for --provenance")
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, _ []string) error {
	rt, err := loadCommandRuntime()
	if err != nil {
		return err
	}

	reporter := configreporter.New(rt.cfg, rt.configReport)
	if configProvenance {
		return writeConfigProvenance(cmd.OutOrStdout(), reporter.ProvenanceHeaders(), reporter.ProvenanceRows())
	}

	if err := reporter.WriteTOML(cmd.OutOrStdout()); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	return nil
}

func writeConfigProvenance(w io.Writer, headers []string, rows [][]string) error {
	if len(headers) > 0 {
		for i, header := range headers {
			if i > 0 {
				if _, err := fmt.Fprint(w, "\t"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprint(w, header); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				if _, err := fmt.Fprint(w, "\t"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprint(w, cell); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}
