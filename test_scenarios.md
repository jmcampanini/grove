# Create Command Test Scenarios

Test scenarios for `cmd/create.go` — the `grove create <phrase>` command.

All scenarios use default config: `feature/` branch prefix, `wt-` worktree prefix,
slugify with lowercase, collapse dashes, max length 50, hash length 4.

## Scenario 1: Simple phrase (happy path)

- **Input phrase:** `"add logging support"`
- **Expected branch:** `feature/add-logging-support`
- **Expected worktree name:** `wt-add-logging-support`
- **Expected stdout:** `<workspace>/wt-add-logging-support`

## Scenario 2: Special characters

- **Input phrase:** `"fix: handle 404 & 500 errors!"`
- **Expected branch:** `feature/fix-handle-404-500-errors`
- **Expected worktree name:** `wt-fix-handle-404-500-errors`
- **Exercises:** Non-alphanumeric chars (`&`, `:`, `!`) replaced with dashes, collapsed

## Scenario 3: Mixed casing

- **Input phrase:** `"Add OAuth2 Google Integration"`
- **Expected branch:** `feature/add-oauth2-google-integration`
- **Expected worktree name:** `wt-add-oauth2-google-integration`
- **Exercises:** Uppercase → lowercase conversion

## Scenario 4: Long phrase triggers hash truncation

- **Input phrase:** `"implement comprehensive user authentication and authorization system with role based access"`
- **Expected:** Branch slug portion ≤ 50 chars, ends with 4-char hash, worktree starts with `wt-`
- **Exercises:** MaxLength truncation + hash suffix

## Scenario 5: Duplicate branch

- **Input phrase:** `"add logging support"`
- **Setup:** `branchExistsFn` returns `true`
- **Expected error contains:** `already exists`

## Scenario 6: Empty phrase

- **Input phrase:** `"   "` (whitespace only)
- **Expected error contains:** `phrase cannot be empty`

## Scenario 7: All special characters (slugifies to empty)

- **Input phrase:** `"@#$%^&*"`
- **Expected error contains:** `empty branch name after slugification`

## Scenario 8: Worktree path already exists on disk

- **Input phrase:** `"add logging support"`
- **Setup:** Create directory at expected worktree path before running
- **Expected error contains:** `already exists`

## Scenario 9: Workspace path error

- **Setup:** `getWorkspacePathFn` returns error
- **Expected error contains:** `failed to get workspace path`

## Scenario 10: Worktree creation error

- **Setup:** `createWorktreeForNewBranchFromRefFn` returns error
- **Expected error contains:** `failed to create branch and worktree`
