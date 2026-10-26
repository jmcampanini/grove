package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/spf13/cobra"
)

func newConfigCmd(_ commandServices) *cobra.Command {
	var provenance bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Print current configuration in TOML format",
		Long: `Print the effective configuration as TOML on stdout after applying every
layer. Nothing is modified, nothing prompts, and the output is never
styled, whatever --color says.

Values load in this order, and a later layer replaces any value an earlier
one sets:

  1. Built-in defaults.
  2. $XDG_CONFIG_HOME/grove/config.toml, or ~/.config/grove/config.toml
     when XDG_CONFIG_HOME is unset, empty, or not an absolute path. A
     relative value is ignored, never resolved against the current directory.
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
fails the command.

Outside a repository, this command still works: it loads the defaults, the
step 2 file, and grove.toml in each directory from the home directory down
to the current directory. grove namer uses the same fallback. Inside a
repository, every command loads the same files in the same order. Steps 3
and 4 follow the main worktree, so a worktree placed under worktree.root
reads the same repository files as the main worktree; only step 5 is
specific to it.

By default the output is the merged TOML and nothing else; it reloads as a
grove.toml.

--provenance (alias --sources) appends field-level provenance after the
TOML: a blank line, a "# Provenance" line, then a tab-separated table with
Path, Value, and Source columns and one row per field, every line prefixed
with "# ". The output stays valid TOML and still reloads. Source is
<default>, the absolute path of the file that last set the value, or
<pflag> for a flag. Nothing is redacted; the configuration holds no secret
fields.

Keys, shown with their defaults:

  [git]
  timeout = "5s"
    Deadline for each git and gh command; zero or positive. Zero sets no
    deadline, but interrupting grove still cancels the command. Captured
    stdout and stderr are each capped at 8 MiB, and both programs run with
    their interactive prompts disabled.

  [github]
  preview_cache_ttl = "5m"
    How long cached gh output stays valid for pr list, pr preview, issue
    list, issue preview, and status; zero or positive. Zero disables the
    cache. Run grove cache --help for the directory.

  [issue]
  branch_template = "issue/{{.Number}}-{{.TitleSlug}}"
    Branch name for grove issue start; non-empty, and it must render
    {{.Number}} directly because issue matching needs the whole number.
  worktree_template = "is-{{.Number}}-{{.TitleSlug}}"
    Worktree directory name for grove issue start; non-empty.

  [local_branch]
  branch_template = "feature/{{.PhraseSlug}}"
    Branch name for grove create; non-empty.
  worktree_template = "wt-{{.BranchSlug}}"
    Worktree directory name for grove create and grove checkout;
    non-empty. The --worktree-template flag replaces it for one invocation.

  [naming]
  lowercase = true
    Lowercase every slug variable.
  max_length = 30
    Cap on each rendered branch and worktree name; zero or positive. Zero
    disables the cap.
  strip_prefixes = ["feature/", "fix/", "issue/"]
    Prefixes removed from a branch name before it becomes {{.BranchSlug}};
    the first match wins.

  [pull_request]
  branch_template = "{{.Branch}}"
    Local branch name for grove pr checkout; non-empty and exempt from
    naming.max_length so it matches the remote branch.
  worktree_template = "pr-{{.Number}}-{{.TitleSlug}}"
    Worktree directory name for grove pr checkout; non-empty.

  [worktree]
  layout = "{{.Space}}/{{.Host}}/{{.Owner}}/{{.Repo}}/{{.Name}}"
    Go template rendered relative to root; non-empty, must use {{.Name}},
    and must render a relative path without "..".
  root = "$CODE_DIR/.worktrees"
    Directory that holds new worktrees; non-empty. $VAR, ${VAR}, and a
    leading ~ expand when a command places a worktree, an unset or empty
    variable fails that command, and the result must be absolute.
  space = "grove"
    One path segment that partitions worktrees by creator: non-empty, no
    slash, not "." or "..", not starting with "-", no control characters.
    The --space flag replaces it for one invocation.

The template variables of each flow and the slug rules are in grove help
naming; the layout variables are in grove help layout. A template is
parsed when a command uses it, and one that names a variable its flow does
not provide fails that command. A rendered branch name must be non-empty
and must not start with "-", contain "..", or contain a control character;
a rendered worktree name follows the space rules above.`,
		Example: `  grove config > grove.toml
  grove config --provenance`,
		Args:    cobra.NoArgs,
		GroupID: "config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfig(cmd, provenance)
		},
	}
	cmd.Flags().BoolVar(&provenance, "provenance", false, "Append the source of each value as TOML comments")
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
		escaped := make([]string, len(cells))
		for i, cell := range cells {
			escaped[i] = commentCell(cell)
		}
		if _, err := fmt.Fprintf(w, "# %s\n", strings.Join(escaped, "\t")); err != nil {
			return err
		}
	}
	return nil
}

// commentCell escapes the bytes a TOML comment cannot carry (control
// characters and invalid UTF-8), so a row stays one comment line whatever a
// discovered file path contains. Ordinary paths pass through unchanged.
func commentCell(cell string) string {
	var b strings.Builder
	for i := 0; i < len(cell); {
		r, size := utf8.DecodeRuneInString(cell[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02x`, cell[i])
		case r < 0x20 || r == 0x7f:
			quoted := strconv.QuoteRune(r)
			b.WriteString(quoted[1 : len(quoted)-1])
		default:
			b.WriteRune(r)
		}
		i += size
	}
	return b.String()
}
