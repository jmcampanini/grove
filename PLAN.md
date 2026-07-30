# Plan: Remove `log.file` config, always log to the XDG state path (issue #75)

Implements issue #75 with scope adjustments approved by the user during design review.
This document is self-contained; you do not need to read the issue.

## Outcome

Grove always writes its log to one fixed private location. The `log.file` config key is
removed with a strict clean break. Logging is installed once at the command boundary so
every invocation (`status`, `logs`, `config`, `init`, `docs`, help, completions) follows
the same lifecycle.

## Settled decisions (do not relitigate)

| Decision | Resolution |
|---|---|
| Log destination | Always `$XDG_STATE_HOME/grove/grove.log`, falling back to `~/.local/state/grove/grove.log`. No config key, no flag. |
| Compat mode | **Strict clean break.** `log.file` / `LogConfig` may survive only in git history and change records (this file, commit messages, PR text). No aliases, no migration guards, no key-specific error messages, no tests pinning old behavior. A stale `[log]` key in a user's TOML fails via the loader's ordinary unknown-key error. |
| Permissions | Create state dir `0700` and log file `0600`. **Creation only** — do NOT chmod-repair pre-existing files/dirs, do NOT add symlink or non-regular-file checks. (User explicitly trimmed the issue's hardening scope: "make it so we don't start mistakes," not "fix other people's mistakes.") |
| Existing log migration | The final verification step deletes the machine's existing log file once so it is recreated with correct permissions. No code handles this. |
| Persisted content | **Unchanged.** The file remains an ANSI-stripped mirror of the terminal log stream. No redaction, no level capping, no changes to git/gh logging call sites. (User explicitly declined the issue's redaction scope.) |
| Install point | Top of `Execute()` in `cmd/root.go`, before `ExecuteContext`, pairing with the existing `defer logging.Close()`. |
| Path derivation | `DefaultLogFilePath()` moves from `internal/config/defaults.go` to `internal/logging`. `logging.Setup()` becomes zero-argument and computes the path itself. `internal/config` ends up logging-free. |
| Rotation | Out of scope. File grows unbounded, as today. |
| Failure mode | Unwritable path / undeterminable home: one stderr warning, command continues without file logging, stdout untouched. |

## Current state (for orientation)

- `internal/logging/tee.go` — `Setup(filePath string)` tees the charm logger to stderr +
  file (ANSI-stripped), creating dir/file with `0755`/`0644`. Called only from
  `loadCommandRuntime` (`cmd/runtime_context.go:98`), so config-less commands never log to file.
- `internal/config/config.go` — `Config.Log` field + `LogConfig` struct (`toml:"log"`, `file` key).
- `internal/config/defaults.go` — `DefaultLogFilePath()` (XDG logic) feeds `DefaultConfig()`.
- `cmd/logs.go` — ~75 lines (`loadLogConfig`, `resolveLogConfigPaths`, `resolveLogPath`)
  duplicating config discovery solely to find `cfg.Log.File`.
- `cmd/docs.go:111` — documents `log.file` behavior.
- `main.go` prints the final error via `fmt.Fprintf(os.Stderr, ...)` — not through the
  logger. Leave that as is (stdout payloads / stderr diagnostics / plain file separation
  already holds).

## Implementation steps

### 1. `internal/logging`

- Move `DefaultLogFilePath()` here from `internal/config/defaults.go`, unchanged in
  behavior: `$XDG_STATE_HOME/grove/grove.log`; else `~/.local/state/grove/grove.log`;
  empty string when the home directory cannot be determined.
- Change `Setup(filePath string)` to `Setup() error`:
  - Compute the path via `DefaultLogFilePath()`; if empty, return an error (home unknown).
  - `os.MkdirAll(dir, 0700)` (was `0755`); `os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)` (was `0644`).
  - Everything else (tee writer, color profile capture, `Close()`) stays as is.

### 2. `cmd/root.go`

- At the top of `Execute()`, before `ExecuteContext`:
  ```go
  if err := logging.Setup(); err != nil {
      log.Warn("failed to set up file logging", "error", err)
  }
  ```
  The warning goes to stderr only (the tee is not installed on failure), satisfying the
  one-warning fail-safe. `defer logging.Close()` is already there.

### 3. `cmd/runtime_context.go`

- Delete the `logging.Setup(cfg.Log.File)` call and its warning (lines ~98-100); drop the
  now-unused `internal/logging` import.

### 4. `internal/config`

- `config.go`: remove the `Log` field from `Config` and delete the `LogConfig` type.
  (Field order stays alphabetical.)
- `defaults.go`: remove the `Log:` entry from `DefaultConfig()` and delete
  `DefaultLogFilePath()` (moved in step 1).

### 5. `cmd/logs.go`

- Delete `loadLogConfig` and `resolveLogConfigPaths` entirely.
- `resolveLogPath` no longer needs config/git/context: return
  `logging.DefaultLogFilePath()`, erroring when it is empty. Simplify the signatures of
  its callers (`cmd/logs_path.go`, `cmd/logs_tail.go`) accordingly — dropping the
  `context.Context` parameter where it becomes unused is fine.
- Rewrite the `logs` command's `Long` help: grove always logs to
  `$XDG_STATE_HOME/grove/grove.log` (fallback `~/.local/state/grove/grove.log`); file
  logging is skipped for a single invocation, with a stderr warning, if the file cannot
  be opened; `--debug` raises the level. No mention of `log.file`.

### 6. `cmd/docs.go`

- Remove the `log.file` bullet (line ~111). Check surrounding text for coherence.

### 7. Clean-break sweep

After the above, these must return **zero hits** in tracked files (excluding this PLAN.md
and git history):

```sh
grep -rn "log\.file" --include="*.go" --include="*.md" .
grep -rn "LogConfig\|cfg\.Log\b\|\.Log\.File" --include="*.go" .
grep -rn "config\.DefaultLogFilePath" --include="*.go" .
```

Known hits to clean: `cmd/config_test.go:20,24` (fixture row + expected output),
`internal/config/config_test.go:49-50,761-770,~799` (default assertions,
`TestDefaultLogFilePath`, loader table entries).

## Tests

Follow AGENTS.md: table-driven where applicable; run `make check`; build with `make build`.
All tests must isolate the path via `t.Setenv("XDG_STATE_HOME", t.TempDir())` (and
`t.Setenv("HOME", ...)` for the fallback case) so nothing touches the real state dir.

- `internal/logging/tee_test.go` (update for zero-arg `Setup`):
  - creates dir with `0700` and file with `0600` on first write
  - honors `XDG_STATE_HOME`; falls back to `~/.local/state` when unset
  - appends across successive `Setup`/`Close` cycles
  - unwritable parent → `Setup` returns an error and the logger still writes to stderr
- `internal/logging` or config tests: move/adapt the `DefaultLogFilePath` cases
  (`XDG_STATE_HOME` set / unset) to the logging package.
- `cmd/logs_path_test.go` / `cmd/logs_tail_test.go`: point at the fixed env-derived path;
  no config involvement.
- `internal/config/config_test.go` / `cmd/config_test.go`: remove log-key fixtures; add
  (or confirm an existing) loader test that an unknown `[log]` table fails with the
  loader's ordinary unknown-key error — asserting only that it errors, not any
  log-specific message.

## End-to-end verification (agent-run, required)

Use `.sandbox/` in the repo root for scratch dirs (not `/tmp`). Run after `make check`
and `make build` pass:

1. **Custom XDG**: `env XDG_STATE_HOME=$PWD/.sandbox/xdg build/grove logs path` →
   prints `.sandbox/xdg/grove/grove.log`. Run `build/grove status` (or another
   representative command) with the same env from a worktree; verify the file exists,
   dir is `0700`, file is `0600` (`stat -f "%Lp"` on macOS), and `build/grove logs tail`
   prints content.
2. **Fallback**: `env -u XDG_STATE_HOME HOME=$PWD/.sandbox/home build/grove logs path` →
   prints `.sandbox/home/.local/state/grove/grove.log`; repeat the creation checks.
3. **Fail-safe**: point `XDG_STATE_HOME` at a read-only dir (`chmod 555`); run a command;
   expect exactly one stderr warning, successful command exit, and stdout identical to a
   normal run.
4. **Stale key**: write `[log]\nfile = "/tmp/x.log"` into a sandbox config the loader
   reads; expect an ordinary unknown-key load failure, with no migration-specific text.
5. **Sweep**: run the step-7 greps; confirm zero hits.
6. **One-time migration on this machine**: delete the real existing log so it is
   recreated with correct permissions:
   `rm -f "${XDG_STATE_HOME:-$HOME/.local/state}/grove/grove.log"`.
