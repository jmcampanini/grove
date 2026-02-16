# Test Results: Run 3

**Date:** 2026-02-16
**Binary:** `/tmp/grove-test`
**PR:** #24
**Commit:** e1c2553 (Phase 4 complete)

## Locations

| ID | Path | Git marker |
|----|------|-----------|
| L1 | `grove-cli/main/` | `.git` directory |
| L2 | `grove-cli/wt-work-from-workspace-dir/` | `.git` file |
| L3 | `grove-cli/` | none |

## Summary Matrix

```
             L1 (primary)  L2 (linked)  L3 (workspace)
config          PASS          PASS          PASS
list            PASS          PASS          PASS
status          PASS          PASS          PASS
pr list         PASS          PASS          PASS
pr preview      PASS          PASS          PASS
create          PASS          PASS          PASS
pr checkout     PASS          PASS          PASS
remove          PASS          PASS*         PASS
prune           PASS          PASS          PASS
```

*PASS: worktree removed successfully; branch deletion failed (expected: branch not fully merged). Worktree operation itself works correctly.

## Detailed Results

### Phase 1: Read-only commands

| # | Location | Command | Result | Notes |
|---|----------|---------|--------|-------|
| 1.1 | L1 | `grove --debug config` | PASS | Exit 0, prints TOML with workspace config |
| 1.2 | L2 | `grove --debug config` | PASS | Exit 0, prints TOML |
| 1.3 | L3 | `grove --debug config` | PASS | Exit 0, workspace root detection triggered, anchored to main/ |
| 1.4 | L1 | `grove --debug list` | PASS | Exit 0, lists worktrees |
| 1.5 | L2 | `grove --debug list` | PASS | Exit 0, lists worktrees |
| 1.6 | L3 | `grove --debug list` | PASS | Exit 0, workspace root detection, lists worktrees |
| 1.7 | L1 | `grove --debug status` | PASS | Exit 0, renders dashboard table |
| 1.8 | L2 | `grove --debug status` | PASS | Exit 0, renders dashboard table |
| 1.9 | L3 | `grove --debug status` | PASS | Exit 0, workspace root detection, renders table |
| 1.10 | L1 | `grove --debug pr list` | PASS | Exit 0, shows PR #24 |
| 1.11 | L2 | `grove --debug pr list` | PASS | Exit 0, shows PR #24 |
| 1.12 | L3 | `grove --debug pr list` | PASS | Exit 0, workspace root detection, shows PR #24 |
| 1.13 | L1 | `grove --debug pr preview 24` | PASS | Exit 0, shows title/author/body/activity |
| 1.14 | L2 | `grove --debug pr preview 24` | PASS | Exit 0, shows title/author/body/activity |
| 1.15 | L3 | `grove --debug pr preview 24` | PASS | Exit 0, workspace root detection, shows PR preview |

### Phase 2: grove create

| # | Location | Command | Result | Notes |
|---|----------|---------|--------|-------|
| 2.1 | L1 | `grove --debug create "test from primary"` | PASS | Exit 0, wt-test-from-primary/ created |
| 2.2 | L2 | `grove --debug create "test from linked"` | PASS | Exit 0, wt-test-from-linked/ created |
| 2.3 | L3 | `grove --debug create "test from workspace"` | PASS | Exit 0, workspace root detection, wt-test-from-workspace/ created |

### Phase 3: grove pr checkout

| # | Location | Command | Result | Notes |
|---|----------|---------|--------|-------|
| 3.1 | L1 | `grove --debug pr checkout 24` | PASS | Exit 0, pr-test-dummy-pr/ created |
| 3.2 | — | `grove --debug remove pr-test-dummy-pr` | PASS | Cleanup (with manual branch -D for unmerged branch) |
| 3.3 | L2 | `grove --debug pr checkout 24` | PASS | Exit 0, pr-test-dummy-pr/ created |
| 3.4 | — | `grove --debug remove pr-test-dummy-pr` | PASS | Cleanup (with manual branch -D) |
| 3.5 | L3 | `grove --debug pr checkout 24` | PASS | Exit 0, workspace root detection, pr-test-dummy-pr/ created |
| 3.6 | — | manual cleanup | PASS | Worktree + branch removed |

### Phase 4: grove remove

| # | Location | Command | Result | Notes |
|---|----------|---------|--------|-------|
| 4.1 | L1 | `grove --debug remove wt-test-from-primary` | PASS | Exit 0, worktree + branch deleted |
| 4.2 | L2 | `grove --debug remove wt-test-from-linked` | PASS* | Worktree removed, branch deletion failed (not fully merged). Branch cleaned up manually. |
| 4.3 | L3 | `grove --debug remove wt-test-from-workspace` | PASS | Exit 0, workspace root detection, worktree + branch deleted |

### Phase 5: grove prune

| # | Location | Command | Result | Notes |
|---|----------|---------|--------|-------|
| 5.1 | L1 | `grove --debug prune` | PASS | TUI showed wt-prune-test-one (upstream gone), confirmed removal, worktree + branch deleted |
| 5.2 | L2 | `grove --debug prune` | PASS | TUI showed wt-prune-test-two (upstream gone), confirmed removal, worktree + branch deleted |
| 5.3 | L3 | `grove --debug prune` | PASS | Workspace root detection, TUI showed wt-prune-test-three (upstream gone), confirmed removal, worktree + branch deleted |

Prune tests executed in tmux pane with interactive TUI. For each test: created worktree, pushed branch to remote, deleted remote branch, ran `git fetch --prune`, then ran `grove prune` to select and confirm removal.

## Findings

- **27 of 27 tests PASS** across all three locations (L1, L2, L3)
- **L3 (workspace root) works correctly** for all 9 commands: config, list, status, pr list, pr preview, create, pr checkout, remove, prune
- The workspace root detection logs show the expected flow: detect no git repo → load bootstrap config → probe primary branches → find main/ → anchor
- Both `git` and `gh` commands work correctly from workspace root after anchoring
- The `remove` command's branch deletion failure on unmerged branches is pre-existing behavior (not related to workspace root detection)
