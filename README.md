# grove

Grove manages git worktrees as a workspace: one directory per branch, all siblings of the primary worktree. It creates a branch and worktree from a phrase, a GitHub pull request, or a GitHub issue, and lists, syncs, and prunes the worktrees it finds.

Command help is the canonical reference: `grove --help` and each command's `--help` describe every user-facing contract, `grove config --help` describes configuration precedence, `grove help workspace` describes the workspace layout, and `grove help exit-codes` describes exit statuses. `grove docs` prints a longer Markdown reference that supplements command help.

## Install

The Homebrew formula builds from the `main` branch HEAD; there is no tagged release.

### Homebrew (macOS)

```sh
brew tap jmcampanini/grove https://github.com/jmcampanini/grove
brew install --HEAD jmcampanini/grove/grove
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

## Representative commands

Run these from inside a workspace (see `grove help workspace`).

| Command | Result |
|---|---|
| `grove create "add user authentication"` | Create a branch and a sibling worktree from the phrase and print the worktree path. |
| `grove create "add user authentication" --from-remote-primary` | Same, but branch from the latest remote primary branch without updating the primary worktree; meant for automation. |
| `grove checkout feature/fix-login` | Check out an existing local or remote branch into a new worktree. |
| `grove pr checkout 42` | Check out a pull request into a worktree. |
| `grove issue start 17` | Create a branch and worktree to work on an issue. |
| `grove status` | Show a dashboard of every worktree with branch tracking, dirty state, and pull request information. |
| `grove sync` | Fetch and hard-reset the current branch to its remote tracking branch after confirmation. |
| `grove prune` | Interactively remove worktrees that are likely no longer needed. |
| `grove config --provenance` | Print each configuration value with its source. |

## Required external programs

Grove runs `git` for every worktree, git, pull request, and issue command, and `gh` for the `pr` and `issue` commands and `grove prune`. Both must be on `PATH`. Grove disables their interactive prompts (`GIT_TERMINAL_PROMPT=0`, `GH_PROMPT_DISABLED=1`), so `gh` must already be authenticated. `grove status` asks `gh` for pull request data and shows a worktree as local when that call fails. Grove never runs `fzf`; the `--fzf` flags only shape output for a pipeline you build.

## Configuration

Grove reads `grove.toml` from the XDG config directory, from your home directory and each directory below it down to the main worktree, the current worktree, and the current directory, in that order, with later files overriding earlier ones and the `--worktree-template` flag overriding them all; `grove config --help` documents the full precedence, `grove config` prints the values in effect, and `grove docs` prints the schema.
