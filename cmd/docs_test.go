package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocsCommandWritesReference(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	err := runDocs(cmd, nil)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "# grove reference")
	assert.Contains(t, output, "grove config --provenance")
	assert.Contains(t, output, "## Logging")
	assert.Contains(t, output, "$XDG_STATE_HOME/grove/grove.log")
	assert.Contains(t, output, "By default, Grove emits info, warning, and error diagnostics.")
	assert.Contains(t, output, "Pass --debug to also emit debug diagnostics.")
	assert.Contains(t, output, "Pass --quiet to emit only error diagnostics.")
	assert.Contains(t, output, "Command failures remain visible on")
	assert.Contains(t, output, "[workspace]")
	assert.Contains(t, output, `strip_branch_prefix = ["issue/"]`)
	assert.Contains(t, output, `worktree_prefix = "is-"`)
	assert.Contains(t, output, "Zero means no configured deadline")
	assert.Contains(t, output, "each limited to 8 MiB")
}

func TestDocsCommandMetadata(t *testing.T) {
	cmd := newDocsCmd()
	assert.Equal(t, "utility", cmd.GroupID)
	assert.Equal(t, "docs", cmd.Use)
}
