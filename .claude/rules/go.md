---
paths:
  - "**/*.go"
---

# Go conventions for Munus

These are the repository's own conventions; general Go practice applies where they are silent.

## Every file

- Start with the Apache-2.0 header block exactly as in `main.go` (test files too), then a blank
  line, then `package`.
- `gofmt -s`. There is no `.golangci.yml`, so golangci-lint v2 runs its defaults; do not add a
  config without asking. A suppression (`//nolint`, `// #nosec Gxxx`) is a last resort, needs
  the maintainer's agreement and carries its reason on the same line, as the existing
  `#nosec` comments do.

## Design

- All application code is package `pkg`. Shared types (models, DTOs, options) live in
  `pkg/database.go`. Exported names are public API for other modules: keep them stable, document
  new ones with doc comments that start with the name, and export nothing that is not needed.
- No package-level mutable state. Inject what varies: the clock (`now time.Time` parameters, as in
  `parseDeadlineAt`, `deadlineLabel`, `listDeadline`), the environment (`getenv` functions, as in
  `resolveDatabaseLocation`), writers (GORM's log writer via `openDatabase`).
- Optional capabilities are discovered with small unexported interfaces (for example
  `databaseFiler`), not by type-asserting to `*Database`. TUI models and `TaskServiceAdapter`
  depend on the `Storage` interface so tests can use fakes.
- Prefer the standard library and existing dependencies. A new module needs the maintainer's
  approval, a license review, `make licenses` and a `NOTICE` entry. `tools/licenses` stays
  stdlib-only.
- No new goroutines, channels or mutexes unless the task needs them. TUI background work uses
  Bubble Tea commands (see the CLI/TUI rule).

## Errors and context

- Return errors with context; wrap with `%w` when callers inspect them; compare with
  `errors.Is`/`errors.As` (for example `ErrTaskNotFound`), never by string.
- Discard an error only deliberately (`_ =` / `_, _ =`) and say why in a comment when it is not
  obvious. No `panic` for user input, I/O or database failures.
- Functions that do storage, import, export or backup work take `ctx context.Context` first and
  use GORM's `WithContext`. CLI commands pass `cmd.Context()`; the TUI creates a context per
  operation at its event boundary. Never store a context in a struct (the TUI keeps only a
  transfer's `CancelFunc`).
- Bound everything read from outside (files, stdin, env, flags) before using it.

## Tests

- Tests sit next to the code in package `pkg`, are deterministic (no sleeps, no machine time zone,
  no real home directory, no network) and use `t.TempDir()`, `t.Helper()`, `t.Cleanup()`.
- Anything that can write a backup calls `setTestHome(t)` first.
- Table-driven tests where inputs share a pattern; regression tests name the behaviour they pin.
- A new parser or decoder of untrusted input gets a fuzz target, added to `FUZZ_TARGETS` in the
  `Makefile`.
- Run `make race` for anything touching transactions, concurrent opens or TUI transfers.
