# Architecture & Maintainability Guidance (authoritative)

> Authoritative per `AGENTS.md`. `CONTRIBUTING.md` must stay consistent with this file.

## Overview
Munus is a single-binary Go CLI/TUI task manager backed by a local SQLite file.

- Language: Go (`go.mod` declares `go 1.26.0`).
- CLI framework: `spf13/cobra`.
- TUI: `charmbracelet/bubbletea` + `charmbracelet/lipgloss`.
- Persistence: `gorm.io/gorm` with `gorm.io/driver/sqlite`, which wraps
  `mattn/go-sqlite3` — **requires CGO** (a C toolchain at build time).

## Layering
| Layer | Location | Responsibility |
|---|---|---|
| Entry point | `main.go`, `database_path.go` | Resolve DB path (`MUNUS_DB_PATH`, default `munus.db` in CWD), create a *deferred* DB, set `main.version`, run root command, close DB |
| CLI | `internal/logic-cli.go` | Cobra commands: root (TUI), `add`, `list`, `complete`, `delete`, `export`, `import`; root `PersistentPreRunE` opens the DB (skipped for help/completion) |
| TUI | `internal/ui-form.go`, `internal/ui-list.go` | Bubble Tea models for task entry form and list/dashboard (incl. import/export overlay, optional Vim mode) |
| Domain helpers | `internal/logic-tui.go`, `internal/ext-deadline.go` | Overdue/upcoming logic, deadline parsing (bounded) |
| Text safety | `internal/ext-text.go` | Shared length limits, control-character/UTF-8 validation, terminal sanitising |
| Transfer | `internal/ext-export-import.go` | Versioned JSON (v1) export, plan/apply import (merge/replace), backups |
| Storage | `internal/database.go` | `Storage` interface, `Database` (gorm) implementation, all shared types |

## Conventions
- All shared types (models, DTOs, options) live in `internal/database.go`.
- UI and CLI code depend on the `Storage` interface; tests use it for fakes. Optional
  capabilities are discovered with small unexported interfaces (e.g. `databaseFiler`)
  rather than type-asserting to `*Database`.
- Commands write via `cmd.OutOrStdout()` / read via `cmd.InOrStdin()` so they are testable.
  Yes/no prompts go through `Confirm` (whole line, `y`/`yes` any case, EOF = no).
- Every source file carries the Apache-2.0 license header (see `CONTRIBUTING.md`).
- Formatting: `gofmt -s -w .` before PRs. CI also runs `go vet` and `golangci-lint` v2.13.2.
- Tests live next to code (`*_test.go`); CI runs `go test -v -coverprofile=coverage.out ./...`.
  Tests that write backups must call `setTestHome(t)`; setup errors must `t.Fatalf`, never `return`.

## Data rules
- **Task IDs are stable.** Imports never renumber existing tasks. `TaskServiceAdapter.ReplaceAll`
  keeps numeric IDs in `1..2^31-1`; other IDs (empty, `tsk_…` placeholders, oversized) get a new
  database ID. `Database.ReplaceAllTasks` inserts explicit-ID rows before auto-ID rows in one
  transaction (the table uses `AUTOINCREMENT`).
- **Export format v1** carries an optional `completed_at`; importers fall back to `updated_at`
  for completed tasks without it.
- **Text policy:** `add`, the TUI form and `import --strict` reject control characters, invalid
  UTF-8 and over-length text. Default import strips control characters (CRLF → LF) so older
  exports/backups always restore. All task text written to the terminal goes through
  `sanitizeForTerminal`.
- **Files:** new DB, export and backup files are `0600`, backup dir `0700`; exports use a random
  temp file + rename and refuse to overwrite the active database (compared via the file sqlite
  reports in `pragma_database_list`).
- GORM's logger is silent (`gormLogWriter`); errors are returned, never printed to stdout.

## Build constraints
- Builds that include the SQLite driver must use `CGO_ENABLED=1`; a `CGO_ENABLED=0`
  build compiles but cannot open a database.
- Releases (`cd.yml`) build natively per OS with CGO; Linux binaries are linked statically
  (`-tags sqlite_omit_load_extension,osusergo,netgo`, `-extldflags -static`). darwin/amd64 is
  cross-compiled by clang on the Apple Silicon runner. windows/arm64 is not built.
- The Dockerfile builds with CGO on Alpine for the image's own platform and runs as UID 10001
  with `HOME=/app/data`.

## Maintainability guidance
- Keep changes scoped; follow the nearest existing pattern (see `AGENTS.md`).
- Any change to import must keep `PlanImport` and `ApplyImport` on the same `merge` logic and
  preserve the ID rules above.
- Merge import snapshots current tasks outside the replace transaction; concurrent writers during
  an import can lose updates (pre-existing, single-user tool).
