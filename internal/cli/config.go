package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/spf13/cobra"
)

func newConfigCmd(_ commandServices) *cobra.Command {
	var provenance bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print current configuration in TOML format",
		Long: `Print the effective configuration as TOML on stdout after applying every
layer. Nothing is modified.

Values load in this order, and a later layer replaces any value an earlier
one sets:

  1. Built-in defaults.
  2. $XDG_CONFIG_HOME/grove/config.toml, or ~/.config/grove/config.toml
     when XDG_CONFIG_HOME is unset or not an absolute path. A relative value
     is ignored, never resolved against the current directory.
  3. grove.toml in each directory from the home directory down to the parent
     of the main worktree, when the main worktree is under the home
     directory. The current directory is skipped here and loaded at step 6.
  4. grove.toml in the main worktree root.
  5. grove.toml in the current worktree root, when it differs from the main
     worktree.
  6. grove.toml in the current directory, when it differs from the worktree
     root.
  7. The global --space and --worktree-template flags, which set
     worktree.space and local_branch.worktree_template when given.

Each file is loaded once. No environment variable sets a configuration
value; XDG_CONFIG_HOME only changes where step 2 looks. Every file is
optional: a missing path or a directory at a candidate path is skipped, and
symbolic links are followed. A file that exists but cannot be read or
parsed, or that contains a key grove does not recognize, fails the command.
The merged result is validated after the last layer; an invalid result
fails the command. grove docs lists the schema and validation rules.

Outside a repository, this command still works: it loads the defaults, the
step 2 file, and grove.toml in each directory from the home directory down
to the current directory. grove namer uses the same fallback. Inside a
repository, every command loads the same files in the same order. Steps 3
and 4 follow the main worktree, so a worktree placed under worktree.root
reads the same repository files as the main worktree; only step 5 is
specific to it.

By default the output is the merged TOML and nothing else; it reloads as a
grove.toml:

  grove config > grove.toml

--provenance (alias --sources) appends field-level provenance after the
TOML: a blank line, a "# Provenance" line, then a tab-separated table with
Path, Value, and Source columns and one row per field, every line prefixed
with "# ". The output stays valid TOML and still reloads. Source is
<default>, the absolute path of the file that last set the value, or
<pflag> for a flag. Nothing is redacted; the configuration holds no secret
fields.`,
		Args:    cobra.NoArgs,
		GroupID: "config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfig(cmd, provenance)
		},
	}
	cmd.Flags().BoolVar(&provenance, "provenance", false, "Append field-level configuration provenance as TOML comments")
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
	if err := reporter.WriteTOML(cmd.OutOrStdout()); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if provenance {
		return writeConfigProvenance(cmd.OutOrStdout(), reporter.ProvenanceHeaders(), reporter.ProvenanceRows())
	}
	return nil
}

// writeConfigProvenance appends the provenance table as TOML comment lines so
// the report that precedes it stays loadable.
func writeConfigProvenance(w io.Writer, headers []string, rows [][]string) error {
	if _, err := fmt.Fprintln(w, "\n# Provenance"); err != nil {
		return err
	}
	for _, cells := range append([][]string{headers}, rows...) {
		if _, err := fmt.Fprintf(w, "# %s\n", strings.Join(cells, "\t")); err != nil {
			return err
		}
	}
	return nil
}
