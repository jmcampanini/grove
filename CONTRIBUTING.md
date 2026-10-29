# Contributing


## Development

1. Run `mise tasks` to see available tasks.
2. Use `mise run build` to build the binary (outputs to `build/grove`).
3. Run `mise run check` to validate changes.

The binary builds from `main.go` at the repository root. Commands live in `internal/cli/`, with filenames matching their command paths, such as `pr_checkout.go`. Shared capabilities that do not depend on Cobra live in their own packages under `internal/`.

To upgrade a tool, run `mise upgrade --bump <tool>`, then `mise lock` and `mise run check`.
