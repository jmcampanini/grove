# Context: PR Worktree Feature for grove-cli

## What is grove-cli?

Grove-cli is a Git worktree workspace manager written in Go. It helps developers easily create and manage Git worktrees with intelligent naming and configuration.

### Current Commands
- `grove list` - Lists all worktrees (supports `--fzf` flag)
- `grove create <phrase>` - Creates a new branch and worktree from a phrase
- `grove init <shell>` - Generates shell integration functions

### Shell Functions
- `grc` - Grove create: creates worktree and cd's into it
- `grs` - Grove switch: fzf selection of worktrees and cd's into selection

### Key Internal Packages
- `internal/config/` - TOML configuration loading with defaults
- `internal/git/` - Git CLI wrapper with worktree operations
- `internal/github/` - GitHub interface using `gh` CLI (already has PR listing/fetching)
- `internal/naming/` - Branch and worktree name generation with slugification
- `internal/shell/` - Embedded shell scripts for bash/zsh/fish

---

## Feature Request

Add the ability to create worktrees from GitHub pull requests with three new commands:

### `grove pr list`
- Lists open PRs for the current repository
- Shows: PR number, title, author, branch, created date, updated date, local status
- Default output: lipgloss table (pretty formatted)
- With `--fzf`: tab-separated format for fzf parsing
- Indicates which PRs already have local worktrees

### `grove pr preview <number>`
- Shows detailed PR information
- Designed for use as fzf `--preview` command
- Shows: title, author, branch, state, dates, files changed, PR body

### `grove pr create <number>`
- Creates a local worktree from a PR
- Fetches the remote branch
- Creates local branch using configurable template
- Creates worktree with `pr-` prefix
- Idempotent: returns existing path if already created

### `grp` Shell Function
- Interactive fzf selection of PRs with preview
- On selection, creates worktree (if needed) and cd's into it

---

## User Design Decisions

1. **Command structure**: Nested (`grove pr list`) vs flat (`grove pr-list`)
   - **Choice**: Nested

2. **Branch naming for PR worktrees**: How to name local branches
   - **Choice**: Configurable template with `{{.BranchName}}` and `{{.Number}}`
   - Default: `{{.BranchName}}` (uses remote branch name)

3. **Duplicate handling**: What happens if `pr create` run twice on same PR
   - **Choice**: Return existing worktree path (idempotent)

4. **Worktree prefix**: What prefix for PR worktree directories
   - **Choice**: `pr-` (separate from regular `wt-` prefix)

5. **Shell function name**: Following grc/grs pattern
   - **Choice**: `grp` (grove pr)

---

## Codebase Research Summary

### Existing Patterns Discovered

**Cobra command structure** (from `cmd/list.go`):
- Commands use `RunE` with error return
- Flags defined in `init()` function
- Output via `cmd.OutOrStdout()`

**FZF integration** (from `cmd/list.go` and shell scripts):
- Tab-separated format: `<data>\t<display>`
- fzf uses `--with-nth 2` to show display, `cut -f1` extracts data
- Shell functions check for `z` (zoxide) fallback to `cd`

**Config system** (from `internal/config/`):
- TOML format with struct tags
- Defaults in `defaults.go`
- Validation in `Config.Validate()`
- Discovery from multiple paths (cwd, worktree root, home)

**GitHub package** (from `internal/github/`):
- Already has `ListPullRequests(query, limit)` and `GetPullRequest(num)`
- `PullRequest` struct has: Number, Title, BranchName, AuthorLogin, AuthorName, Body, CreatedAt, UpdatedAt, FilesChanged, State
- Uses `gh` CLI under the hood

**Git operations** (from `internal/git/`):
- `FetchRemoteBranch(remote, remoteRef, localRef)` - fetches and creates local branch
- `CreateWorktreeForExistingBranch(branchName, path)` - creates worktree from branch
- `ListWorktrees()` - returns all worktrees with branch info

**Naming package** (from `internal/naming/`):
- `WorktreeNamer` handles prefix and slugification
- Pattern to follow for `PRWorktreeNamer`

### Libraries for Table Output
- `github.com/charmbracelet/lipgloss/table` - for pretty table rendering
- Already using charmbracelet/log in the project

---

## New Config Section

```toml
[pr]
branch_template = "{{.BranchName}}"  # or "pr/{{.Number}}"
worktree_prefix = "pr-"
```

Template variables:
- `{{.BranchName}}` - PR's head branch (e.g., "feature/add-auth")
- `{{.Number}}` - PR number (e.g., 123)
