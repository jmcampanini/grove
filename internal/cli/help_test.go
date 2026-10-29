package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/grove/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootHelpDocumentsLogging(t *testing.T) {
	root := newTestRootCommand()
	for _, want := range []string{
		"$XDG_STATE_HOME/grove/grove.log",
		"~/.local/state/grove/grove.log",
		"defaults to info",
		"--debug",
		"--quiet",
		"mutually exclusive",
		"do not change stdout",
	} {
		assert.Contains(t, root.Long, want)
	}
}

// TestRootHelpEndsWithGroupBlock checks the shape the documentation contract
// gives the root long description: it ends with one line per first-level
// command group naming its --help invocation, and the topics are listed.
func TestRootHelpEndsWithGroupBlock(t *testing.T) {
	root := newTestRootCommand()
	_, groups, found := strings.Cut(root.Long, "\nCommand groups:\n")
	require.True(t, found, "root long description must end with a Command groups block")

	var want []string
	for _, child := range root.Commands() {
		if child.HasSubCommands() {
			want = append(want, child.Name())
		}
	}
	lines := strings.Split(strings.TrimRight(groups, "\n"), "\n")
	require.Len(t, lines, len(want))
	for i, line := range lines {
		assert.Equal(t, want[i], strings.Fields(line)[0])
		assert.True(t, strings.HasSuffix(line, "grove "+want[i]+" --help"), line)
	}

	var out bytes.Buffer
	root.SetOut(&out)
	require.NoError(t, root.Help())
	assert.Contains(t, out.String(), "Help topics:")
	var topics int
	for _, child := range root.Commands() {
		if isHelpTopic(child) {
			topics++
			assert.Contains(t, out.String(), "\n  "+child.Name()+" ")
		}
	}
	assert.NotZero(t, topics, "the tree has help topics")
}

func TestMutuallyExclusiveFlagHelpText(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		for _, want := range []string{
			"Creation mode flags are mutually exclusive",
			"--from <ref>",
			"--from-remote-primary",
			"--reuse",
		} {
			assert.Contains(t, newCreateCmd(commandServices{loadRuntime: loadCommandRuntime}).Long, want)
		}
	})
}

// TestConfigHelpDocumentsEveryKey walks the configuration struct so a key
// added to it must be documented in config --help: each section appears as
// its TOML table header and each key as a "  key = <default>" line.
func TestConfigHelpDocumentsEveryKey(t *testing.T) {
	long := newConfigCmd(commandServices{loadRuntime: loadCommandRuntime}).Long
	sections := reflect.TypeFor[config.Config]()
	for i := range sections.NumField() {
		section := sections.Field(i)
		table := section.Tag.Get("toml")
		assert.Containsf(t, long, "\n  ["+table+"]\n", "section %s", table)
		for j := range section.Type.NumField() {
			key := section.Type.Field(j).Tag.Get("toml")
			assert.Containsf(t, long, "\n  "+key+" = ", "key %s.%s", table, key)
		}
	}
	assert.NotContains(t, long, "grove docs")
}

// TestEveryApplicationCommandMeetsTheHelpBaseline is the one help test the
// documentation contract requires: it walks a fresh tree and fails on a
// missing short help, a missing long description, a rendered help line over
// 80 columns (flag usage included), or a help topic whose direct invocation
// prints text other than its help.
func TestEveryApplicationCommandMeetsTheHelpBaseline(t *testing.T) {
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		path := cmd.CommandPath()
		assert.NotEmptyf(t, strings.TrimSpace(cmd.Short), "%s has no short help", path)
		assert.NotEmptyf(t, strings.TrimSpace(cmd.Long), "%s has no long help", path)

		// The help is rendered by a fresh execution, the way a person sees
		// it, so the measured lines include the flag usage Cobra generates.
		args := strings.Fields(path)[1:]
		help, stderr, err := executeForTest(append(args, "--help")...)
		require.NoError(t, err, stderr)
		assert.NotEmpty(t, help)
		for i, line := range strings.Split(help, "\n") {
			assert.LessOrEqualf(t, len([]rune(line)), 80, "%s help line %d is %d columns, want at most 80: %q", path, i+1, len([]rune(line)), line)
		}

		if isHelpTopic(cmd) {
			direct, stderr, err := executeForTest(args...)
			require.NoError(t, err, stderr)
			assert.Equalf(t, help, direct, "%s invoked directly must print its help", path)
			viaHelp, stderr, err := executeForTest(append([]string{"help"}, args...)...)
			require.NoError(t, err, stderr)
			assert.Equalf(t, help, viaHelp, "grove help %s must print the topic's help", cmd.Name())
		}

		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(newTestRootCommand())
}

func TestHelpPrintsCommandHelp(t *testing.T) {
	for _, path := range [][]string{nil, {"pr", "list"}, {"help"}} {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			want, _, err := executeForTest(append(path, "--help")...)
			require.NoError(t, err)

			got, stderr, err := executeForTest(append([]string{"help"}, path...)...)

			require.NoError(t, err)
			assert.Empty(t, stderr)
			assert.Equal(t, want, got)
		})
	}
}

func TestHelpRejectsUnknownCommandPaths(t *testing.T) {
	for _, args := range [][]string{
		{"help", "nope"},
		{"help", "pr", "nope"},
		{"help", "pr", "list", "nope"},
		{"help", "--", "--nope"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, err := executeForTest(args...)

			require.ErrorContains(t, err, "unknown command")
			assert.Equal(t, exitUsage, exitStatus(err))
			assert.Empty(t, stdout)
			assert.Empty(t, stderr)
		})
	}
}

func TestHelpCompletesCommandPaths(t *testing.T) {
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{"co"}, want: []string{"completion", "config"}},
		{args: []string{"he"}, want: []string{"help"}},
		{args: []string{"pr", ""}, want: []string{"checkout", "list", "preview"}},
		{args: []string{"pr", "list", ""}},
		{args: []string{"pr", "unknown", ""}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			stdout, _, err := executeForTest(append([]string{"__complete", "help"}, tt.args...)...)
			require.NoError(t, err)

			lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
			assert.Equal(t, ":4", lines[len(lines)-1], "help completion must disable file completion")
			var names []string
			for _, line := range lines[:len(lines)-1] {
				name, _, _ := strings.Cut(line, "\t")
				names = append(names, name)
			}
			assert.Equal(t, tt.want, names)
		})
	}
}

func TestHelpReturnsOutputErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"help", "pr", "list"}, {"completion"}, {"exit-codes"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stderr bytes.Buffer

			err := executeWithFileLogging(strings.NewReader(""), failingWriter{}, &stderr, args)

			require.ErrorIs(t, err, errWriteFailed)
			assert.Equal(t, exitFailure, exitStatus(err))
			assert.Empty(t, stderr.String(), "the entrypoint reports the returned failure once")
		})
	}
}
