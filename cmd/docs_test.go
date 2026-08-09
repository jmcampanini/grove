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
	assert.Contains(t, output, "--worktree-template: overrides local_branch.worktree_template")
	assert.Contains(t, output, "[naming]\n    lowercase = true\n    max_length = 30\n    strip_prefixes")
	assert.Contains(t, output, `branch_template = "feature/{{.PhraseSlug}}"`)
	assert.Contains(t, output, `worktree_template = "is-{{.Number}}-{{.TitleSlug}}"`)
	assert.Contains(t, output, `worktree_template = "pr-{{.Number}}-{{.TitleSlug}}"`)
	assert.Contains(t, output, "| local_branch | {{.PhraseSlug}} | {{.BranchSlug}} |")
	assert.Contains(t, output, "| pull_request | {{.Number}}, {{.Branch}} | {{.Number}}, {{.TitleSlug}}, {{.BranchSlug}} |")
	assert.Contains(t, output, "rendered result is not slugged again")
	assert.Contains(t, output, "without splitting a rune")
	assert.Contains(t, output, "Pull request branch names are exempt")
	assert.Contains(t, output, "Put {{.Number}} early")
	assert.Contains(t, output, "receive no hash suffix")
	assert.Contains(t, output, "does not apply naming.max_length")
	assert.Contains(t, output, "Zero means no configured deadline")
	assert.Contains(t, output, "each limited to 8 MiB")
}

func TestDocsCommandMetadata(t *testing.T) {
	cmd := newDocsCmd()
	assert.Equal(t, "utility", cmd.GroupID)
	assert.Equal(t, "docs", cmd.Use)
}
