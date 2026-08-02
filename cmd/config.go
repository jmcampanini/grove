package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	var provenance bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print current configuration in TOML format",
		Long: `Print the current effective configuration in TOML format.

This outputs the merged configuration (defaults with any user overrides applied).
The output can be redirected to a file to create a new configuration:

  grove config > grove.toml

Use --provenance to print the source that supplied each configuration value.`,
		Args:    cobra.NoArgs,
		GroupID: "config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfig(cmd, provenance)
		},
	}
	cmd.Flags().BoolVar(&provenance, "provenance", false, "Print field-level configuration provenance")
	cmd.Flags().BoolVar(&provenance, "sources", false, "Alias for --provenance")
	return cmd
}

func runConfig(cmd *cobra.Command, provenance bool) error {
	rt, err := loadCommandRuntime(cmd)
	if err != nil {
		return err
	}

	reporter := configreporter.New(rt.cfg, rt.configReport)
	if provenance {
		return writeConfigProvenance(cmd.OutOrStdout(), reporter.ProvenanceHeaders(), reporter.ProvenanceRows())
	}

	if err := reporter.WriteTOML(cmd.OutOrStdout()); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	return nil
}

func writeConfigProvenance(w io.Writer, headers []string, rows [][]string) error {
	if len(headers) > 0 {
		if err := writeTabSeparatedRow(w, headers); err != nil {
			return err
		}
	}

	for _, row := range rows {
		if err := writeTabSeparatedRow(w, row); err != nil {
			return err
		}
	}
	return nil
}

func writeTabSeparatedRow(w io.Writer, cells []string) error {
	_, err := fmt.Fprintln(w, strings.Join(cells, "\t"))
	return err
}
