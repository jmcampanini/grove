package cmd

import (
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

func TestMutuallyExclusiveFlagHelpText(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		for _, want := range []string{
			"Creation mode flags are mutually exclusive",
			"--from <ref>",
			"--from-remote-primary",
			"--reuse",
		} {
			assert.Contains(t, newCreateCmd().Long, want)
		}
	})
}

func TestExitCodesTopicPrintsSameHelpFromBothEntryPoints(t *testing.T) {
	direct, stderr, err := executeForTest("exit-codes")
	require.NoError(t, err, stderr)
	viaHelp, stderr, err := executeForTest("help", "exit-codes")
	require.NoError(t, err, stderr)

	assert.Equal(t, direct, viaHelp)
	for _, want := range []string{"\n  0  ", "\n  1  ", "$XDG_STATE_HOME/grove/grove.log"} {
		assert.Contains(t, direct, want)
	}
}

func TestEveryApplicationCommandHasWrappedLongHelp(t *testing.T) {
	var visit func(*cobra.Command)
	visit = func(command *cobra.Command) {
		if command.Name() == "help" || command.Name() == "completion" {
			return
		}
		path := command.CommandPath()
		assert.NotEmptyf(t, strings.TrimSpace(command.Long), "%s has no long help", path)
		for field, text := range map[string]string{"Long": command.Long, "Example": command.Example} {
			for i, line := range strings.Split(text, "\n") {
				assert.LessOrEqualf(t, len(line), 80, "%s %s line %d is %d columns, want at most 80: %q", path, field, i+1, len(line), line)
			}
		}
		for _, child := range command.Commands() {
			visit(child)
		}
	}
	visit(newTestRootCommand())
}
