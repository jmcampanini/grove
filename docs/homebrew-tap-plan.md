# Homebrew Tap Distribution for Grove CLI

## Context

Grove CLI (`grove`) is currently installed manually via `make install` to `~/.local/bin`. There's no automated release process, no Homebrew formula, and no git tags. The goal is to set up end-to-end Homebrew tap distribution so that anyone (including yourself across machines) can install via `brew install jmcampanini/tap/grove`. Additionally, you want a per-profile system for managing taps and Brewfiles so different environments (personal, work) can have different tool sets.

---

## Phase 1: Homebrew Tap Concepts (Reference, No Code Changes)

**How it works:** A Homebrew "tap" is a Git repo named `homebrew-<name>` containing Ruby formula files. Homebrew strips the `homebrew-` prefix, so `jmcampanini/homebrew-tap` becomes `jmcampanini/tap`. Users install via `brew install jmcampanini/tap/grove`. GoReleaser automates everything: it cross-compiles binaries, creates a GitHub Release with artifacts, and pushes an updated formula to your tap repo.

**User flow:**
```
brew tap jmcampanini/tap        # one-time
brew install grove              # install
brew upgrade grove              # future updates
```

Or single-command: `brew install jmcampanini/tap/grove`

**Success criteria:** Conceptual understanding — no deliverables.

---

## Phase 2: GitHub Readiness (Manual Checklist)

These are GitHub settings to configure via the web UI or `gh` CLI. Phase produces no code changes to grove-cli.

### 2a. Create the tap repository

```bash
gh repo create jmcampanini/homebrew-tap --public \
  --description "Homebrew formulae" --clone
cd homebrew-tap
mkdir Formula
echo "# Homebrew Tap" > README.md
git add -A && git commit -m "Initial commit"
git push -u origin main
```

GoReleaser will auto-populate `Formula/grove.rb` on first release.

### 2b. Create a GitHub PAT for GoReleaser

GoReleaser needs to push the formula file to the tap repo during release.

- Go to https://github.com/settings/tokens → **Fine-grained tokens** → Generate
- Token name: `goreleaser-tap`
- Repository access: **Only select repositories** → `homebrew-tap`
- Permissions: **Contents → Read and write**
- Copy the token

Then add it as a secret on grove-cli:
```bash
gh secret set GH_PAT --repo jmcampanini/grove-cli
# paste token when prompted
```

### 2c. grove-cli repo settings checklist

Check each item in **Settings → Branches / Rules / Actions**:

| Setting | Where | What to verify |
|---|---|---|
| Branch protection on `main` | Settings → Branches → Add rule | Pattern: `main`, require PR, require `build` status check to pass |
| Tag protection | Settings → Rules → Rulesets → New | Pattern: `v*`, restrict create/delete to maintainers only |
| Actions permissions | Settings → Actions → General | "Read and write permissions" enabled for `GITHUB_TOKEN` |
| PAT secret exists | Settings → Secrets → Actions | `GH_PAT` is listed |
| Dependabot | `.github/dependabot.yml` | Already configured (verified) |

### 2d. Validate checklist

- [ ] `gh repo view jmcampanini/homebrew-tap` succeeds
- [ ] `gh secret list --repo jmcampanini/grove-cli` shows `GH_PAT`
- [ ] Pushing directly to `main` is blocked (test with a dummy commit if needed)
- [ ] CI workflow `build` job appears in required status checks

**Success criteria:** All checklist items pass.
**Validation:** Run the verification commands above.
**Known gaps:** Tag protection rulesets may require GitHub Pro; skip if on free tier.

---

## Phase 3: GoReleaser + Release Workflow

### 3a. Create `.goreleaser.yml`

**File:** `.goreleaser.yml` (project root)

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - main: .
    binary: grove
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w
      - -X github.com/jmcampanini/grove-cli/cmd.Version={{.Version}}
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: checksums.txt

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"
      - "Merge pull request"

brews:
  - name: grove
    repository:
      owner: jmcampanini
      name: homebrew-tap
      token: "{{ .Env.GH_PAT }}"
    directory: Formula
    homepage: https://github.com/jmcampanini/grove-cli
    description: Git worktree workspace manager
    install: |
      bin.install "grove"
      bash_completion.install Utils.safe_popen_read("#{bin}/grove", "completion", "bash").output => "grove"
      zsh_completion.install Utils.safe_popen_read("#{bin}/grove", "completion", "zsh").output => "_grove"
      fish_completion.install Utils.safe_popen_read("#{bin}/grove", "completion", "fish").output => "grove.fish"
    test: |
      system "#{bin}/grove", "--version"
```

Key decisions:
- darwin + linux, amd64 + arm64 (4 binaries)
- Shell completions generated at install time by the formula (simpler than bundling)
- `CGO_ENABLED=0` for static binaries
- Version injected via same ldflags path the Makefile uses

### 3b. Create `.github/workflows/release.yml`

**File:** `.github/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GH_PAT: ${{ secrets.GH_PAT }}
```

### 3c. Update Makefile version strategy

**File:** `Makefile` — change VERSION line:

```makefile
# Before:
VERSION := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
# After:
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')
```

Falls back to timestamp if no tags exist (backwards compatible).

### 3d. First release process

```bash
git tag v0.1.0 --no-sign
git push origin v0.1.0
# Watch: gh run watch (or Actions tab)
# Verify: gh release view v0.1.0
# Verify formula: gh api repos/jmcampanini/homebrew-tap/contents/Formula/grove.rb
```

**Success criteria:** `gh release view v0.1.0` shows 4 archives + checksums; `Formula/grove.rb` exists in tap repo.
**Validation:** `brew install jmcampanini/tap/grove && grove --version` outputs `v0.1.0`.
**Test coverage:** Existing CI tests run in `before.hooks`; formula has a `test` block.
**Known gaps:** First release may need formula tweaks for completion generation — `Utils.safe_popen_read` may need adjustment based on GoReleaser's actual output. We'll fix forward.
**Manual verification:**
1. `brew install jmcampanini/tap/grove` completes without errors
2. `grove --version` prints `0.1.0`
3. `grove <TAB>` produces completions in your shell

---

## Phase 4: Per-Profile Tap + Brewfile System

This is outside the grove-cli repo — it's dotfiles/shell config.

### 4a. Directory structure

```
~/.config/brew/
├── profiles/
│   ├── personal.taps     # one tap per line, processed first
│   ├── personal.Brewfile  # standard Brewfile
│   ├── work.taps
│   └── work.Brewfile
└── brew-profile           # shell function to run a profile
```

### 4b. Tap file format

**File:** `~/.config/brew/profiles/personal.taps`
```
jmcampanini/tap
```

**File:** `~/.config/brew/profiles/personal.Brewfile`
```ruby
# Formulae from personal tap
brew "grove"

# Other personal tools
brew "fzf"
brew "zoxide"
brew "gh"
```

**File:** `~/.config/brew/profiles/work.taps`
```
jmcampanini/tap
some-org/tap
```

**File:** `~/.config/brew/profiles/work.Brewfile`
```ruby
brew "grove"
brew "some-org/tap/internal-tool"
```

### 4c. Shell function

**File:** `~/.config/brew/brew-profile`
```bash
brew-profile() {
  local profile="${1:?Usage: brew-profile <name>}"
  local dir="$HOME/.config/brew/profiles"
  local tapfile="$dir/$profile.taps"
  local brewfile="$dir/$profile.Brewfile"

  if [[ ! -f "$tapfile" ]]; then
    echo "No tap file: $tapfile" >&2
    return 1
  fi
  if [[ ! -f "$brewfile" ]]; then
    echo "No Brewfile: $brewfile" >&2
    return 1
  fi

  # Stage 1: tap everything
  while IFS= read -r tap || [[ -n "$tap" ]]; do
    [[ -z "$tap" || "$tap" == \#* ]] && continue
    brew tap "$tap" 2>/dev/null || echo "Already tapped: $tap"
  done < "$tapfile"

  # Stage 2: bundle install
  brew bundle --file="$brewfile"
}
```

Source it from your shell rc:
```bash
# In ~/.zshrc or ~/.bashrc
[[ -f ~/.config/brew/brew-profile ]] && source ~/.config/brew/brew-profile
```

Usage:
```bash
brew-profile personal   # taps first, then installs Brewfile
brew-profile work
```

### 4d. Fish variant (if needed)

```fish
# ~/.config/fish/functions/brew-profile.fish
function brew-profile
    set -l profile $argv[1]
    set -l dir "$HOME/.config/brew/profiles"
    set -l tapfile "$dir/$profile.taps"
    set -l brewfile "$dir/$profile.Brewfile"

    for tap in (string match -rv '^\s*(#|$)' < "$tapfile")
        brew tap "$tap" 2>/dev/null; or echo "Already tapped: $tap"
    end

    brew bundle --file="$brewfile"
end
```

**Success criteria:** `brew-profile personal` taps `jmcampanini/tap` and installs grove without errors.
**Validation:** Run `brew-profile personal`, then `grove --version`.
**Known gaps:** `brew bundle` must be installed (`brew tap homebrew/bundle` — usually pre-installed on modern Homebrew).
**Manual verification:**
1. Create the files above
2. Run `brew-profile personal`
3. Confirm `brew tap` lists `jmcampanini/tap`
4. Confirm `grove --version` works

---

## Summary of Files to Create/Modify in grove-cli

| File | Action |
|---|---|
| `.goreleaser.yml` | Create |
| `.github/workflows/release.yml` | Create |
| `Makefile` (line 7, VERSION) | Edit one line |

Everything else (tap repo, PAT, branch protection, profile scripts) is GitHub config or dotfiles outside this repo.
