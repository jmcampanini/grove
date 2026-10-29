# grove

Grove manages git worktrees: one directory per branch, placed under a configurable root instead of next to the clone. It creates a branch and worktree from a phrase, a GitHub pull request, or a GitHub issue, and lists, syncs, and prunes the worktrees it finds. `grove sync` prompts before discarding uncommitted changes unless `--force` is passed, and `grove prune` removes worktrees and their branches only after you select and confirm them.


## Install

Grove publishes versioned binaries for Linux and macOS on amd64 and arm64, with prerequisites and channel-switching guidance in the [installation notes](docs/installation.md).

Install or upgrade the latest release with [mise](https://mise.jdx.dev), after installing Git and [GitHub CLI](https://cli.github.com):

```sh
mise use -g github:jmcampanini/grove
mise upgrade github:jmcampanini/grove
```

Install or upgrade with Homebrew, which also installs Git, GitHub CLI, and shell completions:

```sh
brew install --cask jmcampanini/tap/grove
brew upgrade --cask jmcampanini/tap/grove
```

For archive installs or upgrades, download your platform's [release archive](https://github.com/jmcampanini/grove/releases), then extract it and copy `grove` into a directory on `PATH`:

```sh
tar -xzf "<downloaded-archive>.tar.gz"
install -m 755 grove "<directory-on-PATH>/grove"
```

Install or upgrade the latest `main` commit with the alternative Homebrew HEAD formula:

```sh
brew tap jmcampanini/grove https://github.com/jmcampanini/grove
brew install --HEAD jmcampanini/grove/grove
brew upgrade --fetch-HEAD jmcampanini/grove/grove
```

Build from source with mise, Git, and GitHub CLI installed; [CONTRIBUTING.md](CONTRIBUTING.md) covers development and tool upgrades:

```sh
git clone https://github.com/jmcampanini/grove
cd grove
mise trust
mise install
mise run build
install -m 755 build/grove "<directory-on-PATH>/grove"
```


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

`grove --help`, each command's `--help`, and the help topics `grove help layout`, `grove help naming`, and `grove help exit-codes` are the canonical reference.


## Required external programs

- `git`: every command that reads or changes a repository runs it.
- `gh`: the `pr` and `issue` commands, `grove prune`, and the pull request data in `grove status` need it, and it must already be authenticated because grove disables its prompts.


## Configuration

Grove reads `$XDG_CONFIG_HOME/grove/config.toml` (or `~/.config/grove/config.toml`), then `grove.toml` from your home directory and each directory below it down to the main worktree, from the current worktree, and from the current directory. `grove config --help` documents the discovery order.

```toml
[worktree]
root = "~/Code/.worktrees"
```
