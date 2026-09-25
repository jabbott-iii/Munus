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
| CLI | `internal/logic-cli.go` | Cobra commands: root (TUI), `add`, `edit`, `list` (filters), `complete`, `delete`, `export`, `import`; root `PersistentPreRunE` opens the DB (skipped for help/completion) |
| TUI | `internal/ui-form.go`, `internal/ui-list.go` | Bubble Tea models for the task form (create and edit) and list/dashboard (status cycling, filters, help panel, import/export overlay, optional Vim mode) |
| Domain helpers | `internal/logic-tui.go`, `internal/ext-deadline.go` | Status transitions, overdue/upcoming logic, calendar-day deadline labels (clock passed in), task filters, deadline parsing (bounded) |
| Text safety | `internal/ext-text.go` | Shared length limits, control-character/UTF-8 validation, tag rules, terminal sanitising |
| Transfer | `internal/ext-export-import.go` | Versioned JSON export (v2; imports v1 and v2), plan/apply import (merge/replace), stdin input, backups |
| Storage | `internal/database.go` | `Storage` interface, `Database` (gorm) implementation incl. tags, migration and consistency triggers, all shared types |

## Conventions
- All shared types (models, DTOs, options) live in `internal/database.go`.
- UI and CLI code depend on the `Storage` interface; tests use it for fakes. Optional
  capabilities are discovered with small unexported interfaces (e.g. `databaseFiler`)
  rather than type-asserting to `*Database`.
- Follow `intel/golang.md`. Storage methods and import/export/backup functions take
  `ctx context.Context` first and use GORM `WithContext`; CLI commands pass `cmd.Context()` (`main`
  runs `ExecuteContext(context.Background())`); the TUI passes `context.Background()` per operation at
  its event boundary. Contexts are never stored in structs. No package-level mutable state (GORM's
  log writer is injected via `openDatabase`); discarded errors carry a comment saying why.
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
- **Status:** `status` is `todo`, `doing` or `done`; `completed` (kept for compatibility) is true
  exactly when the status is `done`, and `completed_at` is set exactly when it is true.
  `ItemModel.BeforeSave` enforces this on every write (when they disagree, `completed` decides done
  versus not done); use `setStatus` to change both.
- **Tags** live in `tags` (unique lowercase names) and `task_tags` (links); names are 1–32 letters,
  digits, `-` or `_`, at most 10 per task (`normalizeTags`). Writes replace a task's links and prune
  unused tags; `loadTags` returns them sorted.
- **Older binaries:** v2.1.1 knows nothing about `status` or tags. Migration backfills `status` from
  `completed`, removes orphaned tag links and installs `consistencyTriggers` (status follows
  `completed` on insert/update; deleting a task removes its links), so a migrated database stays
  consistent if v2.1.1 writes to it. Migrations write only when something needs changing, so an
  up-to-date database can be opened read-only. Schema changes must stay additive (new columns need
  defaults) and must not loosen file permissions.
- **Missing tasks:** `UpdateTask` and `DeleteTask` return `ErrTaskNotFound` for a task that no
  longer exists (never re-create it); the TUI reloads the list on that error.
- **Export format v2** adds `status` and `tags`; v1 files (no status/tags) still import, with the
  status derived from `completed`, and merging a v1 file keeps existing tasks' tags and `doing`
  status. v2.1.1 and older reject v2 files. Exports carry an optional `completed_at`; importers fall
  back to `updated_at` for completed tasks without it.
- **Text policy:** `add`, the TUI form and `import --strict` reject control characters, invalid
  UTF-8 and over-length text. Default import strips control characters (CRLF → LF) so older
  exports/backups always restore. All task text written to the terminal goes through
  `sanitizeForTerminal`.
- **Files:** new DB, export and backup files are `0600`, backup dir `0700`; exports use a random
  temp file + rename and refuse to overwrite the active database (compared via the file sqlite
  reports in `pragma_database_list`).
- GORM's logger is silent (writer injected via `openDatabase`, `io.Discard` in production); errors
  are returned, never printed to stdout.

## Build constraints
- Builds that include the SQLite driver must use `CGO_ENABLED=1`; a `CGO_ENABLED=0`
  build compiles but cannot open a database.
- Releases (`cd.yml`) build natively per OS with CGO; Linux binaries are linked statically
  (`-tags sqlite_omit_load_extension,osusergo,netgo`, `-extldflags -static`). darwin/amd64 is
  cross-compiled by clang on the Apple Silicon runner. windows/arm64 is built natively on the
  `windows-11-arm` runner with llvm-mingw (`CC=aarch64-w64-mingw32-clang`), downloaded from the
  pinned GitHub release and verified against its SHA-256 before use; bump both values together.
- The Dockerfile builds with CGO on Alpine for the image's own platform and runs as UID 10001
  with `HOME=/app/data`. Builder (`golang:1.26-alpine3.24`) and runtime (`alpine:3.24`) use the same
  Alpine release (same musl), and `apk` packages are pinned (`build-base`, `ca-certificates`).
  To bump: move both images to the same new Alpine release, look up the current package versions
  for that release (for example `apk policy build-base ca-certificates` in the new image, or the
  aports `x.y-stable` branch), update the pins, and confirm `hadolint Dockerfile` and `docker.yml`
  pass. A pin fails the build once Alpine drops that package revision, so bump when that happens.

## Maintainability guidance
- Keep changes scoped; follow the nearest existing pattern (see `AGENTS.md`).
- Any change to import must keep `PlanImport` and `ApplyImport` on the same merge logic
  (`mergeVersion`) and preserve the ID rules above.
- Import reads the current tasks, writes the backup and replaces the tasks inside one storage
  transaction (`ReplaceAllTasksFunc`), so concurrent writes are never lost; the preview can still
  differ from the result if another process writes between plan and apply.
- The Makefile targets (`make check`) mirror CI locally; keep them in step with `ci.yml`.
