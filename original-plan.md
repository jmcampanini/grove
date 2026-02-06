# Plan: PR Worktree Commands for grove-cli

## Summary
Add `grove pr list`, `grove pr preview`, `grove pr create` commands and `grp` shell function to manage GitHub pull request worktrees.

## User Preferences (Confirmed)
- **Command structure**: Nested (`grove pr list`)
- **Branch naming**: Configurable template with `{{.BranchName}}`, `{{.Number}}`; default `{{.BranchName}}`
- **Duplicate handling**: Return existing worktree path (idempotent)
- **Worktree prefix**: `pr-` (separate from regular `wt-`)
- **Shell function**: `grp`

## Dependencies
- **go-humanize**: `github.com/dustin/go-humanize` for relative time formatting ("1h ago")

---

## Implementation Phases

### Phase 1: Config Changes

**Files to modify:**
- `internal/config/config.go` - Add `PRConfig` struct, add to `Config` struct
- `internal/config/defaults.go` - Add PR defaults to `DefaultConfig()`

**defaults.go addition:**
```go
func DefaultConfig() Config {
    return Config{
        // ... existing fields ...
        PR: PRConfig{
            BranchTemplate: "{{.BranchName}}",
            WorktreePrefix: "pr-",
        },
    }
}
```

```go
// Add between Git and Slugify (alpha order)
type PRConfig struct {
    BranchTemplate   string `toml:"branch_template"`   // default: "{{.BranchName}}"
    WorktreePrefix   string `toml:"worktree_prefix"`   // default: "pr-"
}
```

**Validation** in `Validate()`:
```go
// Require non-empty worktree prefix to distinguish PR worktrees from regular ones
if c.PR.WorktreePrefix == "" {
    return fmt.Errorf("pr.worktree_prefix cannot be empty")
}
// Note: Template validation (parsing, field checking, branch name rules) is deferred
// to PRWorktreeNamer constructor to avoid type duplication and circular imports.
```

---

### Phase 2: PR Naming Package

**File to create:** `internal/naming/pr.go`

```go
type PRTemplateData struct {
    BranchName string  // PR's head branch (e.g., "feature/add-auth")
    Number     int     // PR number (e.g., 123)
}

type PRWorktreeNamer struct {
    branchTemplate *template.Template
    worktreePrefix string
    slugifyOpts    SlugifyOptions
}

func NewPRWorktreeNamer(prCfg config.PRConfig, slugCfg config.SlugifyConfig) (*PRWorktreeNamer, error)
func (n *PRWorktreeNamer) GenerateBranchName(pr github.PullRequest) (string, error)
func (n *PRWorktreeNamer) GenerateWorktreeName(branchName string) string
func (n *PRWorktreeNamer) HasPrefix(name string) bool
func (n *PRWorktreeNamer) ExtractFromAbsolutePath(absPath string) string
```

**Constructor validates template** (moved from config.Validate to use real PRTemplateData type):
```go
func NewPRWorktreeNamer(prCfg config.PRConfig, slugCfg config.SlugifyConfig) (*PRWorktreeNamer, error) {
    // 1. Parse template
    tmpl, err := template.New("branch").Parse(prCfg.BranchTemplate)
    if err != nil {
        return nil, fmt.Errorf("invalid branch_template: %w", err)
    }

    // 2. Execute with test data to verify fields exist
    var buf bytes.Buffer
    testData := PRTemplateData{BranchName: "test/branch", Number: 1}
    if err := tmpl.Execute(&buf, testData); err != nil {
        return nil, fmt.Errorf("branch_template uses invalid field: %w", err)
    }

    // 3. Validate output is valid git branch name
    if !isValidBranchName(buf.String()) {
        return nil, fmt.Errorf("branch_template produces invalid branch name: %s", buf.String())
    }

    return &PRWorktreeNamer{...}, nil
}

// isValidBranchName validates git branch name with simplified rules.
// Checks only the most common invalid patterns:
// - No ".." anywhere
// - No control characters (ASCII < 32, DEL)
// - No leading "-"
// Edge cases not covered here will fail at git worktree creation time
// with clear git error messages.
func isValidBranchName(name string) bool
```

Note: Follows same pattern as `WorktreeNamer`. `HasPrefix` and `ExtractFromAbsolutePath`
are needed for display formatting in `pr list`.

**Tests:** `internal/naming/pr_test.go` - Table-driven tests for various templates.

---

### Phase 3: PR Matcher Utility

**File to create:** `internal/pr/matcher.go`

```go
type Matcher struct {
    namer *naming.PRWorktreeNamer
}

type WorktreeMatch struct {
    PR            github.PullRequest
    HasWorktree   bool
    WorktreePath  string
}

func NewMatcher(namer *naming.PRWorktreeNamer) *Matcher

func (m *Matcher) Match(prs []github.PullRequest, worktrees []git.Worktree) []WorktreeMatch

func (m *Matcher) FindWorktreeForPR(pr github.PullRequest, worktrees []git.Worktree) *git.Worktree
```

**Matching Logic**: The matcher checks if any worktree's local branch name matches either:
1. The template-generated branch name for the PR, OR
2. The PR's remote branch name directly (catches manually created worktrees)

```go
func (m *Matcher) FindWorktreeForPR(pr github.PullRequest, worktrees []git.Worktree) *git.Worktree {
    // Apply template to get expected local branch name
    expectedBranch, err := m.namer.GenerateBranchName(pr)
    if err != nil {
        expectedBranch = "" // Continue with direct match only
    }

    // Search worktrees for matching branch
    for i := range worktrees {
        if branch, ok := worktrees[i].Ref.FullBranch(); ok {
            // Match 1: Template-generated branch name (grove pr create)
            if expectedBranch != "" && branch.Name == expectedBranch {
                return &worktrees[i]
            }
            // Match 2: PR's remote branch name directly (manual worktrees)
            if branch.Name == pr.BranchName {
                return &worktrees[i]
            }
        }
    }
    return nil
}
```

**Note**: This matches both worktrees created via `grove pr create` (template-generated names)
AND manually created worktrees that use the PR's exact branch name. This provides broader
detection at the cost of potential false positives if unrelated branches happen to share names.

**Tests:** `internal/pr/matcher_test.go`

---

### Phase 4: Commands

**Files to create:**

1. `cmd/pr.go` - Parent command
   ```go
   var prCmd = &cobra.Command{
       Use:   "pr",
       Short: "Manage pull request worktrees",
   }
   ```

2. `cmd/pr_list.go` - List PRs with worktree status
   - `--fzf` flag for fzf-compatible output
   - Default: lipgloss table with columns: **#, Title, Author, Branch, State, Local, Updated**
   - "State" column shows lowercase state: "open", "draft", "closed", "merged" (use `strings.ToLower(string(pr.State))` for display). Lowercase is intentional for visual consistency in tables. Future-proofed for `--state` flag.
   - **Query**: Use `PRQuery{State: "open"}` explicitly to lock behavior. Add comment noting future extension for `--state` flag.
   - "Local" column shows ✓ when worktree exists, empty otherwise
   - "Updated" column uses `humanize.Time(pr.UpdatedAt)` from go-humanize
   - **Sanitization**: Replace tabs and newlines with spaces in ALL columns going into the fzf TSV (title, branch, author, state) to prevent parsing issues. While git branch names can't contain these characters, author names from GitHub could theoretically contain them.

   **Table format:**
   ```
   | #   | Title       | Author | Branch      | State | Local | Updated |
   |-----|-------------|--------|-------------|-------|-------|---------|
   | 123 | Add feature | user   | feature/add | open  | ✓     | 1h ago  |
   | 124 | Fix bug     | other  | fix/bug     | draft |       | 3d ago  |
   ```

   **FZF format:** `<number>\t<searchable>\t<display>`

   Three columns with distinct purposes:
   - **Column 1 (Extract)**: PR number - extracted via `cut -f1` after selection for `pr create`, also passed to preview via `{1}`
   - **Column 2 (Search)**: Searchable content - fzf searches this column, hidden from display
   - **Column 3 (Display)**: Pretty formatted string - shown via `--with-nth 3`

   **Column 2 searchable content:** Space-concatenated: number, title, branch, author, state
   - Example: `123 Add authentication feature/add-auth jsmith open`
   - **Note**: PR body text is intentionally excluded. While the original prompt requested body search,
     the current approach (title, branch, author, state) is deemed sufficient. Body text would require
     sanitization (newlines, tabs break fzf parsing) and significantly increase line length.
   - Includes state for filtering (e.g., type "draft" to find draft PRs)

   **Column 3 display:** `✓ #123 Add authentication [jsmith] feature/add-auth`

   **Example output line:**
   ```
   123\t123 Add auth feature/add-auth jsmith open\t✓ #123 Add auth [jsmith] feature/add-auth
   ```

3. `cmd/pr_preview.go` - Show PR details
   - Takes PR number as argument
   - Two sequential API fetches (PR info + files), ~400-1000ms latency is acceptable
   - **Note**: The original prompt suggested progressive loading (show instant data, then fetch more).
     Sequential API calls are simpler and the latency is acceptable for preview use. This is a deliberate
     simplification. Parallelization can be added in v2 if needed.
   - **`--fzf` flag**: Changes error handling behavior
     - Without flag (default): Return errors normally (proper exit code, stderr)
     - With `--fzf`: Print errors to stdout and return nil (for fzf preview pane)

   ```go
   var fzfMode bool // set via --fzf flag

   func runPreview(cmd *cobra.Command, args []string) error {
       prNum := parseNumber(args[0])

       pr, err := gh.GetPullRequest(prNum)
       if err != nil {
           if fzfMode {
               // Print to stdout so error displays in fzf preview pane
               fmt.Printf("Error: %v\n", err)
               return nil
           }
           return err
       }

       files, err := gh.GetPullRequestFiles(prNum)
       if err != nil {
           if fzfMode {
               fmt.Printf("Error: %v\n", err)
               return nil
           }
           return err
       }

       fmt.Printf("PR #%d\n", pr.Number)
       fmt.Printf("─────────────────────────────\n")
       fmt.Printf("Title:  %s\n", pr.Title)
       fmt.Printf("Author: %s\n", pr.AuthorLogin)
       fmt.Printf("Branch: %s\n", pr.BranchName)
       fmt.Printf("State:  %s\n", pr.State)
       fmt.Printf("\n")

       // Show file list with +/- counts (limit to 30 files)
       const maxFiles = 30
       fmt.Printf("Files changed (%d):\n", pr.FilesChanged)
       displayCount := len(files)
       if displayCount > maxFiles {
           displayCount = maxFiles
       }
       for _, f := range files[:displayCount] {
           fmt.Printf("  %s (+%d, -%d)\n", f.Path, f.Additions, f.Deletions)
       }
       if len(files) > maxFiles {
           fmt.Printf("  (and %d more files...)\n", len(files)-maxFiles)
       }

       fmt.Printf("\n%s\n", pr.Body)
       return nil
   }
   ```

   **Note**: Requires adding `GetPullRequestFiles` to `internal/github/` package.
   **Limitation**: GitHub API returns max 30 files per page; we accept this limit and don't
   implement pagination. Display is limited to 30 with "(and N more files...)" indicator.

   ```go
   // Add to internal/github/github.go interface
   GetPullRequestFiles(prNum int) ([]PullRequestFile, error)

   // Validate checks if gh CLI is available and authenticated.
   // Returns nil if ready to use, or a descriptive error:
   // - "gh CLI not found: install from https://cli.github.com"
   // - "gh CLI not authenticated: run 'gh auth login'"
   // Note: Does not check if current directory is a GitHub repo.
   // Non-GitHub repos will fail naturally when gh commands are executed.
   Validate() error
   ```

   **Validate() implementation** in `github_cli.go`:
   ```go
   func (g *GitHubCli) Validate() error {
       // Check if gh is installed
       if _, err := exec.LookPath("gh"); err != nil {
           return fmt.Errorf("gh CLI not found: install from https://cli.github.com")
       }

       // Check auth status - gh auth status exits non-zero if not authenticated
       if _, err := g.executeGhCommand("auth", "status"); err != nil {
           return fmt.Errorf("gh CLI not authenticated: run 'gh auth login'")
       }

       return nil
   }
   ```

   **Usage in commands**: Call `gh.Validate()` at the start of each pr subcommand before other operations.

   ```go
   // Add to internal/github/pull_request.go
   type PullRequestFile struct {
       Additions int
       Deletions int
       Path      string
   }
   ```

4. `cmd/pr_create.go` - Create worktree from PR
   - Takes PR number
   - Idempotent: returns existing path if worktree exists (user runs `git pull` to update)
   - **Fork limitation documented:**
   ```go
   Long: `Create a local worktree from a GitHub pull request.

   Note: Only works with PRs from the same repository. Fork PRs are not yet supported.`,
   ```

   **Flow:**
   1. Fetch PR info via `gh.GetPullRequest(num)`
   2. **Detect fork PRs (fail fast)** → return specific error if fork (see below)
   3. **Warn if PR is merged/closed** → print to stderr: `"Note: PR #%d is %s"` but continue
   4. Generate local branch name via template
   5. **Check if worktree already exists using Matcher** (fetch all worktrees, use `FindWorktreeForPR`)
      - If exists → output path to stdout, print "Worktree already exists" to stderr, return
   6. **Check for path collision**: Generate expected worktree path, then `os.Stat()` to check if it exists.
      If path exists but Matcher didn't find a matching worktree, return error:
      `"worktree path %s already exists (not a PR worktree or different branch)"`
   7. **Check if local branch already exists** (retry-friendly): `git.BranchExists(localBranch, false)`
      - If exists → skip fetch, proceed to step 9
   8. Fetch remote branch: `git.FetchRemoteBranch("origin", pr.BranchName, localBranch)`
   9. Create worktree: `git.CreateWorktreeForExistingBranch(localBranch, wtPath)`
   10. Output absolute path to stdout

   **Note**: `FetchRemoteBranch` and `CreateWorktreeForExistingBranch` already exist in `internal/git/git.go`.

   **Fork detection** (using IsCrossRepository field):
   ```go
   // Detect fork PRs immediately after fetching PR info (fail fast)
   if pr.IsCrossRepository {
       return fmt.Errorf("PR #%d is from a fork, which is not yet supported.\nTip: You can manually add the fork as a remote and create a worktree with 'git worktree add'", pr.Number)
   }
   ```

   **Note**: Requires adding `IsCrossRepository` field to `PullRequest` struct.
   Maps directly to GitHub API field `isCrossRepository` (boolean).

**Tests:** `cmd/pr_list_test.go`, `cmd/pr_preview_test.go`, `cmd/pr_create_test.go`

---

### Phase 5: Shell Scripts

**Files to create:**
- `internal/shell/scripts/grp.bash`
- `internal/shell/scripts/grp.zsh`
- `internal/shell/scripts/grp.fish`

Pattern (bash/zsh) - follows existing `grs` script conventions:
```bash
# FZF column layout: <number>\t<searchable>\t<display>
#   --with-nth 3   → show column 3 (pretty display)
#   {1}            → PR number for pr create and preview
#   cut -f1        → extract PR number after selection
grp() {
    local pr_num
    pr_num=$(grove pr list --fzf | fzf \
        --delimiter '\t' \
        --with-nth 3 \
        --preview 'grove pr preview --fzf {1}' \
        --preview-window 'right:50%:wrap:delay:300' \
        | cut -f1)
    if [ -n "$pr_num" ]; then
        # Validate PR number is numeric (defensive check)
        if ! [[ "$pr_num" =~ ^[0-9]+$ ]]; then
            echo "Invalid PR number: $pr_num" >&2
            return 1
        fi
        local output
        # Don't redirect stderr - let info/error messages display to terminal (matches grs pattern)
        if output=$(grove pr create "$pr_num"); then
            # Prefer zoxide (z) when available (same pattern as grs)
            if command -v z &> /dev/null; then
                z "$output"
            else
                cd "$output"
            fi
        else
            return 1
        fi
    fi
}
```

Fish script follows existing `grs.fish` patterns (uses `set -l`, `test`, `function...end`).

**File to modify:** `internal/shell/functions.go`
- Add `//go:embed` directives for grp scripts
- Update `GenerateFish()`, `GenerateZsh()`, `GenerateBash()` to include grp

---

### Phase 6: grove list Enhancement

**File to modify:** `cmd/list.go`

Add `[PR]` marker to distinguish PR worktrees in `grove list` output:
- Check if worktree directory name starts with `pr.worktree_prefix` (default: `pr-`)
- Display `[PR]` tag before the worktree name in both table and fzf output
- Example: `[PR] pr-feature-auth` vs `wt-main`

```go
// In list display logic
func formatWorktreeName(name string, prPrefix string) string {
    if strings.HasPrefix(name, prPrefix) {
        return "[PR] " + name
    }
    return name
}
```

---

## PR Number to Worktree Path Transformation

Complete end-to-end flow showing how a PR number becomes a worktree path:

**Example Configuration:**
```toml
[pr]
branch_template = "{{.BranchName}}"
worktree_prefix = "pr-"

[slugify]
lowercase = true
replace_non_alphanum = true
collapse_dashes = true
```

**Transformation Steps:**

```
Input: PR #123
  └─ BranchName: "feature/add-auth"
  └─ Workspace: /Users/me/code/myrepo

Step 1: Apply branch_template
  Template: "{{.BranchName}}"
  Data: {BranchName: "feature/add-auth", Number: 123}
  Result: "feature/add-auth"

Step 2: Apply slugify to branch name
  Input: "feature/add-auth"
  Operations: lowercase, replace / with -, collapse dashes
  Result: "feature-add-auth"

Step 3: Apply worktree_prefix
  Input: "feature-add-auth"
  Prefix: "pr-"
  Result: "pr-feature-add-auth"

Step 4: Join with workspace path
  Workspace: /Users/me/code/myrepo
  Directory: "pr-feature-add-auth"
  Result: /Users/me/code/myrepo/pr-feature-add-auth

Output: /Users/me/code/myrepo/pr-feature-add-auth
```

**Alternative Template Example:**

```
Input: PR #123 with branch_template = "pr/{{.Number}}"

Step 1: Apply template → "pr/123"
Step 2: Slugify → "pr-123"
Step 3: Smart prefix check: "pr-123" starts with "pr-" → skip prefix
Step 4: Join → /Users/me/code/myrepo/pr-123
```

---

## Critical Files

| File | Action |
|------|--------|
| `internal/config/config.go` | Add PRConfig struct |
| `internal/config/defaults.go` | Add PR defaults to `DefaultConfig()` |
| `internal/naming/pr.go` | NEW: PR naming logic, `isValidBranchName()` |
| `internal/pr/matcher.go` | NEW: PR-worktree matcher |
| `internal/github/github.go` | Add `GetPullRequestFiles(num)` and `Validate()` to interface |
| `internal/github/github_cli.go` | Implement `GetPullRequestFiles` (follows existing pattern); implement `Validate()` for gh CLI detection |
| `internal/github/pull_request.go` | Add `IsCrossRepository` field; update `prJsonFields` to include `isCrossRepository` |
| `cmd/pr.go` | NEW: Parent command |
| `cmd/pr_list.go` | NEW: List command |
| `cmd/pr_preview.go` | NEW: Preview command |
| `cmd/pr_create.go` | NEW: Create command |
| `cmd/list.go` | Add `[PR]` marker for worktrees matching `pr.worktree_prefix` |
| `internal/shell/scripts/grp.{bash,zsh,fish}` | NEW: Shell functions |
| `internal/shell/functions.go` | Add grp embeds |

---

## Key Design Decisions

1. **Template-based branch naming**: Flexible, users can use `{{.BranchName}}` or `pr/{{.Number}}`
2. **Separate `pr-` prefix**: Distinguishes PR worktrees from regular `wt-` worktrees
3. **Idempotent create**: Safe to run multiple times, just returns existing path. User runs `git pull` manually to update stale worktrees.
4. **Dual-match strategy for worktree detection**: For each PR, check worktrees for either (a) the template-generated local branch name, or (b) the PR's remote branch name directly. This catches both `grove pr create` worktrees and manually created ones.
5. **Lipgloss tables**: Beautiful default output, degraded to 3-column tab-separated for fzf
6. **3-column fzf format (deliberate divergence)**: `<number>\t<searchable>\t<display>` differs from `grove list`'s 2-column `<path>\t<display>` format. This is an **intentional and deliberate** design choice:
   - `grove list`: Path IS the extractable value AND part of display. 2 columns sufficient.
   - `grove pr list`: PR number is extractable, but searchable content (title, author, branch, state) differs from pretty display. 3 columns required.
   - Consistency was considered and rejected: forcing 2 columns would degrade search UX or display quality.
7. **Preview fetches via API**: Each preview makes two sequential API calls (~400-1000ms). Simpler than caching or parallelization, acceptable latency for preview use.
8. **File list in preview**: Shows actual file paths with +/- line counts, not just a total count
9. **Minimal error handling**: Wrap errors with context, let them propagate naturally
10. **Mock GitHub interface for testing**: Unit tests use mock implementation of GitHub interface; no integration tests with real gh CLI
11. **Specific gh CLI errors**: Detect missing/unauthenticated gh CLI and show actionable error messages

---

## Execution Order

1. Config changes (PRConfig)
2. Naming package (PRWorktreeNamer)
3. Matcher utility
4. Parent `pr` command
5. `pr list` command
6. `pr preview` command
7. `pr create` command
8. Shell scripts (`grp`)
9. `grove list` enhancement ([PR] marker)
10. Run `make check` to validate

---

## Known Limitations (v1)

1. **Fork PRs not supported**: Only works with PRs from the same repository. PRs from forks require fetching from the forker's remote, which is not implemented. Fork PRs are detected via `IsCrossRepository` field and a specific error message is shown.

2. **Config changes may break matching**: If you change `branch_template` after creating PR worktrees, previously created worktrees may not be detected as "Local" in `pr list`. The matcher regenerates expected branch names using the current config.

3. **State flag deferred**: `pr list` only shows open PRs. Flags like `--state closed` or `--all` are deferred to a future version.

4. **No JSON output**: `--json` flag for scriptability is deferred.

5. **No rate limiting protection**: Preview API calls are made on each fzf cursor movement. Shell scripts use fzf's `delay:300` option to debounce, but rapid scrolling may still trigger many requests. Additional caching deferred to v2 if users report issues. **Note**: `delay:300` requires fzf 0.32+; older versions may make more preview API calls.

6. **Preview parallelization deferred**: Two sequential API calls in preview (~400-1000ms) could be parallelized in v2 if latency becomes a concern.

7. **Smart prefix detection edge case**: Branches already starting with the `worktree_prefix` pattern (e.g., `pr-hotfix/bug` with prefix `pr-`) will not get the prefix added again. This prevents `pr-pr-*` names but means such branches won't have distinguishable PR worktree directories. This is rare in practice and the behavior is consistent once understood.

8. **FZF search is fuzzy across all fields**: The searchable column concatenates number, title, branch, author, and state with spaces. Searching "jsmith" will match PRs with "jsmith" in the title as well as PRs authored by jsmith. This is acceptable for fuzzy search UX.

9. **Dual-match may return non-template worktrees**: The matcher checks both template-generated names AND PR remote branch names. This means `grove pr create` may return an existing worktree that uses a different branch name than your template would generate. For example, with template `pr/{{.Number}}`, if you manually created a worktree for branch `feature/add-auth`, running `grove pr create 123` (where PR #123 has that branch) returns the existing worktree rather than creating `pr-123`. This is intentional to avoid duplicate worktrees.

10. **Preview file list limited to 30 files**: GitHub API returns max 30 files per page. We accept this limit and don't implement pagination. The header shows actual count via `pr.FilesChanged`, and "(and N more files...)" appears when truncated.

11. **Shell script UX improvements deferred**: Scripts don't check for fzf installation or provide helpful error messages. TODO for future version to improve shell function consistency and UX.

12. **Template validation uses test data**: Branch template is validated at config load with synthetic test data (`BranchName: "test/branch", Number: 1`). If a real PR has a branch name that produces an invalid git branch (e.g., contains `..`), the error occurs at create-time, not config-time.

13. **No PR cleanup command**: There's no `grove pr clean` to remove worktrees for merged/closed PRs. Users should manually remove PR worktrees using `git worktree remove`. A dedicated cleanup command is deferred to v2.

14. **grove list mixes PR and regular worktrees**: `grove list` shows all worktrees together, but PR worktrees are visually distinguished with a `[PR]` marker based on the configured `pr.worktree_prefix`.

---

## Design Decision: Smart Prefix Detection

### Problem

When using a branch template that includes a prefix pattern (e.g., `branch_template = "pr/{{.Number}}"`), the slugify + prefix flow produces redundant prefixes:

```
branch_template = "pr/{{.Number}}"   worktree_prefix = "pr-"

"pr/123" → slugify → "pr-123" → add prefix → "pr-pr-123"  ← PROBLEM
```

### Solution: Smart Detection

The `PRWorktreeNamer.GenerateWorktreeName` method will check if the slugified branch name already starts with the `worktree_prefix`. If so, skip adding the prefix.

**Note**: This behavior is intentionally different from `WorktreeNamer.Generate()`, which always adds the prefix. PR templates are more likely to include prefix patterns (e.g., `pr/{{.Number}}`), making smart detection more valuable here. This divergence is documented and accepted.

**Implementation in `internal/naming/pr.go`:**
```go
func (n *PRWorktreeNamer) GenerateWorktreeName(branchName string) string {
    slug := Slugify(branchName, n.slugifyOpts)
    if slug == "" {
        return ""
    }

    // Smart detection: skip prefix if slug already starts with it
    if strings.HasPrefix(slug, n.worktreePrefix) {
        return slug
    }

    return n.worktreePrefix + slug
}
```

**Examples:**

| Branch Template | Slugified | Prefix | Result |
|-----------------|-----------|--------|--------|
| `pr/{{.Number}}` → `pr/123` | `pr-123` | `pr-` | `pr-123` (prefix skipped) |
| `{{.BranchName}}` → `feature/add-auth` | `feature-add-auth` | `pr-` | `pr-feature-add-auth` |
| `{{.BranchName}}` → `pr-fix/bug` | `pr-fix-bug` | `pr-` | `pr-fix-bug` (prefix skipped) |

### Known Edge Cases

1. **Branches starting with prefix pattern** (e.g., `pr-fix/bug`) will not get the prefix added. This is acceptable:
   - Such branch names are rare in practice
   - If a branch is already named `pr-*`, the user likely doesn't want `pr-pr-*`
   - The behavior is consistent and predictable once understood

### Updated Transformation Flow

```
Input: PR #123 with branch_template = "pr/{{.Number}}"

Step 1: Apply template → "pr/123"
Step 2: Slugify → "pr-123"
Step 3: Smart prefix check: "pr-123" starts with "pr-" → skip prefix
Step 4: Result: "pr-123"

Output: /Users/me/code/myrepo/pr-123 ✓
```
