package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	var provenance bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print current configuration in TOML format",
		Long: `Print the effective configuration as TOML on stdout after applying every
layer. Nothing is modified.

Values load in this order, and a later layer replaces any value an earlier
one sets:

  1. Built-in defaults.
  2. $XDG_CONFIG_HOME/grove/grove.toml, or ~/.config/grove/grove.toml when
     XDG_CONFIG_HOME is unset or not an absolute path. A relative value is
     ignored, never resolved against the current directory.
  3. grove.toml in each directory from the home directory down to the parent
     of the main worktree, when the main worktree is under the home
     directory. The current directory is skipped here and loaded at step 6.
  4. grove.toml in the main worktree root.
  5. grove.toml in the current worktree root, when it differs from the main
     worktree.
  6. grove.toml in the current directory, when it differs from the worktree
     root.
  7. The global --worktree-template flag, which sets
     local_branch.worktree_template when given.

Each file is loaded once. No environment variable sets a configuration
value; XDG_CONFIG_HOME only changes where step 2 looks. Every file is
optional: a missing path or a directory at a candidate path is skipped, and
symbolic links are followed. A file that exists but cannot be read or
parsed, or that contains a key grove does not recognize, fails the command.
The merged result is validated after the last layer; an invalid result
fails the command. grove docs lists the schema and validation rules.

From a workspace root, the main worktree is the child directory named after
the first matching workspace.primary_branches entry. Outside a repository or
workspace, this command still works: it loads the defaults, the step 2 file,
and grove.toml in each directory from the home directory down to the
current directory. grove namer and grove resolve use the same fallback.
Inside a repository, every command loads the same files in the same order.

The output is the merged TOML and nothing else; it reloads as a grove.toml:

  grove config > grove.toml

--provenance (alias --sources) prints a tab-separated table instead, with
Path, Value, and Source columns and one row per field. Source is <default>,
the absolute path of the file that last set the value, or <pflag> for the
flag. Nothing is redacted; the configuration holds no secret fields.`,
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
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	cfg, report, err := loadReportingConfig(cmd, cwd)
	if err != nil {
		return err
	}

	reporter := configreporter.New(cfg, report)
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
