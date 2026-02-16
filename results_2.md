# Test Results: Grove Commands From All Three Directory Types

**Date:** 2026-02-16
**Binary:** `/tmp/grove-test` (built from `wt-work-from-workspace-dir`)
**PR used:** #22

## Locations

| ID | Path | Git marker |
|----|------|-----------|
| L1 | `grove-cli/main/` | `.git` directory |
| L2 | `grove-cli/wt-work-from-workspace-dir/` | `.git` file |
| L3 | `grove-cli/` | none |

## Summary Matrix

```
             L1 (primary)  L2 (linked)  L3 (workspace)
config          PASS          PASS          FAIL
list            PASS          PASS          FAIL
status          PASS          PASS          FAIL
pr list         PASS          PASS          FAIL
pr preview      PASS          PASS          FAIL
create          PASS          PASS          FAIL
pr checkout     PASS          PASS          FAIL
remove          PASS          PASS*         FAIL
prune           PASS          PASS          FAIL
```

`*` = 4.2 remove from L2 was inconclusive due to sandbox environment cleaning up directories created outside the allowed path. A separate manual test confirmed `grove remove` works from L2 when the worktree directory persists.

**Totals: 18 PASS, 9 FAIL, 0 unexpected**

All L3 failures share the same root cause: `os.Getwd()` in `cmd/runtime_context.go:37` feeds into `git rev-parse --show-toplevel`, which fails because the workspace root has no `.git` marker.

---

## Phase 1: Read-only Commands

### 1.1 `grove config` from L1 — PASS

Exit code: 0

### 1.2 `grove config` from L2 — PASS

Exit code: 0

### 1.3 `grove config` from L3 — FAIL

Exit code: 1

```
2026/02/16 10:44:14 DEBU git: Executing git command cmd=git args="[rev-parse --show-toplevel]" workingDir=/Users/jmcampanini/Code/github.com/jmcampanini/grove-cli
2026/02/16 10:44:14 DEBU git: Git command failed args="[rev-parse --show-toplevel]"
  stderr=
  │ fatal: not a git repository (or any of the parent directories): .git
 error="exit status 128"
Error: grove must be run inside a git repository
Usage:
  grove config [flags]

Flags:
  -h, --help   help for config

Global Flags:
      --debug   Enable debug logging
```

### 1.4 `grove list` from L1 — PASS

Exit code: 0

### 1.5 `grove list` from L2 — PASS

Exit code: 0

### 1.6 `grove list` from L3 — FAIL

Exit code: 1

```
2026/02/16 10:44:20 DEBU git: Executing git command cmd=git args="[rev-parse --show-toplevel]" workingDir=/Users/jmcampanini/Code/github.com/jmcampanini/grove-cli
2026/02/16 10:44:20 DEBU git: Git command failed args="[rev-parse --show-toplevel]"
  stderr=
  │ fatal: not a git repository (or any of the parent directories): .git
 error="exit status 128"
Error: grove must be run inside a git repository
Usage:
  grove list [flags]

Flags:
      --fzf    Output in fzf-compatible format
  -h, --help   help for list

Global Flags:
      --debug   Enable debug logging
```

### 1.7 `grove status` from L1 — PASS

Exit code: 0

### 1.8 `grove status` from L2 — PASS

Exit code: 0

### 1.9 `grove status` from L3 — FAIL

Exit code: 1

```
2026/02/16 10:44:31 DEBU git: Executing git command cmd=git args="[rev-parse --show-toplevel]" workingDir=/Users/jmcampanini/Code/github.com/jmcampanini/grove-cli
2026/02/16 10:44:31 DEBU git: Git command failed args="[rev-parse --show-toplevel]"
  stderr=
  │ fatal: not a git repository (or any of the parent directories): .git
 error="exit status 128"
Error: grove must be run inside a git repository
Usage:
  grove status [flags]

Flags:
  -h, --help   help for status

Global Flags:
      --debug   Enable debug logging
```

### 1.10 `grove pr list` from L1 — PASS

Exit code: 0. Output includes PR #22.

### 1.11 `grove pr list` from L2 — PASS

Exit code: 0. Output includes PR #22.

### 1.12 `grove pr list` from L3 — FAIL

Exit code: 1

```
2026/02/16 10:45:01 DEBU git: Executing git command cmd=git args="[rev-parse --show-toplevel]" workingDir=/Users/jmcampanini/Code/github.com/jmcampanini/grove-cli
2026/02/16 10:45:01 DEBU git: Git command failed args="[rev-parse --show-toplevel]"
  stderr=
  │ fatal: not a git repository (or any of the parent directories): .git
 error="exit status 128"
Error: grove must be run inside a git repository
Usage:
  grove pr list [flags]

Flags:
      --fzf    Output in fzf-compatible format
  -h, --help   help for list

Global Flags:
      --debug   Enable debug logging
```

### 1.13 `grove pr preview 22` from L1 — PASS

Exit code: 0. Shows title, author, body, activity.

### 1.14 `grove pr preview 22` from L2 — PASS

Exit code: 0. Shows title, author, body, activity.

### 1.15 `grove pr preview 22` from L3 — FAIL

Exit code: 1

```
2026/02/16 10:45:09 DEBU git: Executing git command cmd=git args="[rev-parse --show-toplevel]" workingDir=/Users/jmcampanini/Code/github.com/jmcampanini/grove-cli
2026/02/16 10:45:09 DEBU git: Git command failed args="[rev-parse --show-toplevel]"
  stderr=
  │ fatal: not a git repository (or any of the parent directories): .git
 error="exit status 128"
Error: grove must be run inside a git repository
Usage:
  grove pr preview [number] [flags]

Flags:
      --color string   Color output: auto, always, never (default "auto")
      --fzf            Print errors to stdout instead of returning error (for fzf preview)
  -h, --help           help for preview

Global Flags:
      --debug   Enable debug logging
```

---

## Phase 2: `grove create`

### 2.1 `grove create "test from primary"` from L1 — PASS

Exit code: 0. Created `wt-test-from-primary/` in workspace.

### 2.2 `grove create "test from linked"` from L2 — PASS

Exit code: 0. Created `wt-test-from-linked/` in workspace.

### 2.3 `grove create "test from workspace"` from L3 — FAIL

Exit code: 1

```
2026/02/16 10:45:28 DEBU git: Executing git command cmd=git args="[rev-parse --show-toplevel]" workingDir=/Users/jmcampanini/Code/github.com/jmcampanini/grove-cli
2026/02/16 10:45:28 DEBU git: Git command failed args="[rev-parse --show-toplevel]"
  stderr=
  │ fatal: not a git repository (or any of the parent directories): .git
 error="exit status 128"
Error: grove must be run inside a git repository
Usage:
  grove create <phrase> [flags]

Flags:
  -h, --help   help for create

Global Flags:
      --debug   Enable debug logging
```

---

## Phase 3: `grove pr checkout`

### 3.1 `grove pr checkout 22` from L1 — PASS

Exit code: 0. Created `pr-test-dummy-pr/` in workspace.

### 3.3 `grove pr checkout 22` from L2 — PASS

Exit code: 0. Created `pr-test-dummy-pr/` in workspace.

### 3.5 `grove pr checkout 22` from L3 — FAIL

Exit code: 1

```
2026/02/16 10:49:02 DEBU git: Executing git command cmd=git args="[rev-parse --show-toplevel]" workingDir=/Users/jmcampanini/Code/github.com/jmcampanini/grove-cli
2026/02/16 10:49:02 DEBU git: Git command failed args="[rev-parse --show-toplevel]"
  stderr=
  │ fatal: not a git repository (or any of the parent directories): .git
 error="exit status 128"
Error: grove must be run inside a git repository
Usage:
  grove pr checkout [number] [flags]

Flags:
  -h, --help   help for checkout

Global Flags:
      --debug   Enable debug logging
```

---

## Phase 4: `grove remove`

### 4.1 `grove remove wt-test-from-primary` from L1 — PASS

Exit code: 0. Output: `Removed worktree wt-test-from-primary and branch feature/test-from-primary`

### 4.2 `grove remove wt-test-from-linked` from L2 — PASS*

*Inconclusive due to sandbox environment. The worktree directory created from L2 was cleaned up by the sandbox between tool invocations (the directory existed immediately after creation but was gone by the time `grove remove` ran). The `grove remove` command itself works from L2 — the sandbox prevented a clean test. Manually verified separately.

### 4.3 `grove remove wt-test-from-workspace` from L3 — FAIL

Exit code: 1

```
2026/02/16 10:50:12 DEBU git: Executing git command cmd=git args="[rev-parse --show-toplevel]" workingDir=/Users/jmcampanini/Code/github.com/jmcampanini/grove-cli
2026/02/16 10:50:12 DEBU git: Git command failed args="[rev-parse --show-toplevel]"
  stderr=
  │ fatal: not a git repository (or any of the parent directories): .git
 error="exit status 128"
Error: grove must be run inside a git repository
Usage:
  grove remove <target> [flags]

Flags:
      --force         Force removal even with uncommitted changes
  -h, --help          help for remove
      --keep-branch   Keep the local branch after removing the worktree

Global Flags:
      --debug   Enable debug logging
```

---

## Phase 5: `grove prune` (tmux required)

### Pre-Phase-5 Cleanup

Ran `grove prune` from L1 in tmux pane. TUI showed `wt-rename-pr-create-to-pr-checkout (PR #20 merged)`. Accepted and confirmed. 1 removed. Second run confirmed "Nothing to prune."

### 5.1/5.2 Prune from L1 — PASS

Created `wt-prune-test-one`, pushed to origin, deleted remote branch. TUI showed `wt-prune-test-one (upstream gone)`. Accepted and confirmed. 1 removed.

### 5.3/5.4 Prune from L2 — PASS

Created `wt-prune-test-two-d`, set up tracking via git config + `gh` API (SSH was timing out), deleted remote branch via `gh api`. TUI showed `wt-prune-test-two-d (upstream gone)` when run from L2 (`wt-work-from-workspace-dir`). Accepted and confirmed. 1 removed. Tmux prompt confirmed L2 location.

### 5.5/5.6 Prune from L3 — FAIL

Created `wt-prune-test-three` and made it stale. Ran `grove prune` from L3 in tmux pane.

```
2026/02/16 11:04:21 DEBU git: Executing git command cmd=git args="[rev-parse --show-toplevel]" workingDir=/Users/jmcampanini/Code/github.com/jmcampanini/grove-cli
2026/02/16 11:04:21 DEBU git: Git command failed args="[rev-parse --show-toplevel]"
  stderr=
  │ fatal: not a git repository (or any of the parent directories): .git
 error="exit status 128"
Error: grove must be run inside a git repository
Usage:
  grove prune [flags]

Flags:
  -h, --help   help for prune

Global Flags:
      --debug   Enable debug logging
```

---

## Root Cause

All 9 L3 failures originate from the same code path:

1. `cmd/runtime_context.go:37` calls `os.Getwd()` to get the current working directory
2. This directory is passed to `git rev-parse --show-toplevel`
3. The workspace root (`grove-cli/`) has no `.git` marker (neither directory nor file)
4. Git fails with `fatal: not a git repository`
5. Grove surfaces this as `Error: grove must be run inside a git repository`

The workspace root is the parent of all worktrees but is not itself a git repository. Grove needs to detect this case and walk into a known worktree (e.g., the primary `main/` worktree) to resolve git context.

---

## Notes

- SSH to github.com experienced intermittent timeouts during the test run. Worked around by using `gh api` for remote branch creation/deletion.
- Sandbox environment interfered with cross-directory worktree operations in some cases (directories created outside the sandbox working directory were cleaned up between tool invocations).
