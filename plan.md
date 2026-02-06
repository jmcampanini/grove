# Cleanup Plan: PR Worktree Feature Branch

## Context

The `feature/add-pr-commands` branch has all 6 phases of the PR worktree feature fully implemented and passing `make check`. However, WIP unstaged changes introduced TODO annotations and a partial method removal that breaks compilation. This plan addresses all TODOs, completes the partial refactor, and gets the branch to a clean mergeable state.

## Phases

### Phase 1: Remove unused methods from `PRWorktreeNamer` + fix tests

`HasPrefix` and `ExtractFromAbsolutePath` were removed from `internal/naming/pr.go` (unstaged) but tests still reference them. No production code uses these methods.

**Files:**
- `internal/naming/pr.go` — finalize removal of `HasPrefix()` and `ExtractFromAbsolutePath()`, remove `"path/filepath"` import
- `internal/naming/pr_test.go` — delete `TestPRWorktreeNamer_HasPrefix` (lines 240-292) and `TestPRWorktreeNamer_ExtractFromAbsolutePath` (lines 294-340)

**Success criteria:** `go build ./...` and `go test ./internal/naming/...` pass with no compilation errors.

**Validation approach:** Run `go build ./...` then `go test ./internal/naming/...`.

**Test coverage:** Delete `TestPRWorktreeNamer_HasPrefix` (5 cases) and `TestPRWorktreeNamer_ExtractFromAbsolutePath` (5 cases). No replacement tests needed — these methods are unused in production code.

**Known gaps:** None. The methods had no callers outside their own tests.

**Manual verification:**
1. Confirm `HasPrefix` and `ExtractFromAbsolutePath` do not appear anywhere in `internal/naming/pr.go`
2. Confirm `pr_test.go` has no references to either method
3. `go build ./...` succeeds

---

### Phase 2: Reconcile with origin/main

Merge `origin/main` into the feature branch to pick up any changes that landed while this branch was stale. Resolve any conflicts and ensure the combined result builds and passes all checks.

**Steps:**
1. `git fetch origin`
2. `git merge --no-gpg-sign origin/main`
3. Resolve any merge conflicts
4. Run `make check` to verify the merged result is clean

**Success criteria:** `make check` passes on the merged branch with no conflicts remaining.

**Validation approach:** Run `make check` after the merge completes.

**Test coverage:** No new tests. This phase validates that existing tests still pass after incorporating upstream changes.

**Known gaps:** If main has diverged significantly, conflicts may require judgment calls. Any non-trivial conflicts will be flagged for review before proceeding.

**Manual verification:**
1. `git log --oneline -5` shows the merge commit from origin/main
2. `git diff origin/main...HEAD` shows only the PR feature changes
3. `make check` passes cleanly

---

### Phase 3: Refactor `isValidBranchName` to return an error reason

Currently returns `bool`. Change to return `(bool, string)` where the string explains why validation failed.

**Files:**
- `internal/naming/pr.go` — change `isValidBranchName(name string) bool` to `isValidBranchName(name string) (bool, string)`, simplify the comment block since the reason string is self-documenting. Update the caller in `NewPRWorktreeNamer` to use the reason in the error message.
- `internal/naming/pr_test.go` — update `TestIsValidBranchName` to check both return values

**Success criteria:** `go test ./internal/naming/...` passes. `isValidBranchName` returns `(bool, string)` and `NewPRWorktreeNamer` uses the reason string in its error message.

**Validation approach:** Run `go test ./internal/naming/...`.

**Test coverage:** Update `TestIsValidBranchName` (14 existing cases) to assert on both the bool and the reason string. Add reason expectations for each failing case (e.g., `"contains '..'"`, `"starts with '-'"`, `"contains control character"`, `"empty"`).

**Known gaps:** The reason strings are for developer-facing error messages, not end-user display. No i18n consideration.

**Manual verification:**
1. Check `isValidBranchName` signature is `(bool, string)`
2. Check `NewPRWorktreeNamer` error message includes the reason (e.g., `"branch_template produces invalid branch name: contains '..'"`)
3. `go test ./internal/naming/... -run TestIsValidBranchName` passes

---

### Phase 4: Clean up `GetPullRequestFiles` JSON parsing

Use `PullRequestFile` directly with json tags instead of an anonymous struct.

**Files:**
- `internal/github/pull_request.go` — add json tags to `PullRequestFile` struct fields
- `internal/github/github_cli.go` — replace anonymous struct in `GetPullRequestFiles` with direct unmarshal into `[]PullRequestFile` wrapped in the outer `{"files": ...}` structure. The wrapper struct stays (gh returns `{"files": [...]}`), but inner elements use `PullRequestFile` directly.

**Success criteria:** `go test ./internal/github/...` passes. No anonymous struct duplication in `GetPullRequestFiles`.

**Validation approach:** Run `go test ./internal/github/...`.

**Test coverage:** No new tests needed — existing tests for `GetPullRequestFiles` in the mock-based command tests already verify the struct fields propagate correctly. The change is a pure refactor of internal JSON parsing.

**Known gaps:** The outer wrapper struct `{"files": [...]}` still uses an anonymous struct since it represents gh CLI's JSON envelope, not a domain type.

**Manual verification:**
1. `PullRequestFile` in `pull_request.go` has `json:"..."` tags on all fields
2. `GetPullRequestFiles` in `github_cli.go` uses `PullRequestFile` inside the wrapper, not an anonymous struct
3. `go build ./...` succeeds

---

### Phase 5: Refactor output functions to take `io.Writer`

Remove `cobra.Command` dependency from all output functions. This makes them independently testable.

**Files:**
- `cmd/pr_list.go`:
  - `outputPRListTable(cmd *cobra.Command, matches)` → `outputPRListTable(w io.Writer, matches)`
  - `outputPRListFzf(cmd *cobra.Command, matches)` → `outputPRListFzf(w io.Writer, matches)`
  - Update callers in `runPRList` to pass `cmd.OutOrStdout()`
- `cmd/pr_list_test.go` — pass `&buf` directly instead of `cmd.SetOut(&buf)` + cmd
- `cmd/pr_preview.go`:
  - `outputPRPreview(cmd *cobra.Command, pr, files)` → `outputPRPreview(w io.Writer, pr, files)`
  - Update caller in `runPRPreview` to pass `cmd.OutOrStdout()`
- `cmd/pr_preview_test.go` — pass `&buf` directly instead of using cobra.Command
- `cmd/pr_create.go`:
  - Inline `runPRCreateWithDeps` into `runPRCreate` (merge the dependency injection logic)
  - `createPRWorktree` signature changes: replace `cmd *cobra.Command` with `stdout io.Writer, stderr io.Writer`
  - Keep `prCreateDeps` and `prCreateContext` for testing, but the test entry point becomes `createPRWorktree` (the core logic) rather than the full CLI flow
- `cmd/pr_create_test.go` — update tests to call the refactored functions with `io.Writer` instead of cobra.Command

**Success criteria:** `go test ./cmd/...` passes. No `cobra.Command` in any output function signature. All output functions accept `io.Writer`.

**Validation approach:** Run `go test ./cmd/...`. Grep output function signatures to confirm they take `io.Writer`.

**Test coverage:** All existing tests are updated to pass `*bytes.Buffer` directly. Test count and coverage remain the same — this is a signature refactor, not a behavior change. The `pr_create_test.go` tests call `createPRWorktree(stdout, stderr, ...)` instead of `runPRCreateWithDeps(cmd, ...)`.

**Known gaps:** The `runPRList`, `runPRPreview`, and `runPRCreate` functions still use `cobra.Command` (they're the CLI entry points). Only the output/logic functions are decoupled. The `handlePreviewError` function still references the global `prPreviewFzfFlag` — this could be parameterized in a future refactor but is out of scope here.

**Manual verification:**
1. `outputPRListTable`, `outputPRListFzf`, `outputPRPreview` all take `io.Writer` as first param
2. `createPRWorktree` takes `stdout io.Writer, stderr io.Writer` instead of `cmd *cobra.Command`
3. No test file imports `cobra` solely for output buffer wiring (some tests may still use cobra for other reasons)

---

### Phase 6: Drop remaining TODOs and clean up matcher

Remove the TODOs that were decided against or are out of scope:

**Files:**
- `internal/config/config.go` — remove `// TODO: will need to figure out how to consolidate this and the worktreeConfig.NewPrefix` (this is a future consideration, not for this PR)
- `internal/pr/matcher.go` — remove `// TODO: this should only return matches` and `// TODO: also include the git.Worktree`. Fix struct field alignment (remove extra space between `HasWorktree` and `PR`).
- `internal/github/github.go` — the verbose `Validate()` doc comment was already trimmed in the unstaged diff; keep the trimmed version

**Success criteria:** `grep -r "TODO" cmd/ internal/` returns zero results for files touched by this branch (excluding pre-existing TODOs in `pull_request.go` PRQuery struct which are unrelated).

**Validation approach:** `grep -rn "TODO" cmd/pr_*.go internal/naming/pr.go internal/pr/matcher.go internal/github/github.go internal/github/github_cli.go internal/config/config.go` returns nothing.

**Test coverage:** No test changes. This phase is purely removing comments and fixing struct alignment.

**Known gaps:** The two pre-existing TODOs on `PRQuery` in `pull_request.go` (lines 33-34: "add ignore-users field" and "add default updated within days") are out of scope — they predate this cleanup and are future feature ideas.

**Manual verification:**
1. Open `internal/pr/matcher.go` — no TODO comments, `WorktreeMatch` fields are aligned
2. Open `internal/config/config.go` — no TODO on `WorktreePrefix`
3. Run the grep command above, confirm empty output

---

### Phase 7: Rename planning files + final validation

**Files:**
- The three files are already renamed to `original-context.md`, `original-plan.md`, `original-prompt.md` (done before planning)
- `.claude/settings.json` — discard the unstaged change (restore to committed version)

**Success criteria:** `make check` passes. `git diff` shows only intentional changes. No TODO comments in files touched by this branch (except the pre-existing `PRQuery` ones).

**Validation approach:** Run `make check`. Review `git status` to confirm all changes are intentional.

**Test coverage:** No test changes. This phase is file renames and discarding an unrelated settings change.

**Known gaps:** The `original-*.md` files are kept in the repo for historical context. They could be removed entirely if preferred, but keeping them is harmless and provides provenance.

**Manual verification:**
1. `ls original-*.md` shows three files
2. `.claude/settings.json` matches the committed version (plugins re-enabled)
3. `make check` passes cleanly

---

## Verification

1. Run `make check` — must pass with 0 lint issues and all tests green
2. `grep -rn "TODO" cmd/pr_*.go internal/naming/pr.go internal/pr/matcher.go internal/github/github.go internal/github/github_cli.go internal/config/config.go` — must return nothing
3. `go build ./...` — must succeed
