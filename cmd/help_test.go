package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootHelpDocumentsLogging(t *testing.T) {
	root := newRootCmd()
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
