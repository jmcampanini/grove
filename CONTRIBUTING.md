# Contributing


## Development

1. Run `mise tasks` to see available tasks.
2. Use `mise run build` to build the binary (outputs to `build/grove`).
3. Run `mise run check` to validate changes.

`mise run check` includes `mise run release-snapshot`, which validates the
GoReleaser configuration and builds all four release archives into `dist/`
without publishing. `mise run clean` removes both `build/` and `dist/`.

The Release workflow validates version tags and default-branch ancestry, runs
the checks, and invokes `mise run release`. Stable releases update
`jmcampanini/homebrew-tap` using Grove's dedicated `HOMEBREW_TAP_DEPLOY_KEY`
secret; prereleases leave the tap unchanged. The first release version has
not been selected.

The binary builds from `main.go` at the repository root. Commands live in
`internal/cli/`, with filenames matching their command paths, such as
`pr_checkout.go`. Shared capabilities that do not depend on Cobra live in
their own packages under `internal/`.
