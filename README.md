# grove

Manage git worktrees as a workspace: one directory per branch, all siblings.

## Install

### Homebrew (macOS)

```sh
brew tap jmcampanini/grove-cli https://github.com/jmcampanini/grove-cli
brew install --HEAD jmcampanini/grove-cli/grove
```

Upgrade to the latest commit:

```sh
brew upgrade --fetch-HEAD grove
```

### From source

```sh
make build
# then copy ./build/grove to a directory on your PATH
```

## Quickstart

From inside a workspace (see `grove help workspace`):

```sh
grove create "add user authentication"
```

Grove creates a branch, adds a worktree for it as a sibling of `main/`, and prints the new worktree path.

For automation that should always start from the latest remote primary branch without updating the primary worktree:

```sh
grove create "add user authentication" --from-remote-primary
```

## Reference

For the full command reference, run `grove --help` or `grove docs`. Topic pages:

- `grove help workspace` — workspace layout and requirements
- `grove help exit-codes` — exit codes and error categories
- `grove help logs` — where logs go and how to inspect them
