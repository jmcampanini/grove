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

### 2. Ensure dummy PR exists
The test run requires an open PR with at least two comments. If one already exists from a previous run, reuse it. Only create a new one if needed.

**PR number: 22** (`https://github.com/jmcampanini/grove-cli/pull/22`)

From `main/` worktree, check for an existing open PR with our test title:
```
PR=$(gh pr list --repo jmcampanini/grove-cli --search "Test PR - grove workspace testing" --json number --jq '.[0].number')
```

If `$PR` is empty, create it:
```
git checkout -b test/dummy-pr
echo "test file for grove workspace testing" > test-dummy.txt
git add test-dummy.txt
git commit -m "test: dummy PR for workspace testing"
git push -u origin test/dummy-pr
gh pr create --title "Test PR - grove workspace testing" --body "Dummy PR for testing grove commands from workspace root"
PR=$(gh pr list --repo jmcampanini/grove-cli --search "Test PR - grove workspace testing" --json number --jq '.[0].number')
gh pr comment $PR --body "First test comment"
gh pr comment $PR --body "Second test comment"
```

If `$PR` is set, switch `main/` back to the main branch:
```
git checkout main
```

Verify: `echo $PR` should print `22`.

---

## Test Sequence

### Phase 1: Read-only commands (all locations)

Each command runs once from each location. Success = exit code 0 + expected output.

| # | Location | Command | Success criteria |
|---|----------|---------|-----------------|
| 1.1 | L1 | `grove --debug config` | Exit 0, prints TOML |
| 1.2 | L2 | `grove --debug config` | Exit 0, prints TOML |
| 1.3 | L3 | `grove --debug config` | Exit 0, prints TOML |
| 1.4 | L1 | `grove --debug list` | Exit 0, lists worktree names |
| 1.5 | L2 | `grove --debug list` | Exit 0, lists worktree names |
| 1.6 | L3 | `grove --debug list` | Exit 0, lists worktree names |
| 1.7 | L1 | `grove --debug status` | Exit 0, renders dashboard table |
| 1.8 | L2 | `grove --debug status` | Exit 0, renders dashboard table |
| 1.9 | L3 | `grove --debug status` | Exit 0, renders dashboard table |
| 1.10 | L1 | `grove --debug pr list` | Exit 0, output includes PR `$PR` |
| 1.11 | L2 | `grove --debug pr list` | Exit 0, output includes PR `$PR` |
| 1.12 | L3 | `grove --debug pr list` | Exit 0, output includes PR `$PR` |
| 1.13 | L1 | `grove --debug pr preview $PR` | Exit 0, shows title/author/body |
| 1.14 | L2 | `grove --debug pr preview $PR` | Exit 0, shows title/author/body |
| 1.15 | L3 | `grove --debug pr preview $PR` | Exit 0, shows title/author/body |

### Phase 2: `grove create` (all locations)

| # | Location | Command | Success criteria |
|---|----------|---------|-----------------|
| 2.1 | L1 | `grove --debug create "test from primary"` | Exit 0, `wt-test-from-primary/` exists in workspace |
| 2.2 | L2 | `grove --debug create "test from linked"` | Exit 0, `wt-test-from-linked/` exists in workspace |
| 2.3 | L3 | `grove --debug create "test from workspace"` | Exit 0, `wt-test-from-workspace/` exists in workspace |

### Phase 3: `grove pr checkout` (all locations, with cleanup)

Each run creates `pr-$PR/` in the workspace. Must remove before the next run.

| # | Location | Command | Success criteria |
|---|----------|---------|-----------------|
| 3.1 | L1 | `grove --debug pr checkout $PR` | Exit 0, `pr-$PR/` exists |
| 3.2 | — | `grove --debug remove pr-$PR` (cleanup) | `pr-$PR/` removed |
| 3.3 | L2 | `grove --debug pr checkout $PR` | Exit 0, `pr-$PR/` exists |
| 3.4 | — | `grove --debug remove pr-$PR` (cleanup) | `pr-$PR/` removed |
| 3.5 | L3 | `grove --debug pr checkout $PR` | Exit 0, `pr-$PR/` exists |
| 3.6 | — | `grove --debug remove pr-$PR` (cleanup) | `pr-$PR/` removed |

### Phase 4: `grove remove` (all locations)

Uses the 3 worktrees created in Phase 2.

| # | Location | Command | Success criteria |
|---|----------|---------|-----------------|
| 4.1 | L1 | `grove --debug remove wt-test-from-primary` | Exit 0, directory gone, branch deleted |
| 4.2 | L2 | `grove --debug remove wt-test-from-linked` | Exit 0, directory gone, branch deleted |
| 4.3 | L3 | `grove --debug remove wt-test-from-workspace` | Exit 0, directory gone, branch deleted |

### Phase 5: `grove prune` (all locations)

Prune uses a charmbracelet/huh TUI that requires a real TTY. **All prune commands must be run inside a tmux pane** — they will fail with `could not open a new TTY` if invoked from a non-interactive context.

Each run needs a prunable worktree. We create one fresh before each prune test, push it, then delete the remote branch to trigger the "upstream gone" reason.

**tmux pane setup:** Each interactive test should spin up a new tmux pane. Before starting a new pane, ensure the current test is complete, close the pane (`tmux kill-pane -t "$PANE_ID"`), and then move on to the next test.
```
PANE_ID=$(tmux split-window -h -l 50% -t "$TMUX_PANE" -P -F '#{pane_id}')
```

**Before each prune test** (setup can run non-interactively):

> **Note:** If SSH to github.com is timing out, use the `gh` CLI instead:
> - Push: `gh api repos/OWNER/REPO/git/refs -f ref=refs/heads/feature/prune-test-<N> -f sha=$(git rev-parse HEAD)`
> - Delete: `gh api repos/OWNER/REPO/git/refs/heads/feature/prune-test-<N> -X DELETE`
> - Then `git fetch --prune` (if SSH works) or manually: `git remote set-branches --add origin feature/prune-test-<N>` / `git config --unset branch.feature/prune-test-<N>.remote` etc.

```
grove --debug create "prune test <N>"
cd wt-prune-test-<N> && git push -u origin feature/prune-test-<N>
git push origin --delete feature/prune-test-<N>
```

**The `grove prune` command itself must be sent to the tmux pane:**
```
tmux send-keys -t "$PANE_ID" "cd <location> && /tmp/grove-test --debug prune" Enter
```
Then read the pane buffer with `tmux capture-pane -t "$PANE_ID" -p` to verify output, and send `Enter` keystrokes to interact with the TUI (accept selection → confirm removal).

| # | Location | Command | Success criteria |
|---|----------|---------|-----------------|
| 5.1 | L1 | Create + make stale `wt-prune-test-one` | Worktree exists, upstream deleted |
| 5.2 | L1 | `grove --debug prune` (in tmux pane) | TUI launches, shows `wt-prune-test-one` as "upstream gone", accept all → removed |
| 5.3 | L2 | Create + make stale `wt-prune-test-two` | Worktree exists, upstream deleted |
| 5.4 | L2 | `grove --debug prune` (in tmux pane) | TUI launches, shows `wt-prune-test-two`, accept all → removed |
| 5.5 | L3 | Create + make stale `wt-prune-test-three` | Worktree exists, upstream deleted |
| 5.6 | L3 | `grove --debug prune` (in tmux pane) | TUI launches, shows `wt-prune-test-three`, accept all → removed |

**Note:** Pre-existing stale worktrees are cleaned up before this phase (see Pre-Phase-5 Cleanup). Each prune run should show only our test worktree. TUI interaction: Enter to accept the pre-selected item → Enter to confirm removal.

---

## Pre-Phase-5 Cleanup

Before prune testing, remove any pre-existing stale worktrees so prune only shows our test ones. These must also run in a tmux pane (TUI requirement):

1. Run `grove prune` from L1 (in tmux pane) to clear existing stale worktrees
2. Verify `grove prune` now reports "Nothing to prune."

---

## Teardown

1. Close the dummy PR: `gh pr close $PR --delete-branch`
2. Delete local test branch: `git branch -D test/dummy-pr`
3. Verify workspace is clean: `ls` workspace root, `grove list`

---

## Execution Approach

- Build the binary once, invoke via `/tmp/grove-test` from each location
- **Prune commands require a tmux pane** (charmbracelet/huh TUI needs a real TTY)
- Run commands sequentially, recording exit code for each
- **Always pass `--debug`** on every grove command to get full `DEBU`-level log output (git commands executed, cache hits, etc.). Capture stderr alongside stdout (`2>&1`).
- For L3 tests: these will initially fail (proving the bug), then pass after the fix

### Results file

Build a single `results_<N>.md` file (e.g., `results_1.md`, `results_2.md`) incrementally as the test progresses. Increment the number to avoid overwriting previous runs.

1. **Before testing begins**, create `results.md` with a header, the date, binary path, PR number, location table, and an empty summary matrix.
2. **After each test**, append the result to the file immediately — don't wait until the end. Each entry should include:
   - Test number, location, command
   - Exit code
   - `PASS` or `FAIL`
   - For **failed** tests: the complete, untruncated stdout+stderr output in a fenced code block. Do not summarize.
   - For **passed** tests: a one-line confirmation is sufficient.
3. **After all tests complete**, go back and fill in the summary matrix at the top, then append a "Findings" section at the bottom with root cause analysis and any notes (SSH workarounds, sandbox issues, etc.).

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
