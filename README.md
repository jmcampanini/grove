# grove

Grove manages git worktrees: one directory per branch, placed under a configurable root instead of next to the clone. It creates a branch and worktree from a phrase, a GitHub pull request, or a GitHub issue, and lists, syncs, and prunes the worktrees it finds. `grove sync` prompts before discarding uncommitted changes unless `--force` is passed, and `grove prune` removes worktrees and their branches only after you select and confirm them.


## Install

Grove is configured to publish versioned binaries for Linux and macOS on amd64 and arm64. Until the first stable release is published, use the HEAD formula or build from source below.


### Versioned releases

After the first stable release, install or upgrade with mise or the shared Homebrew tap:

```sh
mise use -g github:jmcampanini/grove
mise upgrade github:jmcampanini/grove
brew install --cask jmcampanini/tap/grove
brew upgrade --cask jmcampanini/tap/grove
```

Or download the archive for your OS and architecture from the [releases page](https://github.com/jmcampanini/grove/releases), extract it, and copy `grove` onto your `PATH`. To upgrade, download the new release and replace the binary. Releases include SHA-256 checksums and GitHub build provenance attestations. On macOS, if a browser download is quarantined, remove the attribute with `xattr -d com.apple.quarantine grove` before running it.

Homebrew installs Git, `gh`, and bash/zsh/fish completions. For archive or mise installs, install Git and [GitHub CLI](https://cli.github.com) separately. Run `gh auth login` before using Grove's GitHub commands.


### HEAD formula (macOS alternative)

The existing formula builds the latest `main` commit and remains available independently of versioned releases:

```sh
brew tap jmcampanini/grove https://github.com/jmcampanini/grove
brew install --HEAD jmcampanini/grove/grove
```

Upgrade to the latest commit:

```sh
brew upgrade --fetch-HEAD jmcampanini/grove/grove
```

The HEAD formula and release cask both install `grove`; keep one installed at a time. To switch to releases after the first stable release, uninstall the HEAD formula with `brew uninstall --formula jmcampanini/grove/grove` before installing the cask.


### From source

Install [mise](https://mise.jdx.dev), Git, and `gh` (`brew install git gh mise` on macOS), then run:

```sh
git clone https://github.com/jmcampanini/grove
cd grove
mise trust
mise install
mise run build
```

Copy `./build/grove` to a directory on your `PATH`.


## Representative commands

Run these from inside a repository: the clone or any of its worktrees.

| Command | Result |
|---|---|
| `grove create "add user authentication"` | Create a branch and worktree from the phrase and print the worktree path. |
| `grove create "run tests" --space claude` | Same, placed under the `claude` space. |
| `grove create "add user authentication" --from-remote-primary` | Same, but branch from the latest remote primary branch without updating the primary worktree; meant for automation. |
| `grove checkout feature/fix-login` | Check out an existing local or remote branch into a new worktree. |
| `grove pr checkout 42` | Check out a pull request into a worktree. |
| `grove issue start 17` | Create a branch and worktree to work on an issue. |
| `grove status` | Show a dashboard of every worktree with branch tracking, dirty state, and pull request information. |
| `grove sync` | Fetch and hard-reset the current branch to its remote tracking branch. |
| `grove prune` | Interactively remove worktrees that are likely no longer needed. |

`grove --help`, each command's `--help`, and the help topics `grove help layout` and `grove help exit-codes` are the canonical reference; `grove docs` supplements them.


## Required external programs

- `git`: every command that reads or changes a repository runs it.
- `gh`: the `pr` and `issue` commands, `grove prune`, and the pull request data in `grove status` need it, and it must already be authenticated because grove disables its prompts.


## Configuration

Grove reads `$XDG_CONFIG_HOME/grove/config.toml` (or `~/.config/grove/config.toml`), then `grove.toml` from your home directory and each directory below it down to the main worktree, from the current worktree, and from the current directory. `grove config --help` documents the discovery order.

```toml
[worktree]
root = "~/Code/.worktrees"
```
