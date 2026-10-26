package cli

import (
	"bytes"
	"strings"
	"testing"

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
	assert.Contains(t, out.String(), "Help Topics")
	for _, topic := range []string{"exit-codes", "layout", "naming"} {
		assert.Contains(t, out.String(), "\n  "+topic+" ")
	}
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

func TestConfigHelpDocumentsEveryKey(t *testing.T) {
	long := newConfigCmd(commandServices{loadRuntime: loadCommandRuntime}).Long
	for _, key := range []string{
		"[git]\n  timeout",
		"[github]\n  preview_cache_ttl",
		"[issue]\n  branch_template",
		"[issue]\n  branch_template = \"issue/{{.Number}}-{{.TitleSlug}}\"",
		"[local_branch]\n  branch_template = \"feature/{{.PhraseSlug}}\"",
		"[naming]\n  lowercase = true",
		"max_length = 30",
		"strip_prefixes = [\"feature/\", \"fix/\", \"issue/\"]",
		"[pull_request]\n  branch_template = \"{{.Branch}}\"",
		"worktree_template = \"pr-{{.Number}}-{{.TitleSlug}}\"",
		"[worktree]\n  layout = \"{{.Space}}/{{.Host}}/{{.Owner}}/{{.Repo}}/{{.Name}}\"",
		"root = \"$CODE_DIR/.worktrees\"",
		"space = \"grove\"",
	} {
		assert.Contains(t, long, key)
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
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			return
		}
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

func TestHelpTopicsAreMarked(t *testing.T) {
	var topics []string
	for _, child := range newTestRootCommand().Commands() {
		if isHelpTopic(child) {
			topics = append(topics, child.Name())
		}
	}
	assert.Equal(t, []string{"exit-codes", "layout", "naming"}, topics)
}
