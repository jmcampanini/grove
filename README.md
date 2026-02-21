grove-cli
=========

A simple CLI tool used to easily create and use multiple git worktrees using a workspace.


Workspace Structure
-------------------

A workspace is a directory containing only git repo roots (the main repo and it's worktrees) as subdirectories.

```
~/Code/org-name/repo-name/  # workspace root (matches remote URL)
├── main/                   # the only full git repository
├── wt-add-auth/            # worktree for feature branch
├── wt-fix-bug-123/         # worktree for bug fix branch
└── pr-456/                 # worktree from pull request (future feature)
```

**Requirements:**

1. Workspace directory name must match the end of the git remote origin URL
2. To be in a workspace, you must be at or below the workspace root directory
3. The workspace only contains git repository or worktrees roots as subdirectories


Installation
------------

### Homebrew (macOS)

```sh
brew tap jmcampanini/grove-cli https://github.com/jmcampanini/grove-cli
brew install --HEAD jmcampanini/grove-cli/grove
```

To upgrade to the latest commit:

```sh
brew upgrade --fetch-HEAD grove
```

### From source

```sh
make install
```


Development
-----------

1. Install `golangci-lint`
2. Run `make help`
