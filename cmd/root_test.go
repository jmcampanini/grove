package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"charm.land/log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func restoreDefaultLogger(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		log.SetLevel(log.InfoLevel)
		log.SetOutput(os.Stderr)
	})
}

func newLogProbeCmd(ran *bool) *cobra.Command {
	return &cobra.Command{
		Use:  "log-probe",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			*ran = true
			log.Debug("probe-debug")
			log.Info("probe-info")
			log.Warn("probe-warn")
			log.Error("probe-error")
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "payload")
			return err
		},
	}
}

func TestRootLoggingModesPreserveStdout(t *testing.T) {
	restoreDefaultLogger(t)

	tests := []struct {
		args        []string
		name        string
		wantAbsent  []string
		wantLevel   log.Level
		wantPresent []string
	}{
		{
			name:        "default",
			wantAbsent:  []string{"probe-debug"},
			wantLevel:   log.InfoLevel,
			wantPresent: []string{"probe-info", "probe-warn", "probe-error"},
		},
		{
			args:        []string{"--debug"},
			name:        "debug",
			wantLevel:   log.DebugLevel,
			wantPresent: []string{"probe-debug", "probe-info", "probe-warn", "probe-error"},
		},
		{
			args:        []string{"--quiet"},
			name:        "quiet",
			wantAbsent:  []string{"probe-debug", "probe-info", "probe-warn"},
			wantLevel:   log.ErrorLevel,
			wantPresent: []string{"probe-error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diagnostics bytes.Buffer
			var stdout bytes.Buffer
			log.SetOutput(&diagnostics)

			ran := false
			root := newRootCmd()
			root.AddCommand(newLogProbeCmd(&ran))
			root.SetArgs(append(tt.args, "log-probe"))
			root.SetOut(&stdout)

			require.NoError(t, root.Execute())
			assert.True(t, ran)
			assert.Equal(t, "payload\n", stdout.String())
			assert.Equal(t, tt.wantLevel, log.GetLevel())
			for _, message := range tt.wantPresent {
				assert.Contains(t, diagnostics.String(), message)
			}
			for _, message := range tt.wantAbsent {
				assert.NotContains(t, diagnostics.String(), message)
			}
		})
	}
}

func TestExecuteRootAppliesQuietBeforeSetupWarning(t *testing.T) {
	restoreDefaultLogger(t)

	blockedStateDir := filepath.Join(t.TempDir(), "state")
	require.NoError(t, os.WriteFile(blockedStateDir, nil, 0o600))
	t.Setenv("XDG_STATE_HOME", blockedStateDir)

	tests := []struct {
		args        []string
		name        string
		wantWarning bool
	}{
		{
			name:        "default reports setup warning",
			wantWarning: true,
		},
		{
			args: []string{"--quiet"},
			name: "quiet suppresses setup warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diagnostics bytes.Buffer
			log.SetOutput(&diagnostics)

			root := newRootCmd()
			root.SetArgs(append(tt.args, "docs"))
			root.SetOut(io.Discard)

			require.NoError(t, executeRoot(root))
			assert.Equal(t, tt.wantWarning, bytes.Contains(diagnostics.Bytes(), []byte("failed to set up file logging")))
		})
	}
}

func TestRootRejectsConflictingLoggingFlagsBeforeCommandRun(t *testing.T) {
	restoreDefaultLogger(t)

	ran := false
	root := newRootCmd()
	root.AddCommand(newLogProbeCmd(&ran))
	root.SetArgs([]string{"--debug", "--quiet", "log-probe"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "debug")
	assert.Contains(t, err.Error(), "quiet")
	assert.False(t, ran)
	assert.Equal(t, log.InfoLevel, log.GetLevel())
}

func TestRootConstructionResetsLoggingLevel(t *testing.T) {
	restoreDefaultLogger(t)
	log.SetOutput(&bytes.Buffer{})

	firstRan := false
	first := newRootCmd()
	first.AddCommand(newLogProbeCmd(&firstRan))
	first.SetArgs([]string{"--debug", "log-probe"})
	first.SetOut(io.Discard)
	require.NoError(t, first.Execute())
	require.True(t, firstRan)
	require.Equal(t, log.DebugLevel, log.GetLevel())

	secondRan := false
	second := newRootCmd()
	second.AddCommand(newLogProbeCmd(&secondRan))
	second.SetArgs([]string{"log-probe"})
	second.SetOut(io.Discard)
	require.NoError(t, second.Execute())
	assert.True(t, secondRan)
	assert.Equal(t, log.InfoLevel, log.GetLevel())
}

func TestRootConstructionReturnsFreshCommandTree(t *testing.T) {
	restoreDefaultLogger(t)
	assertFreshCommand(t, newRootCmd(), newRootCmd())
}

func assertFreshCommand(t *testing.T, first, second *cobra.Command) {
	t.Helper()
	require.Equal(t, first.Name(), second.Name())
	assert.NotSame(t, first, second)
	assertFreshFlags(t, first.LocalNonPersistentFlags(), second.LocalNonPersistentFlags())
	assertFreshFlags(t, first.PersistentFlags(), second.PersistentFlags())

	firstChildren := commandChildrenByName(first)
	secondChildren := commandChildrenByName(second)
	require.Equal(t, len(firstChildren), len(secondChildren))
	for name, firstChild := range firstChildren {
		secondChild, ok := secondChildren[name]
		require.Truef(t, ok, "second tree is missing command %q", name)
		assertFreshCommand(t, firstChild, secondChild)
	}
}

func assertFreshFlags(t *testing.T, first, second *pflag.FlagSet) {
	t.Helper()
	firstCount := 0
	first.VisitAll(func(firstFlag *pflag.Flag) {
		firstCount++
		secondFlag := second.Lookup(firstFlag.Name)
		require.NotNilf(t, secondFlag, "second command is missing flag %q", firstFlag.Name)
		assert.NotSame(t, firstFlag, secondFlag)
	})

	secondCount := 0
	second.VisitAll(func(*pflag.Flag) {
		secondCount++
	})
	assert.Equal(t, firstCount, secondCount)
}

func commandChildrenByName(cmd *cobra.Command) map[string]*cobra.Command {
	children := make(map[string]*cobra.Command, len(cmd.Commands()))
	for _, child := range cmd.Commands() {
		children[child.Name()] = child
	}
	return children
}
