package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/jmcampanini/grove/internal/config"
	"github.com/jmcampanini/grove/internal/logging"
	"github.com/jmcampanini/grove/internal/render"
	"github.com/jmcampanini/grove/internal/repository"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Version is set at build time via ldflags.
var Version = "n/a"

const (
	diagnosticDebugFlag = "debug"
	diagnosticQuietFlag = "quiet"
)

type commandServices struct {
	loadRuntime func(*cobra.Command) (*repository.Runtime, error)
}

// NewRootCommand builds a fresh grove command tree wired to the given
// streams. Every execution gets its own tree; no state is shared across
// constructions.
func NewRootCommand(in io.Reader, out, errOut io.Writer) *cobra.Command {
	var version bool
	root := &cobra.Command{
		Use:   "grove",
		Short: "Git worktree manager",
		// Groups never suggest corrections for an unknown subcommand, so the
		// root does not either; an unknown command reports only its name.
		DisableSuggestions: true,
		SilenceErrors:      true,
		SilenceUsage:       true,
		Long: `Grove manages git worktrees: one directory per branch, placed under
worktree.root at the path worktree.layout renders (run grove help layout).
It creates a branch and worktree from a phrase, a pull request, or an
issue, and lists, syncs, and prunes the worktrees it finds.

Grove runs git for every command that reads or changes a repository, and
gh for the pr and issue commands, prune, and the pull request column of
status. gh must already be authenticated: grove disables the interactive
prompts of both programs. Each git and gh command is subject to
git.timeout, and its captured stdout and stderr are each capped at 8 MiB;
exceeding either limit fails the command.

Configuration comes from TOML files and the --space and
--worktree-template flags; run grove config --help for the file locations,
precedence, and every key.

Styled output follows --color. auto, the default, colors a stream only
when that stream is a terminal, TERM is not dumb, and NO_COLOR is empty;
always and never override both the detection and NO_COLOR. Help, version,
config, list, and the --fzf, --json, --jsonl, and --csv output of pr list
and issue list are never styled.

Logs are appended to $XDG_STATE_HOME/grove/grove.log
(~/.local/state/grove/grove.log when XDG_STATE_HOME is unset). Logging starts
after argument validation: --help, --version, the help topics, and rejected
input do not create the log file. Diagnostic logging defaults to info. Pass
--debug for debug logging or --quiet to show only errors. The flags are
mutually exclusive and do not change stdout. When the log file cannot be
opened, grove warns on stderr (unless --quiet) and continues without it.

Only sync and prune prompt. Their prompts require a terminal with TERM
set, or TERM=dumb for accessible line-based input without cursor control.
Every other command, including help, version, config, and the help topics,
runs without a terminal.

A failure prints one line on stderr as "grove: <message>" and exits 2 for
a usage error or 1 otherwise; run grove help exit-codes for the contract.

Common tasks:
  Start new work:            grove create "add user auth"
  Check out a branch:        grove checkout feature/fix-login
  Check out a pull request:  grove pr checkout 42
  Work on an issue:          grove issue start 17
  See every worktree:        grove status
  Remove stale worktrees:    grove prune

Command groups:
  cache       The on-disk cache of gh output:          grove cache --help
  completion  Shell completion scripts:               grove completion --help
  issue       GitHub issues and their worktrees:       grove issue --help
  namer       Branch and worktree names from a phrase: grove namer --help
  pr          Pull requests and their worktrees:      grove pr --help`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return usageError{err: err}
			}
			if version {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "grove version %s\n", Version)
				return err
			}
			return cmd.Help()
		},
	}
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)

	registerDiagnosticFlags(root.PersistentFlags())
	root.MarkFlagsMutuallyExclusive(diagnosticDebugFlag, diagnosticQuietFlag)
	registerColorFlag(root.PersistentFlags())
	if err := config.RegisterFlags(root.PersistentFlags()); err != nil {
		panic(err)
	}
	// Cobra's Find strips flags before Execute registers --help and
	// --version, so without these a value flag after either one is read as
	// an operand and "grove --help --worktree-template x" fails with unknown
	// command "x". Registering them here keeps that ordering correct on a
	// root that has no Args validator.
	root.InitDefaultHelpFlag()
	root.Flags().BoolVarP(&version, "version", "v", false, "Print the Grove version")

	root.AddGroup(
		&cobra.Group{ID: "worktree", Title: "Worktree Commands:"},
		&cobra.Group{ID: "git", Title: "Git Commands:"},
		&cobra.Group{ID: "pr", Title: "Pull Request Commands:"},
		&cobra.Group{ID: "issue", Title: "Issue Commands:"},
		&cobra.Group{ID: "config", Title: "Configuration Commands:"},
		&cobra.Group{ID: "utility", Title: "Utility Commands:"},
		&cobra.Group{ID: "topics", Title: "Help topics:"},
	)
	help := newHelpCmd()
	root.SetHelpCommand(help)
	root.AddCommand(help, newCompletionCmd())

	services := commandServices{loadRuntime: loadCommandRuntime}
	root.AddCommand(
		newCacheCmd(services),
		newCatchupCmd(services),
		newCheckoutCmd(services),
		newConfigCmd(services),
		newCreateCmd(services),
		newExitCodesTopicCmd(services),
		newIssueCmd(services),
		newLayoutTopicCmd(services),
		newListCmd(services),
		newNamerCmd(services),
		newNamingTopicCmd(services),
		newPRCmd(services),
		newPruneCmd(services),
		newRemoveCmd(services),
		newResolveCmd(services),
		newStatusCmd(services),
		newSyncCmd(services),
	)
	markUsageErrors(root)

	return root
}

func registerDiagnosticFlags(flags *pflag.FlagSet) {
	flags.Bool(diagnosticDebugFlag, false, "Set diagnostic log level to debug")
	flags.Bool(diagnosticQuietFlag, false, "Set diagnostic log level to error")
}

func executeRoot(root *cobra.Command, args []string) error {
	// Cobra's help callback cannot return an error. Buffer its output so a
	// failed write can be returned after execution through the normal error path.
	renderHelp := root.HelpFunc()
	var helpErr error
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		var output bytes.Buffer
		destination := cmd.OutOrStdout()
		cmd.SetOut(&output)
		renderHelp(cmd, args)
		cmd.SetOut(destination)
		_, helpErr = output.WriteTo(destination)
	})

	root.SetArgs(args)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := root.ExecuteContext(ctx); err != nil {
		return err
	}
	return helpErr
}

// executeWithFileLogging builds a fresh tree on the given streams and runs
// it, teeing diagnostics (and everything else written to stderr) into the
// grove log file. The log file is opened in the root's persistent pre-run,
// after Cobra has parsed flags, handled --help and --version, and validated
// operands, so rejected input never creates the state directory or the log.
// When the log file cannot be opened, a warning is logged and execution
// proceeds with stderr alone.
func executeWithFileLogging(in io.Reader, out, stderr io.Writer, args []string) error {
	root := NewRootCommand(in, out, stderr)

	var tee *logging.Tee
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// Cobra checks flag groups after the persistent pre-run; check them
		// first so conflicting --debug/--quiet never opens the log file.
		if err := cmd.ValidateFlagGroups(); err != nil {
			return usageError{err: err}
		}
		// Help, version, and completion only print metadata, so they neither
		// create the log file nor warn when it cannot be opened.
		if cmd == root || isHelpTopic(cmd) || cmd.Name() == "help" || cmd.Parent().Name() == "completion" || cmd.Name() == "completion" {
			return nil
		}

		opened, err := logging.NewTee(stderr, render.ResolveProfile(colorMode(cmd), stderr))
		if err != nil {
			commandLogger(cmd).Warn("failed to set up file logging", "error", err)
			return nil
		}
		tee = opened
		cmd.Root().SetErr(tee)
		return nil
	}
	defer func() {
		if tee != nil {
			_ = tee.Close()
		}
	}()

	return executeRoot(root, args)
}

// Execute runs the root command on the process streams and returns its exit
// status: 0 on success, 2 on a usage error, 1 on any other failure. A
// failure is reported as one "grove: <message>" line on stderr.
func Execute() int {
	err := executeWithFileLogging(os.Stdin, os.Stdout, os.Stderr, os.Args[1:])
	if err != nil {
		reportFailure(os.Stderr, err)
	}
	return exitStatus(err)
}
