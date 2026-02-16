# Grove Test Spec Results

**Date:** 2026-02-16
**Binary:** `/tmp/grove-test` (built from `wt-work-from-workspace-dir`)
**PR used:** #21 (closed after testing)

## Locations

| ID | Path | Git marker |
|----|------|-----------|
| L1 | `grove-cli/main/` | `.git` directory |
| L2 | `grove-cli/wt-work-from-workspace-dir/` | `.git` file |
| L3 | `grove-cli/` | none |

---

## Phase 1: Read-only commands

| # | Location | Command | Result | Notes |
|---|----------|---------|--------|-------|
| 1.1 | L1 | `grove config` | PASS | Exit 0, prints TOML config |
| 1.2 | L2 | `grove config` | PASS | Exit 0, prints TOML config |
| 1.3 | L3 | `grove config` | **FAIL** | Exit 1: `grove must be run inside a git repository` |
| 1.4 | L1 | `grove list` | PASS | Exit 0, lists 3 worktrees |
| 1.5 | L2 | `grove list` | PASS | Exit 0, lists 3 worktrees |
| 1.6 | L3 | `grove list` | **FAIL** | Exit 1: `grove must be run inside a git repository` |
| 1.7 | L1 | `grove status` | PASS | Exit 0, renders dashboard table |
| 1.8 | L2 | `grove status` | PASS | Exit 0, renders dashboard table |
| 1.9 | L3 | `grove status` | **FAIL** | Exit 1: `grove must be run inside a git repository` |
| 1.10 | L1 | `grove pr list` | PASS | Exit 0, shows PR #21 |
| 1.11 | L2 | `grove pr list` | PASS | Exit 0, shows PR #21 |
| 1.12 | L3 | `grove pr list` | **FAIL** | Exit 1: `grove must be run inside a git repository` |
| 1.13 | L1 | `grove pr preview 21` | PASS | Exit 0, shows title/author/body/activity |
| 1.14 | L2 | `grove pr preview 21` | PASS | Exit 0, shows title/author/body/activity |
| 1.15 | L3 | `grove pr preview 21` | **FAIL** | Exit 1: `grove must be run inside a git repository` |

## Phase 2: `grove create`

| # | Location | Command | Result | Notes |
|---|----------|---------|--------|-------|
| 2.1 | L1 | `grove create "test from primary"` | PASS | Exit 0, `wt-test-from-primary/` created |
| 2.2 | L2 | `grove create "test from linked"` | PASS | Exit 0, `wt-test-from-linked/` created |
| 2.3 | L3 | `grove create "test from workspace"` | **FAIL** | Exit 1: `grove must be run inside a git repository` |

## Phase 3: `grove pr checkout`

| # | Location | Command | Result | Notes |
|---|----------|---------|--------|-------|
| 3.1 | L1 | `grove pr checkout 21` | PASS | Exit 0, `pr-test-dummy-pr/` created |
| 3.2 | — | `grove remove pr-test-dummy-pr` | PASS | Removed (also deleted local branch) |
| 3.3 | L2 | `grove pr checkout 21` | PASS | Exit 0, `pr-test-dummy-pr/` created |
| 3.4 | — | `grove remove pr-test-dummy-pr` | PASS | Removed (also deleted local branch) |
| 3.5 | L3 | `grove pr checkout 21` | **FAIL** | Exit 1: `grove must be run inside a git repository` |
| 3.6 | — | (cleanup skipped) | N/A | Nothing to clean up |

**Note:** The worktree was named `pr-test-dummy-pr` (from the branch name), not `pr-21` as the spec implied.

## Phase 4: `grove remove`

| # | Location | Command | Result | Notes |
|---|----------|---------|--------|-------|
| 4.1 | L1 | `grove remove wt-test-from-primary` | **PARTIAL FAIL** | Exit 1. Directory was removed, but branch delete failed: `the branch 'feature/test-from-primary' is not fully merged`. Worktree removal succeeded, branch deletion did not. |
| 4.2 | L2 | `grove remove wt-test-from-linked` | **PARTIAL FAIL** | Exit 1. Same as 4.1 — directory removed, branch delete failed (not fully merged). |
| 4.3 | L3 | `grove remove wt-test-from-workspace` | **FAIL** | Exit 1: `grove must be run inside a git repository`. (Target never existed since 2.3 failed.) |

**Root cause for 4.1/4.2:** The branches were never pushed or merged, so `git branch -d` refuses to delete them. This is normal git behavior — not a grove bug per se, but grove exits non-zero even though the worktree removal itself succeeded. The error handling could be improved (e.g., suggest `--force` or `--keep-branch`).

## Phase 5: `grove prune`

| # | Location | Command | Result | Notes |
|---|----------|---------|--------|-------|
| 5.1–5.2 | L1 | `grove prune` | **UNTESTABLE** | TUI requires a TTY: `could not open a new TTY: open /dev/tty: device not configured` |
| 5.3–5.4 | L2 | `grove prune` | **UNTESTABLE** | Same TTY error |
| 5.5–5.6 | L3 | `grove prune` | **FAIL** | Exit 1: `grove must be run inside a git repository` (fails before TUI) |

---

## Summary

### Overall matrix

```
             L1 (primary)  L2 (linked)  L3 (workspace)
config          PASS          PASS          FAIL
list            PASS          PASS          FAIL
status          PASS          PASS          FAIL
pr list         PASS          PASS          FAIL
pr preview      PASS          PASS          FAIL
create          PASS          PASS          FAIL
pr checkout     PASS          PASS          FAIL
remove          PARTIAL       PARTIAL       FAIL
prune           UNTESTABLE    UNTESTABLE    FAIL
```

### Failure categories

**1. All L3 tests fail with the same error (9 tests)**
- Error: `grove must be run inside a git repository`
- Root cause: `os.Getwd()` in `cmd/runtime_context.go:37` calls `git rev-parse --show-toplevel` which fails because the workspace root has no `.git` marker.
- This is the primary bug the spec is designed to prove.

**2. `grove remove` partial failures on L1/L2 (2 tests)**
- The worktree directory is removed successfully, but the command exits non-zero because `git branch -d` refuses to delete unmerged branches.
- This is not the workspace-root bug — it's an error-handling issue where grove could handle the "not fully merged" case more gracefully (e.g., warn instead of error, or prompt for force delete).

**3. `grove prune` untestable from non-TTY (2 tests)**
- The charmbracelet/huh TUI requires `/dev/tty`, which is unavailable in this execution context.
- L3 still confirmed to fail with the git repo error (before the TUI even launches).

### Counts

- **PASS:** 14
- **FAIL (L3 workspace root):** 9
- **PARTIAL FAIL (remove branch delete):** 2
- **UNTESTABLE (prune TUI):** 2
- **Total tests attempted:** 27
