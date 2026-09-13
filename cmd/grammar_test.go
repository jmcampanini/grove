package cmd

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commandGrammar declares the positional grammar of one application-owned
// command: every operand count the command accepts and the first count the
// validator rejects on each side of the accepted range. boundary names the
// first configuration, git, or cache error the runner reports in an
// environment with no usable home directory, no git on PATH, and no
// repository; empty means the command prints help or documentation and
// succeeds there.
type commandGrammar struct {
	accept   []int
	boundary string
	reject   []int
}

const (
	boundaryHelp   = ""
	boundaryCache  = "failed to read cache dir"
	boundaryConfig = "failed to load config"
	boundaryGit    = "git command failed"
)

// commandGrammars is the grammar inventory of every application-owned
// command, keyed by command path. A command missing from this table or a
// table entry naming no command fails the inventory test.
var commandGrammars = map[string]commandGrammar{
	"grove":                {accept: []int{0}, reject: []int{1}, boundary: boundaryHelp},
	"grove cache":          {accept: []int{0}, reject: []int{1}, boundary: boundaryHelp},
	"grove cache clear":    {accept: []int{0}, reject: []int{1}, boundary: boundaryCache},
	"grove catchup":        {accept: []int{0}, reject: []int{1}, boundary: boundaryGit},
	"grove checkout":       {accept: []int{1}, reject: []int{0, 2}, boundary: boundaryGit},
	"grove config":         {accept: []int{0}, reject: []int{1}, boundary: boundaryConfig},
	"grove create":         {accept: []int{1}, reject: []int{0, 2}, boundary: boundaryGit},
	"grove docs":           {accept: []int{0}, reject: []int{1}, boundary: boundaryHelp},
	"grove exit-codes":     {accept: []int{0}, reject: []int{1}, boundary: boundaryHelp},
	"grove issue":          {accept: []int{0}, reject: []int{1}, boundary: boundaryHelp},
	"grove issue list":     {accept: []int{0}, reject: []int{1}, boundary: boundaryGit},
	"grove issue preview":  {accept: []int{1}, reject: []int{0, 2}, boundary: boundaryGit},
	"grove issue start":    {accept: []int{1}, reject: []int{0, 2}, boundary: boundaryGit},
	"grove list":           {accept: []int{0}, reject: []int{1}, boundary: boundaryGit},
	"grove namer":          {accept: []int{0}, reject: []int{1}, boundary: boundaryHelp},
	"grove namer branch":   {accept: []int{1}, reject: []int{0, 2}, boundary: boundaryConfig},
	"grove namer slug":     {accept: []int{1}, reject: []int{0, 2}, boundary: boundaryConfig},
	"grove namer worktree": {accept: []int{1}, reject: []int{0, 2}, boundary: boundaryConfig},
	"grove pr":             {accept: []int{0}, reject: []int{1}, boundary: boundaryHelp},
	"grove pr checkout":    {accept: []int{1}, reject: []int{0, 2}, boundary: boundaryGit},
	"grove pr list":        {accept: []int{0}, reject: []int{1}, boundary: boundaryGit},
	"grove pr preview":     {accept: []int{1}, reject: []int{0, 2}, boundary: boundaryGit},
	"grove prune":          {accept: []int{0}, reject: []int{1}, boundary: boundaryGit},
	"grove remove":         {accept: []int{1}, reject: []int{0, 2}, boundary: boundaryGit},
	"grove resolve":        {accept: []int{0, 1}, reject: []int{2}, boundary: boundaryConfig},
	"grove status":         {accept: []int{0}, reject: []int{1}, boundary: boundaryGit},
	"grove sync":           {accept: []int{0}, reject: []int{1}, boundary: boundaryGit},
	"grove workspace":      {accept: []int{0}, reject: []int{1}, boundary: boundaryHelp},
}

// arityErrorPattern matches the messages Cobra's built-in arity validators
// return, so a test can tell a grammar rejection from a runner failure.
const arityErrorPattern = `^(unknown command "[^"]*" for "grove[^"]*"|accepts (at most )?\d+ arg\(s\), received \d+|requires at least \d+ arg\(s\))`

// applicationCommands returns every grove-owned command in the tree, the
// root included, keyed by command path. Cobra's help and completion commands
// are left out.
func applicationCommands(root *cobra.Command) map[string]*cobra.Command {
	commands := map[string]*cobra.Command{root.CommandPath(): root}
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			if child.Name() == "help" || child.Name() == "completion" || strings.HasPrefix(child.Name(), "__") {
				continue
			}
			commands[child.CommandPath()] = child
			visit(child)
		}
	}
	visit(root)
	return commands
}

// commandArgs returns the CLI arguments that address a command by path, so
// "grove cache clear" becomes ["cache", "clear"].
func commandArgs(path string) []string {
	return strings.Fields(path)[1:]
}

// operands returns count operand values. They are numeric so every grammar
// in the table accepts them past its runner's own parsing: issue and pull
// request commands parse a number before any git or gh work.
func operands(count int) []string {
	values := make([]string, count)
	for i := range values {
		values[i] = fmt.Sprintf("%d", i+1)
	}
	return values
}

func sortedGrammarPaths() []string {
	paths := make([]string, 0, len(commandGrammars))
	for path := range commandGrammars {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

// executeWithFileLoggingForTest runs a fresh tree on buffered streams through
// the same entry point Execute uses, so the log-file hook is part of the run.
func executeWithFileLoggingForTest(args ...string) (stdout, stderr string, err error) {
	var out, errOut bytes.Buffer
	err = executeWithFileLogging(strings.NewReader(""), &out, &errOut, args)
	return out.String(), errOut.String(), err
}

// setInvalidEnvironment points every directory grove could read or write at
// a regular file, empties PATH so git and gh cannot run, and moves into an
// empty directory outside any repository. Any command work past validation
// then fails at a recognizable boundary, and any file grove creates shows up
// under the returned directory.
func setInvalidEnvironment(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	require.NoError(t, os.WriteFile(blocked, nil, 0o600))
	cwd := filepath.Join(base, "cwd")
	require.NoError(t, os.Mkdir(cwd, 0o755))

	t.Chdir(cwd)
	for _, name := range []string{"HOME", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME"} {
		t.Setenv(name, blocked)
	}
	t.Setenv("PATH", "")
	return base
}

// snapshotTree records every entry under root by relative path: directories
// as "dir", regular files by size and content hash.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	entries := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			entries[rel] = "dir"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries[rel] = fmt.Sprintf("file:%d:%x", len(data), sha256.Sum256(data))
		return nil
	})
	require.NoError(t, err)
	return entries
}

func TestEveryApplicationCommandDeclaresItsGrammar(t *testing.T) {
	commands := applicationCommands(newTestRootCommand())

	for path, cmd := range commands {
		assert.Nilf(t, cmd.Run, "%s must use RunE, not Run", path)
		assert.NotNilf(t, cmd.RunE, "%s must be runnable through RunE so Cobra validates its operands", path)
		assert.NotNilf(t, cmd.Args, "%s must declare an Args validator", path)
		_, declared := commandGrammars[path]
		assert.Truef(t, declared, "%s has no commandGrammars entry; declare its accepted and rejected arities", path)
	}
	for path := range commandGrammars {
		_, exists := commands[path]
		assert.Truef(t, exists, "commandGrammars entry %q names no command; remove it", path)
	}
}

func TestEveryApplicationCommandGrammarHoldsAtTheBoundary(t *testing.T) {
	base := setInvalidEnvironment(t)
	commands := applicationCommands(newTestRootCommand())

	for _, path := range sortedGrammarPaths() {
		cmd, exists := commands[path]
		if !exists {
			continue // reported by TestEveryApplicationCommandDeclaresItsGrammar
		}
		grammar := commandGrammars[path]
		name := strings.ReplaceAll(cmd.CommandPath(), " ", "/")

		for _, count := range grammar.reject {
			t.Run(fmt.Sprintf("%s/rejects %d operands", name, count), func(t *testing.T) {
				before := snapshotTree(t, base)

				stdout, stderr, err := executeWithFileLoggingForTest(append(commandArgs(path), operands(count)...)...)

				require.Error(t, err)
				assert.Regexp(t, arityErrorPattern, err.Error())
				assert.Empty(t, stdout)
				assert.Empty(t, stderr, "a rejected operand list must not reach logging setup or any runner")
				assert.Equal(t, before, snapshotTree(t, base), "a rejected operand list must not create files")
			})
		}

		for _, count := range grammar.accept {
			t.Run(fmt.Sprintf("%s/accepts %d operands", name, count), func(t *testing.T) {
				stdout, stderr, err := executeWithFileLoggingForTest(append(commandArgs(path), operands(count)...)...)

				assert.Contains(t, stderr, "failed to set up file logging", "an accepted operand list must pass validation and reach logging setup")
				if grammar.boundary == boundaryHelp {
					require.NoError(t, err)
					assert.NotEmpty(t, stdout)
					return
				}
				require.ErrorContains(t, err, grammar.boundary)
				assert.NotRegexp(t, arityErrorPattern, err.Error())
				assert.Empty(t, stdout)
			})
		}
	}
}

func TestFrameworkHelpAndCompletionFlowsSurviveGrammarValidators(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"help"}, want: "Common workflows:"},
		{args: []string{"help", "pr"}, want: "Subcommands:"},
		{args: []string{"help", "pr", "checkout"}, want: "checkout <number>"},
		{args: []string{"completion", "zsh"}, want: "compdef"},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			stdout, stderr, err := executeForTest(tt.args...)

			require.NoError(t, err, stderr)
			assert.Contains(t, stdout, tt.want)
		})
	}
}

func TestEveryApplicationCommandRejectsUnknownFlags(t *testing.T) {
	commands := applicationCommands(newTestRootCommand())
	for _, path := range sortedGrammarPaths() {
		if _, exists := commands[path]; !exists {
			continue
		}
		t.Run(strings.ReplaceAll(path, " ", "/"), func(t *testing.T) {
			stdout, _, err := executeForTest(append(commandArgs(path), "--definitely-not-a-grove-flag")...)

			require.ErrorContains(t, err, "unknown flag")
			assert.Empty(t, stdout)
		})
	}
}

func TestEveryApplicationCommandPrintsHelpToInjectedStdout(t *testing.T) {
	commands := applicationCommands(newTestRootCommand())
	for _, path := range sortedGrammarPaths() {
		cmd, exists := commands[path]
		if !exists {
			continue
		}
		t.Run(strings.ReplaceAll(path, " ", "/"), func(t *testing.T) {
			stdout, stderr, err := executeForTest(append(commandArgs(path), "--help")...)

			require.NoError(t, err)
			assert.Contains(t, stdout, cmd.Name())
			assert.Empty(t, stderr)
		})
	}
}
