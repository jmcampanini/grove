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
	err := writeConfigProvenance(&buf, []string{"Path", "Value", "Source"}, [][]string{
		{"git.timeout", `"5s"`, "<default>"},
		{"naming.maxlength", "42", "/home/jim/.config/grove/config.toml"},
		{"worktree.space", `"grove"`, "/home/jim/odd\ndir\x01\xff/grove.toml"},
	})
	require.NoError(t, err)

	assert.Equal(t, "\n# Provenance\n"+
		"# Path\tValue\tSource\n"+
		"# git.timeout\t\"5s\"\t<default>\n"+
		"# naming.maxlength\t42\t/home/jim/.config/grove/config.toml\n"+
		`# worktree.space	"grove"	/home/jim/odd\ndir\x01\xff/grove.toml`+"\n", buf.String())
}

func TestConfigCommandOutsideRepoRoundTrips(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantProvenance bool
	}{
		{name: "default report", args: []string{"config"}},
		{name: "forced color stays plain TOML", args: []string{"config", "--color=always"}},
		{name: "provenance appended", args: []string{"config", "--provenance"}, wantProvenance: true},
		{name: "sources alias", args: []string{"config", "--sources"}, wantProvenance: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := isolatedConfigHome(t, "[naming]\nmax_length = 42\n")
			workDir := tempDirResolved(t)
			t.Chdir(workDir)

			var out, errOut bytes.Buffer
			root := NewRootCommand(strings.NewReader(""), &out, &errOut)
			root.SetArgs(tt.args)
			require.NoError(t, root.ExecuteContext(context.Background()), errOut.String())

			roundTripPath := filepath.Join(workDir, "roundtrip.toml")
			require.NoError(t, os.WriteFile(roundTripPath, out.Bytes(), 0o644))
			reloaded, _, err := config.Load([]string{roundTripPath}, nil)
			require.NoError(t, err)
			want, _, err := loadReportingConfig(newTestRoot(), workDir)
			require.NoError(t, err)
			assert.Equal(t, want, reloaded, "config output should round-trip to the effective config")
			assert.Equal(t, 42, reloaded.Naming.MaxLength)

			if !tt.wantProvenance {
				assert.NotContains(t, out.String(), "# Provenance")
				return
			}
			report, provenance, found := strings.Cut(out.String(), "\n\n# Provenance\n")
			require.True(t, found, "provenance should follow the TOML after a blank line")
			assert.Contains(t, report, "max_length = 42")
			assert.True(t, strings.HasPrefix(provenance, "# Path\tValue\tSource\n"), provenance)
			assert.Contains(t, provenance, "# naming.maxlength\t42\t"+configPath+"\n")
		})
	}
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
