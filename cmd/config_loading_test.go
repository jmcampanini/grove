package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jmcampanini/grove-cli/internal/config"
)

// isolatedConfigHome creates a fake HOME containing an XDG-default grove
// config file and points the process environment at it.
func isolatedConfigHome(t *testing.T, content string) string {
	t.Helper()

	homeDir := tempDirResolved(t)
	configDir := filepath.Join(homeDir, ".config", "grove")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	configPath := filepath.Join(configDir, "grove.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o644))

	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", "")
	return configPath
}

// tempDirResolved returns a temp dir with symlinks resolved so paths compare
// equal to git and getwd output on macOS.
func tempDirResolved(t *testing.T) string {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	return dir
}

func newTestRoot() *cobra.Command {
	root := NewRootCommand(strings.NewReader(""), io.Discard, io.Discard)
	root.SetContext(context.Background())
	return root
}

func TestLoadReportingConfigOutsideRepo(t *testing.T) {
	configPath := isolatedConfigHome(t, "[naming]\nmax_length = 42\n")
	workDir := tempDirResolved(t)

	cfg, report, err := loadReportingConfig(newTestRoot(), workDir)

	require.NoError(t, err)
	assert.Equal(t, 42, cfg.Naming.MaxLength)
	assert.Equal(t, []string{configPath}, report.LoadedFiles)
}

func TestLoadConfigAtRequireRuntimeOutsideRepo(t *testing.T) {
	isolatedConfigHome(t, "")
	workDir := tempDirResolved(t)

	_, err := loadConfigAt(newTestRoot(), testLogger(), workDir, requireRuntime)

	assert.ErrorIs(t, err, errNotGitRepo)
}

func TestRuntimeAndReportingConfigAgreeInRepo(t *testing.T) {
	isolatedConfigHome(t, "[naming]\nmax_length = 42\n")
	repoDir := tempDirResolved(t)
	initGitRepo(t, repoDir)
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "grove.toml"), []byte("[naming]\nmax_length = 33\n"), 0o644))
	t.Chdir(repoDir)

	root := newTestRoot()
	rt, err := loadCommandRuntime(root)
	require.NoError(t, err)
	cfg, report, err := loadReportingConfig(root, repoDir)
	require.NoError(t, err)

	assert.Equal(t, rt.cfg, cfg)
	assert.Equal(t, rt.configReport, report)
	assert.Equal(t, 33, cfg.Naming.MaxLength, "repo config overrides the XDG layer")
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
