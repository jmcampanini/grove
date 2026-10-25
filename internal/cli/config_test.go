package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/grove/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigCommandHasProvenanceFlags(t *testing.T) {
	cmd := newConfigCmd(commandServices{loadRuntime: loadCommandRuntime})
	assert.NotNil(t, cmd.Flags().Lookup("provenance"))
	assert.NotNil(t, cmd.Flags().Lookup("sources"))
}

func TestWriteConfigProvenance(t *testing.T) {
	var buf bytes.Buffer
	err := writeConfigProvenance(&buf, []string{"Path", "Source"}, [][]string{
		{"git.timeout", "<default>"},
		{"github.preview_cache_ttl", "/tmp/grove.toml"},
	})
	require.NoError(t, err)

	assert.Equal(t, "Path\tSource\ngit.timeout\t<default>\ngithub.preview_cache_ttl\t/tmp/grove.toml\n", buf.String())
}

func TestConfigCommandOutsideRepoRoundTrips(t *testing.T) {
	isolatedConfigHome(t, "[naming]\nmax_length = 42\n")
	workDir := tempDirResolved(t)
	t.Chdir(workDir)

	var out, errOut bytes.Buffer
	root := NewRootCommand(strings.NewReader(""), &out, &errOut)
	root.SetArgs([]string{"config"})
	require.NoError(t, root.ExecuteContext(context.Background()), errOut.String())

	roundTripPath := filepath.Join(workDir, "roundtrip.toml")
	require.NoError(t, os.WriteFile(roundTripPath, out.Bytes(), 0o644))
	reloaded, _, err := config.Load([]string{roundTripPath}, nil)
	require.NoError(t, err)

	want, _, err := loadReportingConfig(newTestRoot(), workDir)
	require.NoError(t, err)
	assert.Equal(t, want, reloaded, "config output should round-trip to the effective config")
	assert.Equal(t, 42, reloaded.Naming.MaxLength)
}

func TestConfigCommandProvenanceNamesSourceFile(t *testing.T) {
	configPath := isolatedConfigHome(t, "[naming]\nmax_length = 42\n")
	workDir := tempDirResolved(t)
	t.Chdir(workDir)

	var out, errOut bytes.Buffer
	root := NewRootCommand(strings.NewReader(""), &out, &errOut)
	root.SetArgs([]string{"config", "--provenance"})
	require.NoError(t, root.ExecuteContext(context.Background()), errOut.String())

	assert.Contains(t, out.String(), configPath)
}

func TestConfigCommandSpaceFlagOverridesFile(t *testing.T) {
	isolatedConfigHome(t, "[worktree]\nspace = \"codex\"\n")
	workDir := tempDirResolved(t)
	t.Chdir(workDir)

	var out, errOut bytes.Buffer
	root := NewRootCommand(strings.NewReader(""), &out, &errOut)
	root.SetArgs([]string{"config", "--space", "claude"})
	require.NoError(t, root.ExecuteContext(context.Background()), errOut.String())

	assert.Contains(t, out.String(), `space = "claude"`)
}

func TestConfigCommandRejectsInvalidSpaceFlag(t *testing.T) {
	isolatedConfigHome(t, "")
	workDir := tempDirResolved(t)
	t.Chdir(workDir)

	var out, errOut bytes.Buffer
	root := NewRootCommand(strings.NewReader(""), &out, &errOut)
	root.SetArgs([]string{"config", "--space", "claude/sub"})
	err := root.ExecuteContext(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "worktree.space cannot contain '/'")
}
