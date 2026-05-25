# Plan: `grove create --from-remote-primary`

## Goal

Add a Grove-native way to create a new worktree branch from the latest remote primary/default branch without requiring callers to manually resolve the default branch or update the primary worktree.

Target workflow:

```sh
grove create "add user authentication" --from-remote-primary
```

This should behave like:

1. Determine the repository's default remote, falling back to `origin`.
2. Determine that remote's primary/default branch from remote HEAD.
3. Narrow-fetch only that branch from the remote. If this requires a new git interface method, flag to the user before proceeding.
4. Create the new branch/worktree from `<remote>/<primary-branch>`.
5. Print only the created worktree path to stdout.

## Non-goals

- Do not update, reset, clean, or otherwise modify the primary worktree.
- Do not perform a broad remote fetch unless needed by existing behavior elsewhere.

## CLI design

Add a boolean flag to `grove create`:

```sh
grove create <phrase> --from-remote-primary
```

Flag behavior:

- Mutually exclusive with `--from`.
- Mutually exclusive with `--reuse`.
- Keeps stdout compatible with existing `grove create`: stdout is the new worktree path only.
- Any fetch/default-branch failures should return normal command errors.

## Implementation steps

1. Update `cmd/create.go`:
   - Register `--from-remote-primary`.
   - Read the flag in `runCreate`.
   - Add a field to `createContext`.
   - Validate incompatible combinations:
     - `--from` with `--from-remote-primary`
     - `--reuse` with `--from-remote-primary`
   - Before creating the branch, resolve the effective base ref:
     - If `--from-remote-primary` is false, preserve existing `--from` behavior.
     - If true:
       1. `GetDefaultRemote("origin")`
       2. `GetRepoDefaultBranch(remote)`
       3. Error if no default branch is configured.
       4. Narrow-fetch that branch.
       5. Use `<remote>/<branch>` as the base ref.

2. Update `internal/git/git.go`:
   - Add a Git interface method for narrow-fetching a remote branch.
   - The method should update the corresponding remote-tracking ref used by `<remote>/<branch>`.

3. Update `internal/git/git_cli.go`:
   - Implement the new narrow-fetch method.
   - Prefer a scoped fetch equivalent to fetching only the default branch from the selected remote.

4. Update tests:
   - `cmd/create_test.go`:
     - Success path fetches default remote primary and creates from `origin/main`.
     - Custom default branch, e.g. `origin/develop`.
     - Custom default remote.
     - Missing remote HEAD/default branch returns a clear error.
     - Fetch failure propagates.
     - `--from` and `--from-remote-primary` are incompatible.
     - `--reuse` and `--from-remote-primary` are incompatible.
     - stdout remains path-only.
   - Git CLI tests, if appropriate:
     - Narrow fetch updates the expected remote-tracking branch.

5. Update docs/help text:
   - Add the flag to `grove create` help.
   - Mention it as the recommended automation-friendly way to start new work from the latest remote primary branch.

## Expected behavior examples

```sh
grove create "add auth" --from-remote-primary
# stdout:
# /path/to/workspace/wt-add-auth
```

If remote HEAD points to `origin/main`, the new branch starts from `origin/main` after a narrow fetch of `main`.

If remote HEAD points to `upstream/trunk` because the default remote is `upstream`, the new branch starts from `upstream/trunk` after fetching `trunk` from `upstream`.

## Validation

Run:

```sh
make check
```
