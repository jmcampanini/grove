package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"charm.land/log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *log.Logger {
	return log.New(io.Discard)
}

func newTestRootCommand() *cobra.Command {
	return NewRootCommand(strings.NewReader(""), io.Discard, io.Discard)
}

// executeForTest runs a fresh command tree on buffered streams, the same way
// Execute wires the process streams.
func executeForTest(args ...string) (stdout, stderr string, err error) {
	var out, errOut bytes.Buffer
	root := NewRootCommand(strings.NewReader(""), &out, &errOut)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errOut.String(), err
}

func newLogProbeCmd(ran *bool) *cobra.Command {
	return &cobra.Command{
		Use:  "log-probe",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			*ran = true
			logger := commandLogger(cmd)
			logger.Debug("probe-debug")
			logger.Info("probe-info")
			logger.Warn("probe-warn")
			logger.Error("probe-error")
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "payload")
			return err
		},
	}
}

func TestRootLoggingModesPreserveStdout(t *testing.T) {
	tests := []struct {
		args        []string
		name        string
		wantAbsent  []string
		wantPresent []string
	}{
		{
			name:        "default",
			wantAbsent:  []string{"probe-debug"},
			wantPresent: []string{"probe-info", "probe-warn", "probe-error"},
		},
		{
			args:        []string{"--debug"},
			name:        "debug",
			wantPresent: []string{"probe-debug", "probe-info", "probe-warn", "probe-error"},
		},
		{
			args:        []string{"--quiet"},
			name:        "quiet",
			wantAbsent:  []string{"probe-debug", "probe-info", "probe-warn"},
			wantPresent: []string{"probe-error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			ran := false
			root := NewRootCommand(strings.NewReader(""), &stdout, &stderr)
			root.AddCommand(newLogProbeCmd(&ran))
			root.SetArgs(append(tt.args, "log-probe"))

			require.NoError(t, root.Execute())
			assert.True(t, ran)
			assert.Equal(t, "payload\n", stdout.String())
			for _, message := range tt.wantPresent {
				assert.Contains(t, stderr.String(), message)
			}
			for _, message := range tt.wantAbsent {
				assert.NotContains(t, stderr.String(), message)
			}
		})
	}
}

func TestExecuteWithFileLoggingAppliesDiagnosticLevelBeforeSetupWarning(t *testing.T) {
	blockedStateDir := filepath.Join(t.TempDir(), "state")
	require.NoError(t, os.WriteFile(blockedStateDir, nil, 0o600))
	t.Setenv("XDG_STATE_HOME", blockedStateDir)

	tests := []struct {
		args        []string
		name        string
		wantErr     string
		wantWarning bool
	}{
		{
			args:        []string{"docs"},
			name:        "default reports setup warning",
			wantWarning: true,
		},
		{
			args:        []string{"--debug", "docs"},
			name:        "debug reports setup warning",
			wantWarning: true,
		},
		{
			args: []string{"--quiet", "docs"},
			name: "quiet suppresses setup warning",
		},
		{
			args:        []string{"--help"},
			name:        "help reports setup warning",
			wantWarning: true,
		},
		{
			args: []string{"--quiet", "--help"},
			name: "quiet before help suppresses setup warning",
		},
		{
			args: []string{"--help", "--quiet"},
			name: "quiet after help suppresses setup warning",
		},
		{
			args: []string{"-h", "--quiet"},
			name: "quiet after shorthand help suppresses setup warning",
		},
		{
			args:    []string{"bogus", "--quiet"},
			name:    "quiet invalid command suppresses setup warning",
			wantErr: "unknown command",
		},
		{
			args:    []string{"--nope", "--quiet", "docs"},
			name:    "quiet after unknown flag suppresses setup warning",
			wantErr: "unknown flag",
		},
		{
			args:        []string{"--debug", "--quiet", "docs"},
			name:        "conflict reports setup warning at default level",
			wantErr:     "none of the others can be",
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer

			err := executeWithFileLogging(strings.NewReader(""), io.Discard, &stderr, tt.args)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
			assert.Equal(t, tt.wantWarning, bytes.Contains(stderr.Bytes(), []byte("failed to set up file logging")))
		})
	}
}

func TestRootRejectsConflictingLoggingFlagsBeforeCommandRun(t *testing.T) {
	ran := false
	root := newTestRootCommand()
	root.AddCommand(newLogProbeCmd(&ran))
	root.SetArgs([]string{"--debug", "--quiet", "log-probe"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "none of the others can be")
	assert.False(t, ran)
}

func TestSequentialExecutionsDoNotLeakDiagnosticLevel(t *testing.T) {
	firstRan := false
	var firstOut, firstErr bytes.Buffer
	first := NewRootCommand(strings.NewReader(""), &firstOut, &firstErr)
	first.AddCommand(newLogProbeCmd(&firstRan))
	first.SetArgs([]string{"--debug", "log-probe"})
	require.NoError(t, first.Execute())
	require.True(t, firstRan)
	require.Contains(t, firstErr.String(), "probe-debug")

	secondRan := false
	var secondOut, secondErr bytes.Buffer
	second := NewRootCommand(strings.NewReader(""), &secondOut, &secondErr)
	second.AddCommand(newLogProbeCmd(&secondRan))
	second.SetArgs([]string{"log-probe"})
	require.NoError(t, second.Execute())
	assert.True(t, secondRan)
	assert.NotContains(t, secondErr.String(), "probe-debug")
	assert.Contains(t, secondErr.String(), "probe-info")
	assert.Equal(t, "payload\n", secondOut.String())
}

func TestParallelExecutionsDoNotLeakDiagnostics(t *testing.T) {
	type execution struct {
		args        []string
		errOut      bytes.Buffer
		out         bytes.Buffer
		ran         bool
		wantAbsent  []string
		wantPresent []string
	}

	executions := []*execution{
		{
			args:        []string{"--debug", "log-probe"},
			wantPresent: []string{"probe-debug", "probe-info", "probe-error"},
		},
		{
			args:        []string{"--quiet", "log-probe"},
			wantAbsent:  []string{"probe-debug", "probe-info", "probe-warn"},
			wantPresent: []string{"probe-error"},
		},
	}

	var wg sync.WaitGroup
	for _, e := range executions {
		root := NewRootCommand(strings.NewReader(""), &e.out, &e.errOut)
		root.AddCommand(newLogProbeCmd(&e.ran))
		root.SetArgs(e.args)

		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, root.Execute())
		}()
	}
	wg.Wait()

	for _, e := range executions {
		assert.True(t, e.ran)
		assert.Equal(t, "payload\n", e.out.String())
		for _, message := range e.wantPresent {
			assert.Contains(t, e.errOut.String(), message)
		}
		for _, message := range e.wantAbsent {
			assert.NotContains(t, e.errOut.String(), message)
		}
	}
}

func TestRootConstructionReturnsFreshCommandTree(t *testing.T) {
	assertFreshCommand(t, newTestRootCommand(), newTestRootCommand())
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

// collectLeaves returns every grove-owned leaf command, skipping the
// Cobra-managed help and completion commands.
func collectLeaves(cmd *cobra.Command) []*cobra.Command {
	var leaves []*cobra.Command
	for _, child := range cmd.Commands() {
		if child.Name() == "help" || child.Name() == "completion" || strings.HasPrefix(child.Name(), "__") {
			continue
		}
		if child.HasSubCommands() {
			leaves = append(leaves, collectLeaves(child)...)
			continue
		}
		leaves = append(leaves, child)
	}
	return leaves
}

// leafArgs returns the CLI arguments that address leaf, e.g. ["cache", "clear"].
func leafArgs(leaf *cobra.Command) []string {
	return strings.Fields(leaf.CommandPath())[1:]
}

func TestEveryLeafDeclaresCommandContract(t *testing.T) {
	leaves := collectLeaves(newTestRootCommand())
	require.NotEmpty(t, leaves)

	for _, leaf := range leaves {
		path := leaf.CommandPath()
		assert.NotNilf(t, leaf.Args, "%s must declare an Args validator", path)
		assert.Nilf(t, leaf.Run, "%s must use RunE, not Run", path)
		if leaf.Runnable() {
			assert.NotNilf(t, leaf.RunE, "%s must use RunE", path)
		}
	}
}

func TestEveryLeafRejectsExcessOperands(t *testing.T) {
	extra := []string{"extra-1", "extra-2", "extra-3", "extra-4", "extra-5"}

	for _, leaf := range collectLeaves(newTestRootCommand()) {
		t.Run(strings.ReplaceAll(leaf.CommandPath(), " ", "/"), func(t *testing.T) {
			stdout, _, err := executeForTest(append(leafArgs(leaf), extra...)...)

			require.Error(t, err, "operands beyond every validator's limit must be rejected before side effects")
			assert.Empty(t, stdout)
		})
	}
}

func TestEveryLeafRejectsUnknownFlags(t *testing.T) {
	for _, leaf := range collectLeaves(newTestRootCommand()) {
		t.Run(strings.ReplaceAll(leaf.CommandPath(), " ", "/"), func(t *testing.T) {
			stdout, _, err := executeForTest(append(leafArgs(leaf), "--definitely-not-a-grove-flag")...)

			require.ErrorContains(t, err, "unknown flag")
			assert.Empty(t, stdout)
		})
	}
}

func TestEveryLeafPrintsHelpToInjectedStdout(t *testing.T) {
	for _, leaf := range collectLeaves(newTestRootCommand()) {
		t.Run(strings.ReplaceAll(leaf.CommandPath(), " ", "/"), func(t *testing.T) {
			stdout, stderr, err := executeForTest(append(leafArgs(leaf), "--help")...)

			require.NoError(t, err)
			assert.Contains(t, stdout, leaf.Name())
			assert.Empty(t, stderr)
		})
	}
}

func TestRootVersionPrintsToInjectedStdout(t *testing.T) {
	stdout, stderr, err := executeForTest("--version")

	require.NoError(t, err)
	assert.Contains(t, stdout, Version)
	assert.Empty(t, stderr)
}

func TestRootRejectsUnknownSubcommand(t *testing.T) {
	stdout, _, err := executeForTest("definitely-not-a-command")

	require.ErrorContains(t, err, "unknown command")
	assert.Empty(t, stdout)
}
