# PLAN: Remove the `grove logs` command (issue #103)

Execute this plan exactly. All design decisions below are settled — do not
relitigate them, add compatibility shims, or expand scope.

## Context

Grove writes file logs to one fixed XDG state path
(`$XDG_STATE_HOME/grove/grove.log`, falling back to
`~/.local/state/grove/grove.log`). The `grove logs` command hierarchy
(`logs path`, `logs tail`) only wraps that fixed path and duplicates standard
tools, so it is being removed per issue #103.

Settled decisions:

- **Full removal, strict clean break.** `grove logs`, `grove logs path`, and
  `grove logs tail` become ordinary unknown-command errors. No aliases, no
  deprecation shims, no migration errors, no tests pinning old behavior.
- **Documentation split by audience.** Root `--help` gets a concise statement
  of the log path + fallback + `--debug` pointer; `grove docs` gets a short
  `## Logging` section carrying the detail.
- **File logging itself is untouched.** Do not modify `internal/logging/` in
  any way. `--debug`, private creation modes, ANSI-free persisted output, and
  warning-only setup failures all keep working.
- **No commit.** Stop with a verified, uncommitted working tree.

## Changes

### 1. Delete files

- `cmd/logs.go`
- `cmd/logs_path.go`
- `cmd/logs_tail.go`
- `cmd/logs_path_test.go`
- `cmd/logs_tail_test.go`

### 2. Drop the tail dependency

`github.com/246859/tail` is imported only by `cmd/logs_tail.go`. After the
deletions, run:

```sh
go mod tidy
```

Confirm `246859/tail` is gone from `go.mod` and `go.sum`.

### 3. `cmd/root.go` — document the log path in root help

Replace `rootCmd`'s `Long` value with:

```go
	Long: `Grove manages git worktrees in a workspace structure.

Common workflows:
  Start new work:       grove create "add user auth"
  Check out a branch:   grove checkout feature/fix-login
  Check out a PR:       grove pr checkout 42
  Work on an issue:     grove issue start 17
  See all worktrees:    grove status

Logs are appended to $XDG_STATE_HOME/grove/grove.log
(~/.local/state/grove/grove.log when XDG_STATE_HOME is unset).
Pass --debug on any command for verbose logging.`,
```

### 4. `cmd/topic_exit_codes.go` — stop referencing `grove logs tail`

Replace the `Long` value of `exitCodesTopicCmd` with:

```go
	Long: `Grove uses the conventional two-value exit scheme:

  0  success
  1  any error (bad arguments, git failures, config errors, I/O, etc.)

Error detail is reported on stderr. When an error is not self-explanatory,
inspect the log file at $XDG_STATE_HOME/grove/grove.log
(~/.local/state/grove/grove.log when XDG_STATE_HOME is unset).`,
```

### 5. `cmd/docs.go` — update the reference document

In `docsMarkdown`:

- Remove these two lines from the "Common commands" list:
  - `- grove logs path: print the fixed log file path.`
  - `- grove logs tail: print recent log lines.`
- Remove this line from the "Topic help pages" list:
  - `- grove help logs: log file location and troubleshooting notes.`
- Append a new final section:

```markdown
## Logging

Grove appends ANSI-free logs for every invocation to a fixed path:

    $XDG_STATE_HOME/grove/grove.log
    ~/.local/state/grove/grove.log   (fallback when XDG_STATE_HOME is unset)

Pass --debug on any command to raise the log level to debug for that invocation.

If grove cannot determine the home directory or open the log file, it prints a
warning to stderr and continues without file logging. Standard output is
unaffected.

Inspect the file with standard tools:

    tail -n 50 ~/.local/state/grove/grove.log
    tail -f ~/.local/state/grove/grove.log
```

### 6. `README.md` — remove the stale topic bullet

Delete this line (and nothing else):

```
- `grove help logs` — where logs go and how to inspect them
```

### 7. Tests

- In `cmd/help_test.go`, add (guards the discoverability requirement — root
  help is now the primary in-product home of the log path):

```go
func TestRootHelpDocumentsLogPath(t *testing.T) {
	for _, want := range []string{
		"$XDG_STATE_HOME/grove/grove.log",
		"~/.local/state/grove/grove.log",
	} {
		assert.Contains(t, rootCmd.Long, want)
	}
}
```

- In `cmd/docs_test.go`, add to `TestDocsCommandWritesReference`:

```go
	assert.Contains(t, output, "## Logging")
	assert.Contains(t, output, "$XDG_STATE_HOME/grove/grove.log")
```

## Verification (all must pass)

The shell is fish — use `env VAR=... cmd` for per-command env vars, not the
`VAR=x cmd` prefix form. Use `.sandbox/` in the repo root for temp files.

1. `make check` — green.
2. `make build` — never `go build` directly. Binary lands at `build/grove`.
3. Clean break behavior — each of these must exit non-zero with an
   unknown-command error (`unknown command "logs"`):
   - `./build/grove logs`
   - `./build/grove logs path`
   - `./build/grove logs tail`
4. `./build/grove --help` — output contains both
   `$XDG_STATE_HOME/grove/grove.log` and `~/.local/state/grove/grove.log`,
   and lists no `logs` command.
5. `./build/grove docs` — output contains `## Logging` and no occurrence of
   `grove logs`.
6. `./build/grove help exit-codes` — shows the new wording, no
   `grove logs tail`.
7. Completion surface: `./build/grove __complete ""` output does not contain
   `logs`.
8. File logging still works:
   ```sh
   env XDG_STATE_HOME=.sandbox/verify-xdg ./build/grove --debug docs > /dev/null
   test -f .sandbox/verify-xdg/grove/grove.log
   ```
   The file must exist and no `failed to set up file logging` warning may
   appear on stderr. Remove `.sandbox/verify-xdg` afterward.
9. Clean-break sweep — no surviving references to the old surface:
   ```sh
   grep -rn "grove logs\|logs path\|logs tail\|246859" --exclude-dir=.git --exclude=PLAN.md .
   ```
   Expect zero hits.

## Finish line

Stop after verification passes. Do NOT commit, push, or open a PR. Leave the
working tree with the uncommitted changes and this untracked PLAN.md in place,
then report the verification results.
