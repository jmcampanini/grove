# Plan: Port Grove config file loading to go-config-loader

## Scope

This plan is intentionally limited to:

1. TOML file loading.
2. Strict unknown-key feedback.
3. Provenance/reporting for loaded config values.

This plan explicitly does **not** include environment variables or flags yet.

## Goals

- Use `github.com/jmcampanini/go-config-loader` as Grove's config loading boundary.
- Keep Grove-specific config file discovery in Grove.
- Remove Grove's custom TOML merge/load implementation.
- Make unknown config keys immediate errors.
- Preserve Grove's app-level validation via `Config.Validate()`.
- Feed config provenance into the `grove config` command.

## Non-goals

- Do not add `GROVE_*` environment variable support yet.
- Do not add config-backed CLI flags yet.
- Do not replace Grove's worktree-aware config path discovery.
- Do not maintain compatibility wrappers solely to avoid touching call sites.

## Architecture after this change

### Grove remains responsible for discovery

Keep:

- `internal/config/discovery.go`
- `ConfigPaths(cwd, worktreeRoot, gitRoot, homeDir)`
- `BootstrapConfigPaths(cwd, homeDir)`

These functions encode Grove-specific policy:

```text
~/.config/grove/grove.toml
ancestor grove.toml files
main worktree grove.toml
current worktree grove.toml
cwd grove.toml
```

The paths are ordered from lowest to highest priority.

### go-config-loader becomes responsible for loading

Use `go-config-loader` directly for:

- parsing TOML files
- merging all existing files in order
- strict unknown-key handling
- producing `LoadReport`
- reporting `LoadedFiles`
- reporting field-level provenance in `Updates`

Conceptual loading flow:

```go
fileLoader, err := configloader.NewMergeAllFilesLoader[config.Config](paths)
if err != nil {
    return err
}

cfg, report, err := configloader.Load(config.DefaultConfig(), fileLoader)
if err != nil {
    return err
}

if err := cfg.Validate(); err != nil {
    return fmt.Errorf("invalid config: %w", err)
}
```

Do not pass `WarnUnknownKeys()` or `IgnoreUnknownKeys()`. The default strict behavior is desired.

## Why use the loader directly?

The earlier wrapper shape:

```go
type LoadResult struct {
    Config      Config
    SourcePaths []string
    Report      configloader.LoadReport
}
```

was mainly a compatibility bridge for Grove's current `internal/config.Loader` API.

Since we are willing to change call sites, prefer using the library's native API directly:

```go
cfg, report, err := configloader.Load(...)
```

Benefits:

- Fewer Grove-specific abstractions.
- Less duplicated naming: `SourcePaths` is already `report.LoadedFiles`.
- Provenance stays in the library's canonical format.
- Future env/flag adoption will compose naturally by adding loaders.
- Call sites make the actual loading pipeline explicit.

Grove should still have small helper functions where they represent Grove-specific orchestration, but not a duplicate generic loader type.

## Proposed implementation

### 1. Add dependency

Add:

```text
github.com/jmcampanini/go-config-loader
```

Also add:

```text
github.com/jmcampanini/go-config-loader/configreporter
```

where provenance/TOML reporting is needed by the config command.

### 2. Remove or shrink `internal/config/loader.go`

The existing `internal/config/loader.go` currently owns:

- `LoadResult`
- `FileSystem`
- `OSFileSystem`
- `Loader`
- `NewLoader`
- `NewDefaultLoader`
- `Loader.Load`

After migration, this custom generic loader layer should go away unless a small Grove-specific helper is still useful.

Preferred direction:

- Delete `Loader`, `LoadResult`, and `FileSystem` abstractions.
- Keep config data types, defaults, validation, and discovery.
- Let command/runtime code call `go-config-loader` directly.

Because the config-loader library has been updated to skip directories, Grove does not need to pre-filter paths just to avoid directory reads.

### 3. Add a small Grove-specific load helper only if it improves readability

If direct calls become repetitive, add a helper with Grove-specific semantics, not a replacement generic loader.

Example:

```go
func LoadFiles(paths []string) (Config, configloader.LoadReport, error) {
    fileLoader, err := configloader.NewMergeAllFilesLoader[Config](paths)
    if err != nil {
        return Config{}, configloader.LoadReport{}, err
    }

    cfg, report, err := configloader.Load(DefaultConfig(), fileLoader)
    if err != nil {
        return Config{}, configloader.LoadReport{}, err
    }

    if err := cfg.Validate(); err != nil {
        return Config{}, configloader.LoadReport{}, fmt.Errorf("invalid config: %w", err)
    }

    return cfg, report, nil
}
```

This helper is acceptable because it captures Grove-specific post-load validation. It should return the library's native `LoadReport`, not wrap it in a parallel result type.

### 4. Update runtime config loading

`cmd/runtime_context.go` currently calls:

```go
bootstrapResult, err := config.NewDefaultLoader().Load(bootstrapPaths)
...
loadResult, err := config.NewDefaultLoader().Load(configPaths)
```

Update this flow to receive both config and report:

```go
bootstrapCfg, bootstrapReport, err := config.LoadFiles(bootstrapPaths)
...
cfg, report, err := config.LoadFiles(configPaths)
```

or call `configloader` directly if no helper is added.

Update `commandRuntime` to retain the report:

```go
type commandRuntime struct {
    cfg          config.Config
    configReport configloader.LoadReport
    cwd          string
    gitClient    git.Git
    workspaceDir string
}
```

Use the report for logging and for `grove config` provenance.

### 5. Update other config loading paths

The following command paths load config independently and should be updated:

- `cmd/runtime_context.go`
- `cmd/logs.go`
- `cmd/namer.go`
- `cmd/resolve.go`

Each should either:

- use the shared `config.LoadFiles(paths)` helper, or
- call `go-config-loader` directly and then run `cfg.Validate()`.

Avoid recreating the old `Loader` abstraction.

### 6. Update `grove config` for provenance

`cmd/config.go` currently re-encodes `rt.cfg` with BurntSushi TOML.

After migration, the runtime should carry both:

- final config
- `configloader.LoadReport`

Then `grove config` can use `configreporter`:

```go
reporter := configreporter.New(rt.cfg, rt.configReport)
```

For TOML output:

```go
return reporter.WriteTOML(cmd.OutOrStdout())
```

For provenance output, add a command option such as:

```text
grove config --provenance
```

or:

```text
grove config --sources
```

The provenance output should show rows from:

```go
reporter.ProvenanceHeaders()
reporter.ProvenanceRows()
```

Initial provenance will include only:

- `<default>`
- loaded TOML file paths

No env or flag sources should appear yet because this plan does not add env/flag loaders.

### 7. Update tests

Expected test changes:

- Tests for `ConfigPaths` and `BootstrapConfigPaths` should remain.
- Tests for Grove's custom `FileSystem`, `OSFileSystem`, and `Loader` should be removed or rewritten.
- File loading tests should assert behavior through the new direct library path or `config.LoadFiles` helper.
- Unknown key tests should now expect an error, not a warning.
- Tests should assert `report.LoadedFiles` instead of `LoadResult.SourcePaths`.
- Add provenance tests for `grove config --provenance` / `--sources`.

Keep coverage for:

- missing files are ignored
- directory paths are ignored by the library
- single file loads
- multiple files merge low to high priority
- later zero values override earlier non-zero values
- invalid TOML returns an error
- unknown keys return an error
- invalid Grove config values return an error from `Config.Validate()`
- provenance identifies default values vs file-provided values

### 8. Update imports

Remove direct BurntSushi TOML usage from Grove where it is only used for config loading/reporting.

Potential remaining TOML import should be unnecessary in `cmd/config.go` if `configreporter` handles TOML output.

## Expected final precedence

For this plan only:

```text
defaults
< discovered TOML files, merged low → high priority
```

No environment variable layer.
No flag layer.

## Expected final provenance sources

For this plan only:

```text
<default>
/path/to/loaded/grove.toml
/path/to/higher-priority/grove.toml
```

## Validation model

The library validates config shape and tags.

Grove still validates app semantics after loading:

```go
if err := cfg.Validate(); err != nil {
    return fmt.Errorf("invalid config: %w", err)
}
```

Examples of Grove-owned validation:

- `git.timeout` cannot be negative
- `github.preview_cache_ttl` cannot be negative
- `pull_request.worktree_prefix` cannot be empty
- `slugify.hash_length` must be compatible with `slugify.max_length`
- `workspace.primary_branches` cannot be empty

## Rollout steps

1. Add `go-config-loader` dependency.
2. Replace custom loader usage in config-loading call sites.
3. Preserve Grove discovery functions unchanged.
4. Run app-level validation after library loading.
5. Update `commandRuntime` to retain `LoadReport`.
6. Update `grove config` to use `configreporter`.
7. Add provenance output option to `grove config`.
8. Update tests for strict unknown keys and provenance.
9. Run `make check`.

## Summary

The migration should make Grove's config architecture simpler:

```text
Grove:
  discover paths
  validate meaning
  use final config

 go-config-loader:
  load files
  merge layers
  reject unknown keys
  report provenance
```

This keeps Grove's worktree-aware behavior while moving generic config loading and provenance into the shared library.
