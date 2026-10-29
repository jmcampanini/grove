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
			t.Setenv("CODE_DIR", "")
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
			assert.Contains(t, out.String(), "# Effective worktree placement\n# unresolved = ")
			assert.Contains(t, out.String(), "unset environment variable CODE_DIR")

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

func TestWriteEffectiveWorktreeConfig(t *testing.T) {
	t.Setenv("HOME", "/home/grove")
	t.Setenv("GROVE_TEST_ROOT", "/workspace")
	t.Setenv("GROVE_MISSING_ROOT", "")

	tests := []struct {
		name   string
		root   string
		layout string
		want   string
	}{
		{
			name:   "expanded environment root",
			root:   "$GROVE_TEST_ROOT/.worktrees",
			layout: "{{.Host}}/{{.Owner}}/{{.Repo}}/{{.Name}}",
			want:   "# expanded_root = \"/workspace/.worktrees\"\n# requires_origin = true\n",
		},
		{
			name:   "home expansion without remote",
			root:   "~/.worktrees",
			layout: "{{.Name}}",
			want:   "# expanded_root = \"/home/grove/.worktrees\"\n# requires_origin = false\n",
		},
		{
			name:   "control characters stay in one comment",
			root:   "/workspace/\nname",
			layout: "{{.Name}}",
			want:   "# expanded_root = \"/workspace/\\nname\"\n# requires_origin = false\n",
		},
		{
			name:   "unresolved root remains inspectable",
			root:   "$GROVE_MISSING_ROOT/.worktrees",
			layout: "{{.Name}}",
			want:   "# unresolved = \"invalid worktree.root: ",
		},
		{
			name:   "invalid layout remains inspectable",
			root:   "/workspace",
			layout: "{{.Missing}}",
			want:   "# unresolved = \"invalid worktree.layout: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			cfg := config.WorktreeConfig{Root: tt.root, Layout: tt.layout}

			err := writeEffectiveWorktreeConfig(&out, cfg)

			if err != nil {
				t.Fatalf("writeEffectiveWorktreeConfig() error = %v", err)
			}
			if !strings.HasPrefix(out.String(), "\n# Effective worktree placement\n"+tt.want) {
				t.Errorf("writeEffectiveWorktreeConfig() = %q, want section with %q", out.String(), tt.want)
			}
			for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
				if !strings.HasPrefix(line, "# ") {
					t.Errorf("effective value escaped its TOML comment: %q", line)
				}
			}
		})
	}
}

func TestWriteEffectiveWorktreeConfigReportsWriteFailure(t *testing.T) {
	err := writeEffectiveWorktreeConfig(failingWriter{}, defaultTestConfig().Worktree)

	require.ErrorIs(t, err, errWriteFailed)
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
