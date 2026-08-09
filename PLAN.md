# Plan: unified naming templates (issue #117)

Replace the implicit worktree-name derivation (prefix + strip + wholesale slugify) with explicit, per-flow naming templates, and put issue/PR numbers in worktree directory names. Clean break: old config keys and flags survive only in git history and the changelog.

## Problem

- PR worktrees carry no PR number (`pr-feature-revalidate-prune`), so directories are hard to identify from autocomplete/fuzzy-find. Issue worktrees get numbers only incidentally, via branch-name derivation.
- Naming is spread across three inconsistent mechanisms: `branch_prefix` (string), `branch_template` (template), and `worktree_prefix` + `strip_branch_prefix` + wholesale slugify (implicit derivation).

Matching is branch-anchored (`internal/issue/matcher.go`, `internal/pr/matcher.go`), so worktree directory renames break nothing: no migration of existing worktrees is needed and none is planned.

## Target configuration

```toml
[naming]
strip_prefixes       = ["feature/", "fix/", "issue/"]
max_length           = 30        # caps every generated name; 0 disables
collapse_dashes      = true
lowercase            = true
replace_non_alphanum = true
trim_dashes          = true

[local_branch]
branch_template   = "feature/{{.PhraseSlug}}"
worktree_template = "wt-{{.BranchSlug}}"

[issue]
branch_template   = "issue/{{.Number}}-{{.TitleSlug}}"
worktree_template = "is-{{.Number}}-{{.TitleSlug}}"

[pull_request]
branch_template   = "{{.Branch}}"      # never truncated — must match the remote
worktree_template = "pr-{{.Number}}-{{.TitleSlug}}"
```

Removed entirely (clean break): `[slugify]` section, `slugify.max_length`, `slugify.hash_length`, `issue.title_slug_max_length`, `issue.strip_branch_prefix`, `issue.worktree_prefix`, `local_branch.branch_prefix`, `local_branch.strip_branch_prefix`, `local_branch.worktree_prefix`, `pull_request.worktree_prefix`, and the `--worktree-prefix` global flag (replaced by `--worktree-template`, bound to `local_branch.worktree_template`).

## Template variables

Rule: a bare noun is the raw value; a `*Slug` variable is its directory-safe form (slugified via the `[naming]` booleans; `BranchSlug` is additionally prefix-stripped first).

| Variable | Meaning | Example |
|---|---|---|
| `{{.Number}}` | Issue or PR number | `117` |
| `{{.Branch}}` | Raw branch string, may contain `/` | `feature/add-auth` |
| `{{.PhraseSlug}}` | Slug of the phrase given to `grove create` | `add-user-auth` |
| `{{.TitleSlug}}` | Slug of the issue/PR title | `issue-and-pr-numbers` |
| `{{.BranchSlug}}` | Branch after `strip_prefixes` (first match wins), slugified | `add-auth` |

Availability per template (enforced by giving each template its own data struct):

| | `branch_template` | `worktree_template` |
|---|---|---|
| local_branch | `PhraseSlug` | `BranchSlug` |
| issue | `Number` (required), `TitleSlug` | `Number`, `TitleSlug`, `BranchSlug` |
| pull_request | `Number`, `Branch` | `Number`, `TitleSlug`, `BranchSlug` |

No custom template functions are registered, by design; the supported contract is literal text plus the variables above. (Go `text/template` built-ins such as `printf` inherently exist but are not documented as part of the contract.)

## Semantics

- **Rendering**: template literals pass through verbatim; variables arrive pre-sanitized. The rendered output is not re-slugified — no smart prefix dedup, no hash suffixes.
- **Length cap**: `naming.max_length` applies to the final rendered name — both branches and worktree directories. Truncation is rune-safe, cuts from the end, then trims trailing dashes. Exemption: `pull_request.branch_template` output is never truncated (must match the remote). Docs note: put `{{.Number}}` early in custom templates since truncation eats from the end.
- **Collisions**: no hash disambiguation anymore. Two long phrases identical in their first ~22 chars truncate to the same branch; the second `grove create` fails with the existing "branch already exists" error. Issue/PR names are collision-proof via the number.
- **Validation** (construction-time, with test data, as today):
  - all four template kinds: parse errors and unknown fields rejected;
  - branch templates: rendered result must be a valid branch name (existing `isValidBranchName`);
  - worktree templates: rendered result must be non-empty, contain no `/`, not start with `-`, not be `.`/`..`, no control characters;
  - `issue.branch_template` must still reference `{{.Number}}` (matching is anchored on it); no such requirement for worktree templates;
  - `config.Validate`: templates non-empty, `naming.max_length >= 0`.
- **Matching**: unchanged. Issue matching stays number-anchored on branch names; PR matching stays dual (template-generated branch, raw head branch). Only the `PullRequestTemplateData.BranchName` field renames to `Branch`.
- **Empty results**: an empty slug (e.g. phrase `"***"`) fails generation with the existing guidance error.

## Implementation steps

### 1. `internal/config`

- `config.go`: add `NamingConfig` (fields alpha-ordered: `CollapseDashes`, `Lowercase`, `MaxLength`, `ReplaceNonAlphanum`, `StripPrefixes`, `TrimDashes`); replace `Slugify` with `Naming` in `Config` (keep section fields alpha-ordered). Rewrite `IssueConfig`/`LocalBranchConfig`/`PullRequestConfig` to exactly `BranchTemplate` + `WorktreeTemplate`. Move the CLI-flag struct tag from the old `LocalBranch.WorktreePrefix` to `LocalBranch.WorktreeTemplate` as `config:"worktree-template"` with updated help text. Update `Validate()` per the semantics above.
- `defaults.go`: the target configuration above.

### 2. `internal/naming`

- `slugify.go`: drop `MaxLength`/`HashLength` from `SlugifyOptions` (four booleans remain); delete `computeHash`/`truncateWithHash`; add `TruncateName(name string, max int) string` (rune-safe cut + `TrimRight "-"`).
- Shared helpers: template parse/execute-with-test-data validation; worktree-name validity check; `leadingLiteral(tmpl)` extracting the literal text before the first action (generalized from `numberAnchorPrefix`), exposed as `WorktreeLiteralPrefix()` on each namer for `grove list`.
- `local_branch.go`: namer holds both parsed templates + naming options; constructor becomes fallible (`NewLocalBranchNamer(cfg.LocalBranch, cfg.Naming) (*LocalBranchNamer, error)`). `GenerateBranchName(phrase)` renders `{PhraseSlug}` then caps; `GenerateWorktreeName(branch)` computes `BranchSlug`, renders, caps. `HasPrefix`/`ExtractFromAbsolutePath` reimplemented on the worktree template's leading literal.
- `issue.go`: keep number-anchor matching logic as is. `TitleSlug` loses its own cap (plain slug of title). Add worktree template with data `{Number, TitleSlug, BranchSlug}`; `GenerateWorktreeName` signature gains issue data (`number, title, branch`). Cap both rendered outputs.
- `pull_request.go`: rename `PullRequestTemplateData.BranchName` → `Branch`. Branch template output uncapped; new worktree template with data `{Number, TitleSlug, BranchSlug}`, capped.

### 3. `internal/issue`, `internal/pr`

- Update matcher call sites for the renamed field; matching logic unchanged.

### 4. `cmd`

- `root.go`: `--worktree-prefix` → `--worktree-template` (help: overrides `local_branch.worktree_template`).
- `create.go`, `checkout.go`: handle the now-fallible local namer; adjust the empty-branch guard to the template model; `create` long help updated (example becomes `--worktree-template "subagent-{{.BranchSlug}}"`).
- `issue_start.go`: pass issue data + chosen branch to `GenerateWorktreeName`.
- `pr_checkout.go`: generate the worktree name from PR data (number, title, local branch) instead of branch alone.
- `issue_list.go`, `pr_list.go`: update namer construction (`cfg.Naming`).
- `list.go`: derive display stripping from the local namer's worktree-template literal and the `[PR]` tag from the PR namer's literal; handle namer construction errors.
- `namer.go`, `namer_slug.go`, `namer_branch.go`, `namer_worktree.go`: use `cfg.Naming`; `namer slug` applies the four booleans (no cap — the cap belongs to name generation, not raw slugs; decide in-code and document in command help).
- `docs.go`: rewrite the TOML schema block, validation notes, and the flags section; document the variables table and the truncation/number-early caveat.

### 5. Tests (table-driven where applicable)

Primary owners at the layer closest to the defect:

- `internal/naming`: rendering per flow, truncation (cap boundaries, dash-trim, rune safety, PR-branch exemption), stripping (first-match order), validation errors (bad field, invalid branch/worktree output, missing `{{.Number}}`), literal-prefix extraction, matching regressions (number anchor unchanged, renamed PR field).
- `internal/config`: defaults, `Validate` cases, flat `[naming]` TOML parsing, flag binding for `--worktree-template`.
- `cmd`: existing integration tests updated for new names/flags (create/checkout/issue_start/pr_checkout/list/namer/config/docs goldens) — wiring confidence only, no re-assertion of naming matrices.

Sweep before finishing (clean break): no surviving references to `worktree_prefix`, `strip_branch_prefix`, `title_slug_max_length`, `slugify`, `branch_prefix`, `BranchName`, or `--worktree-prefix` outside the changelog/PR text.

## End-to-end verification (agent-verified)

1. `make check` (vet, lint, tests) and `make build`.
2. Scratch-repo workflow in `.sandbox/`: init a bare-remote + workspace clone, then with `build/grove`:
   - `grove create "add user authentication"` → path ends `wt-add-user-authentication`; branch `feature/add-user-authentication` (cap 30 applied where relevant; verify a long phrase truncates cleanly, and a repeat long phrase errors "branch already exists");
   - `grove checkout feature/add-user-authentication` from the main worktree errors as expected; `grove list --fzf` strips `wt-` in display;
   - custom `grove.toml` overriding templates + `grove config --provenance` shows the new keys and sources;
   - `grove namer slug|branch|worktree` reflect the new pipeline;
   - `grove create "x" --worktree-template "subagent-{{.BranchSlug}}"` → `subagent-…` directory.
3. Real-GitHub workflow against this repository (cleanup after): `build/grove issue start <open issue>` → directory `is-<n>-<title-slug>` (≤ 30 chars); `build/grove pr checkout <open PR>` → `pr-<n>-<title-slug>`, local branch equals the PR head branch. Remove created worktrees/branches afterwards (`git worktree remove`, `git branch -D`).
