# Test Spec: Grove Commands Work From All Three Directory Types

## Context

Grove uses `os.Getwd()` in `cmd/runtime_context.go:37` to resolve git context via `git rev-parse --show-toplevel`. This works from inside worktrees but fails from the workspace root directory (which has no `.git` at all). We need a test spec that proves every user-facing command works from all three directory types.

## Locations

| ID | Path | Git marker | Description |
|----|------|-----------|-------------|
| L1 | `grove-cli/main/` | `.git` directory | Primary worktree |
| L2 | `grove-cli/wt-work-from-workspace-dir/` | `.git` file | Linked worktree |
| L3 | `grove-cli/` | none | Workspace root |

## Commands In Scope

`config`, `list`, `status`, `pr list`, `pr preview`, `create`, `pr checkout`, `remove`, `prune`

**Excluded:** `init` (no git context needed), `cache clear` (utility), `completion`, `help`

## Setup

### 1. Build grove binary
Build from the current worktree so we can invoke it via absolute path from any directory.
```
go build -o /tmp/grove-test .
```

### 2. Create dummy PR
From `main/` worktree:
```
git checkout -b test/dummy-pr
echo "test file for grove workspace testing" > test-dummy.txt
git add test-dummy.txt
git commit -m "test: dummy PR for workspace testing"
git push -u origin test/dummy-pr
gh pr create --title "Test PR - grove workspace testing" --body "Dummy PR for testing grove commands from workspace root"
gh pr comment $PR --body "First test comment"
gh pr comment $PR --body "Second test comment"
```
Record PR number as `$PR`.

---

## Test Sequence

### Phase 1: Read-only commands (all locations)

Each command runs once from each location. Success = exit code 0 + expected output.

| # | Location | Command | Success criteria |
|---|----------|---------|-----------------|
| 1.1 | L1 | `grove config` | Exit 0, prints TOML |
| 1.2 | L2 | `grove config` | Exit 0, prints TOML |
| 1.3 | L3 | `grove config` | Exit 0, prints TOML |
| 1.4 | L1 | `grove list` | Exit 0, lists worktree names |
| 1.5 | L2 | `grove list` | Exit 0, lists worktree names |
| 1.6 | L3 | `grove list` | Exit 0, lists worktree names |
| 1.7 | L1 | `grove status` | Exit 0, renders dashboard table |
| 1.8 | L2 | `grove status` | Exit 0, renders dashboard table |
| 1.9 | L3 | `grove status` | Exit 0, renders dashboard table |
| 1.10 | L1 | `grove pr list` | Exit 0, output includes PR `$PR` |
| 1.11 | L2 | `grove pr list` | Exit 0, output includes PR `$PR` |
| 1.12 | L3 | `grove pr list` | Exit 0, output includes PR `$PR` |
| 1.13 | L1 | `grove pr preview $PR` | Exit 0, shows title/author/body |
| 1.14 | L2 | `grove pr preview $PR` | Exit 0, shows title/author/body |
| 1.15 | L3 | `grove pr preview $PR` | Exit 0, shows title/author/body |

### Phase 2: `grove create` (all locations)

| # | Location | Command | Success criteria |
|---|----------|---------|-----------------|
| 2.1 | L1 | `grove create "test from primary"` | Exit 0, `wt-test-from-primary/` exists in workspace |
| 2.2 | L2 | `grove create "test from linked"` | Exit 0, `wt-test-from-linked/` exists in workspace |
| 2.3 | L3 | `grove create "test from workspace"` | Exit 0, `wt-test-from-workspace/` exists in workspace |

### Phase 3: `grove pr checkout` (all locations, with cleanup)

Each run creates `pr-$PR/` in the workspace. Must remove before the next run.

| # | Location | Command | Success criteria |
|---|----------|---------|-----------------|
| 3.1 | L1 | `grove pr checkout $PR` | Exit 0, `pr-$PR/` exists |
| 3.2 | — | `grove remove pr-$PR` (cleanup) | `pr-$PR/` removed |
| 3.3 | L2 | `grove pr checkout $PR` | Exit 0, `pr-$PR/` exists |
| 3.4 | — | `grove remove pr-$PR` (cleanup) | `pr-$PR/` removed |
| 3.5 | L3 | `grove pr checkout $PR` | Exit 0, `pr-$PR/` exists |
| 3.6 | — | `grove remove pr-$PR` (cleanup) | `pr-$PR/` removed |

### Phase 4: `grove remove` (all locations)

Uses the 3 worktrees created in Phase 2.

| # | Location | Command | Success criteria |
|---|----------|---------|-----------------|
| 4.1 | L1 | `grove remove wt-test-from-primary` | Exit 0, directory gone, branch deleted |
| 4.2 | L2 | `grove remove wt-test-from-linked` | Exit 0, directory gone, branch deleted |
| 4.3 | L3 | `grove remove wt-test-from-workspace` | Exit 0, directory gone, branch deleted |

### Phase 5: `grove prune` (all locations)

Prune is interactive (charmbracelet/huh TUI). Each run needs a prunable worktree. We create one fresh before each prune test, push it, then delete the remote branch to trigger the "upstream gone" reason.

**Before each prune test:**
```
grove create "prune test <N>"
cd wt-prune-test-<N> && git push -u origin feature/prune-test-<N>
git push origin --delete feature/prune-test-<N>
```

| # | Location | Command | Success criteria |
|---|----------|---------|-----------------|
| 5.1 | L1 | Create + make stale `wt-prune-test-one` | Worktree exists, upstream deleted |
| 5.2 | L1 | `grove prune` | TUI launches, shows `wt-prune-test-one` as "upstream gone", accept all → removed |
| 5.3 | L2 | Create + make stale `wt-prune-test-two` | Worktree exists, upstream deleted |
| 5.4 | L2 | `grove prune` | TUI launches, shows `wt-prune-test-two`, accept all → removed |
| 5.5 | L3 | Create + make stale `wt-prune-test-three` | Worktree exists, upstream deleted |
| 5.6 | L3 | `grove prune` | TUI launches, shows `wt-prune-test-three`, accept all → removed |

**Note:** Pre-existing stale worktrees are cleaned up before this phase (see Pre-Phase-5 Cleanup). Each prune run should show only our test worktree. TUI interaction: Enter to accept the pre-selected item → Enter to confirm removal.

---

## Pre-Phase-5 Cleanup

Before prune testing, remove any pre-existing stale worktrees so prune only shows our test ones:

1. Run `grove prune` from L1 (known working) to clear existing stale worktrees
2. Verify `grove prune` now reports "Nothing to prune."

---

## Teardown

1. Close the dummy PR: `gh pr close $PR --delete-branch`
2. Delete local test branch: `git branch -D test/dummy-pr`
3. Verify workspace is clean: `ls` workspace root, `grove list`

---

## Execution Approach

- Build the binary once, invoke via `/tmp/grove-test` from each location
- Use a tmux pane for interactive commands (prune)
- Run commands sequentially, recording exit code and abbreviated output for each
- For L3 tests: these will initially fail (proving the bug), then pass after the fix
- The test sequence runs in ~5 minutes end-to-end

## Full Test Matrix (27 cells)

```
             L1 (primary)  L2 (linked)  L3 (workspace)
config          1.1           1.2           1.3
list            1.4           1.5           1.6
status          1.7           1.8           1.9
pr list         1.10          1.11          1.12
pr preview      1.13          1.14          1.15
create          2.1           2.2           2.3
pr checkout     3.1           3.3           3.5
remove          4.1           4.2           4.3
prune           5.2           5.4           5.6
```
