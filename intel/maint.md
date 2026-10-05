# Architecture & Maintainability Guidance (authoritative)

> Authoritative per `AGENTS.md`. `CONTRIBUTING.md` must stay consistent with this file.
> Decision, plan-item and note IDs up to D-22, P-060 and N-041 refer to `intel/plan.md` and
> `intel/notes.md` as of commit `442e63c` (both were cleared on 2026-10-03, see `history.md`);
> later IDs (P-061, N-042 onward) refer to the current files.

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
| Entry point | `main.go` | Resolve the DB location (`pkg.LocateDatabase`), create a *deferred* DB, set `main.version`, run root command, close DB |
| DB location | `pkg/database_path.go` | `LocateDatabase`: `MUNUS_DB_PATH`, else `munus.db` in the per-user data directory, plus a notice when the working directory holds an old `./munus.db` |
| CLI | `pkg/logic-cli.go` | Cobra commands: root (TUI), `add`, `edit`, `list` (filters), `complete`, `delete`, `export`, `import`; each command validates its arguments and flags, then calls `openForCommand` to open the DB |
| TUI | `pkg/ui-form.go`, `pkg/ui-list.go` | Bubble Tea models for the task form (create and edit) and list/dashboard (status cycling, filters, help panel, import/export overlay, optional Vim mode) |
| Domain helpers | `pkg/logic-tui.go`, `pkg/ext-deadline.go` | Status transitions, overdue/upcoming logic, calendar-day deadline labels (clock passed in), task filters, deadline parsing (bounded) |
| Text safety | `pkg/ext-text.go` | Title/description length checks (limits declared in `ui-form.go`), control-character/UTF-8 validation, tag rules, terminal sanitising |
| Transfer | `pkg/ext-export-import.go` | Versioned JSON export (v2; imports v1 and v2), plan/apply import (merge/replace), stdin input, backups |
| Storage | `pkg/database.go` | `Storage` interface, `Database` (gorm) implementation incl. tags, migration and consistency triggers, all shared types |
| Release tooling | `tools/licenses/` | Generates and checks `THIRD_PARTY_LICENSES` and the `NOTICE` module list (not part of the binary) |

## Conventions
- All application code is package `pkg` in `pkg/` (importable by other modules, unlike `internal/`);
  `main.go` only wires it up. All shared types (models, DTOs, options) live in `pkg/database.go`.
- The TUI models and the import/export service (`TaskServiceAdapter`) depend on the `Storage`
  interface; tests use it for fakes. CLI commands take the deferred `*Database`, open it with
  `openForCommand` and wrap it in `TaskServiceAdapter` for import and export. Optional
  capabilities are discovered with small unexported interfaces (e.g. `databaseFiler`)
  rather than type-asserting to `*Database`.
- Follow `intel/golang.md`. Storage methods and import/export/backup functions take
  `ctx context.Context` first and use GORM `WithContext`; CLI commands pass `cmd.Context()` (`main`
  runs `ExecuteContext(context.Background())`); the TUI passes `context.Background()` per operation at
  its event boundary, except that each import/export step gets its own cancellable context (only its
  `CancelFunc` is kept, so esc can cancel it). Contexts are never stored in structs. No package-level mutable state (GORM's
  log writer is injected via `openDatabase`); discarded errors carry a comment saying why.
- Commands write via `cmd.OutOrStdout()` / read via `cmd.InOrStdin()` so they are testable.
- Each command checks its arguments and flag values first (cobra `Args` validators, required and
  grouped flags, then value checks at the top of `RunE`) and only then calls `openForCommand`,
  which turns off cobra's usage output and opens the database. Argument and flag errors therefore
  print usage and never create or migrate the database; later errors print only the error line.
  Yes/no prompts go through `Confirm` (whole line, `y`/`yes` any case, EOF = no).
- `openForCommand` writes the deferred database's notice (`DatabaseLocation.Notice`) once to
  `cmd.ErrOrStderr()` before opening, so standard output stays clean (`export --stdout`); help,
  version and argument errors never open the database and so never show it. The root command gets
  the notice from `openForCommandWithNotice` and passes it to the TUI (`tuiOptions.notice`), which
  shows it, sanitised, at the top of every screen because the alternate screen hides earlier output.
- Ctrl+C in CLI commands keeps Go's default behaviour (decided as final, D-17 a): the process exits
  at once and SQLite rolls back an unfinished transaction. Do not add signal handling unless that
  decision is revisited; keep every multi-step write inside one storage transaction so an interrupt
  never leaves partial data. (In the TUI, `ctrl+c` is a key that quits or cancels.)
- Every source file carries the Apache-2.0 license header (see `CONTRIBUTING.md`).
- Formatting: `gofmt -s -w .` before PRs. CI also runs `go vet` and `golangci-lint` v2.13.2.
- Tests live next to code (`*_test.go`); CI runs `go test -v -coverprofile=coverage.out ./...`.
  Tests that write backups must call `setTestHome(t)`; setup errors must `t.Fatalf`, never `return`.

## Data rules
- **Task IDs are stable.** Imports never renumber existing tasks: `storedTaskID` keeps an ID that is
  in `1..maxImportedTaskID` (1,000,000,000) or that is the canonical spelling of a task already in the
  database; other IDs (empty, `tsk_…` placeholders, oversized IDs from a file) get a new database ID.
  The cap stays well below 2^31-1 so a crafted file cannot push later IDs past it. Numeric IDs from
  a file are canonicalised (`"01"` → `"1"`) before merging. `Database.ReplaceAllTasks` inserts
  explicit-ID rows before auto-ID rows in one transaction (the table uses `AUTOINCREMENT`).
- **Status:** `status` is `todo`, `doing` or `done`; `completed` (kept for compatibility) is true
  exactly when the status is `done`, and `completed_at` is set exactly when it is true.
  `ItemModel.BeforeSave` enforces this on every write (when they disagree, `completed` decides done
  versus not done); use `setStatus` to change both. A status-only change (TUI `c`/`s`, `complete`)
  goes through `Storage.UpdateTaskStatus(ctx, id, next, now)` (`Database.SetTaskStatus` wraps it for
  a fixed status): it reads the stored status and writes `next(stored)` in one transaction (plan 4,
  P-052), so the decision never rests on a stale copy (`nextStatus` for `s`; `completedStatus` for
  `complete` and for `c` on a task shown as open; `reopenedStatus` for `complete --undo` and for `c`
  on a task shown as done), and it writes only `status`, `completed`, `completed_at`
  and `updated_at`, so it never writes back a stale copy of the other fields. The callback runs
  inside the transaction and must not block. `itemStatus` and
  `taskStatusOf` only return `todo`, `doing` or `done`; opening a database repairs any other stored
  status (`completed` decides; a `doing` in another case is kept).
- **Tags** live in `tags` (unique lowercase names) and `task_tags` (links); names are 1–32 letters,
  digits, `-` or `_`, at most 10 per task (`normalizeTags`). Writes replace a task's links and prune
  unused tags; `loadTags` returns them sorted.
- **Transactions and migration:** the DSN gets `_txlock=immediate` (unless it sets `_txlock`), so
  every transaction starts with `BEGIN IMMEDIATE`: a transaction holds the write lock from its
  start and concurrent writers wait (busy timeout: 15 s, added by `withBusyTimeout` unless the DSN
  sets `_busy_timeout` or `_timeout`; plan 4, D-14) instead of failing with
  "database is locked" when a read-then-write transaction cannot upgrade its lock. Schema migration
  first runs without a transaction (so an up-to-date database is only read); if that fails, for
  example because another process created a table in between, it runs again inside a transaction,
  i.e. under the write lock. Every migration step must stay idempotent; triggers use
  `CREATE TRIGGER IF NOT EXISTS`.
- **Older binaries:** v2.1.1 knows nothing about `status` or tags. Migration backfills `status` from
  `completed`, removes orphaned tag links and installs `consistencyTriggers` (status follows
  `completed` on insert/update; deleting a task removes its links), so a migrated database stays
  consistent if v2.1.1 writes to it. Migrations write only when something needs changing, so an
  up-to-date database can be opened read-only. Schema changes must stay additive (new columns need
  defaults) and must not loosen file permissions.
- **Deadlines:** relative deadlines count from an injected clock (`parseDeadlineAt`,
  `relativeDeadline`; `ParseDeadline` passes `time.Now()`): `d`, `w` and `M` are calendar units
  added with `AddDate` (same clock time across DST), `h` and `m` are elapsed time added after them.
  Absolute deadlines are read in the clock's location. `munus list` shows a deadline in the
  clock's location in `deadlineInputLayout` (`listDeadline`), followed for unfinished tasks by the
  TUI's relative label when it is overdue or at most three calendar days away
  (`relativeDeadlineLabel`, which `deadlineLabel` uses too), and `none` without a deadline.
- **Missing tasks:** `UpdateTask` and `DeleteTask` return `ErrTaskNotFound` for a task that no
  longer exists (never re-create it); the TUI reloads the list on that error.
- **Export format v2** adds `status` and `tags`; v1 files (no status/tags) still import, with the
  status derived from `completed`, and merging a v1 file keeps existing tasks' tags and `doing`
  status. v2.1.1 and older reject v2 files. Exports carry an optional `completed_at`; importers fall
  back to `updated_at` for completed tasks without it.
- **Text policy:** `add`, `edit`, the TUI form and `import --strict` reject control characters
  (C0/C1 and the bidi embedding/override/isolate controls U+202A–U+202E, U+2066–U+2069), invalid
  UTF-8, over-length text and blank titles (`add`, `edit` and the form also blank descriptions). Default
  import strips control characters (CRLF → LF), shortens text over the byte limits at a rune
  boundary (`shortenToLimit`; plan 4, D-13; counted as `Shortened` in the plan and result) and then
  stores a title that is blank as `(untitled)`, so older exports/backups always restore. All task text written to the terminal goes
  through `sanitizeForTerminal`; the root command writes its errors through `terminalSafeWriter`,
  and TUI error lines are sanitised too.
- **Import limits:** at most 32 MiB is read (`maxImportFileSize`); `decodeExportBundle` streams the
  task list, rejects more than 50,000 tasks (`maxImportTasks`) and any data after the bundle, and
  otherwise decodes like `json.Unmarshal`. The order is options → size → decode → validation →
  storage, so a rejected file never reaches storage.
- **Database location (plan 5):** `MUNUS_DB_PATH` wins when it is set and non-empty (the user
  manages that path and its directory). Otherwise the database is `munus.db` in the per-user data
  directory (`defaultDataDir`): `$XDG_DATA_HOME/munus` (absolute values only) or
  `~/.local/share/munus` on Linux and other Unix systems, `~/Library/Application Support/munus` on
  macOS, `%LOCALAPPDATA%\munus` on Windows (local, not roaming: it holds a SQLite file). The
  location is resolved at start without side effects (`pkg.LocateDatabase` calls
  `resolveDatabaseLocation`, whose inputs are injected for tests) and handed to
  `pkg.NewDeferredDatabaseAt`; the first open creates the directory
  (`ensurePrivateDir`), so help and version create nothing. A location that cannot be resolved (no
  home directory or `%LOCALAPPDATA%`, a relative path, a `?` in the path, which sqlite would read as
  parameters) is carried as `DatabaseLocation.Err` and reported by the first open with a hint to set
  `MUNUS_DB_PATH`. When the default is used and the working directory holds a regular file
  `munus.db` that is not the default database (where v3.0.0 and older kept it), the location carries
  a notice; the old file is never opened, moved or changed (D-21). Backups stay in
  `~/.munus/backups`.
- **Files:** new DB, export and backup files are `0600`; the backup directory and the default data
  directory are `0700`, both through `ensurePrivateDir` (created so, and tightened when others can
  access them). New database files are
  pre-created `0600` with `O_EXCL` for plain paths, `?` DSNs and `file:` URIs (`databaseFilePath`
  mirrors go-sqlite3 and sqlite's URI rules; a pre-created file is removed again if the driver then
  rejects the DSN). Exports use a random temp file + rename and refuse to overwrite the active
  database (compared via the file sqlite reports in `pragma_database_list`).
- **TUI transfers (plan 4, P-054):** export, import preview and import never run in `Update`. Each
  step starts as a Bubble Tea command (`startTransfer` gives it a `*transferOp`, which holds the
  cancel function of the step's context and identifies the step by pointer, so a late result never
  matches a newer step, even in another list model) that touches only values captured when it
  started, never the model, and reports a `transferResultMsg`; the import file is read through
  `ListModel.readImport`. While a step runs the dialog shows it and ignores keys except esc (cancel,
  close the dialog) and ctrl+c (cancel, quit). A result the dialog no longer waits for is dropped
  for a preview; for an export or import its outcome is still put in the status line (by the form
  too, which hands it to the list it returns to), and an import reloads the list. The import itself
  stays one storage transaction, which a cancelled context rolls back unless it has already
  committed; `ExportToFile` checks the context once more before writing. Two waits cannot be
  interrupted: a read blocked in the operating system (for example opening a FIFO without a writer)
  and SQLite's busy wait for another process's lock, which ends when the lock is released or the
  busy timeout passes; the interface stops waiting at once and reports the outcome when it comes.
- GORM's logger is silent (writer injected via `openDatabase`, `io.Discard` in production); errors
  are returned, never printed to stdout.

## Build constraints
- Builds that include the SQLite driver must use `CGO_ENABLED=1`; a `CGO_ENABLED=0`
  build compiles but cannot open a database.
- Releases (`cd.yml`) build natively per OS and architecture with CGO. Linux binaries come from
  the Dockerfile's `static` stage (`docker build --target static --output …` on the amd64 and
  arm64 runners): fully static with musl (no glibc, so no LGPL static-linking terms), `-tags
  sqlite_omit_load_extension,osusergo,netgo` (keep in step with `linuxReleaseTags` in
  `tools/licenses`), and `-Wl,-z,stack-size=8388608` because musl otherwise gives threads,
  including the ones cgo calls into SQLite on, a 128 KiB stack. The Docker build context excludes
  `.git`, so these binaries carry no VCS build information (`go version -m` still lists the
  modules). darwin/amd64 is built and smoke-run natively on `macos-15-intel` (supported until the
  macOS 15 image is retired in Fall 2027; revisit before then). windows/arm64 is built natively on
  the `windows-11-arm` runner with llvm-mingw (`CC=aarch64-w64-mingw32-clang`), downloaded from
  the pinned GitHub release and verified against its SHA-256 before use; bump both values
  together.
- The Dockerfile builds with CGO on Alpine for the image's own platform and runs as UID 10001 with
  `HOME=/app/data`. Stages: `source` (toolchain, modules, sources) → `builder` (image binary,
  dynamic musl) and `static-builder` → `static` (release binary, only built when targeted) → the
  runtime image, which also carries `LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES` in
  `/usr/share/licenses/munus/`. Both builds stamp `main.version` from the `VERSION` build argument
  (default `dev`; `cd.yml` passes the tag) and link with `-Wl,-z,stack-size=8388608` (musl's default
  thread stack is 128 KiB; `docker.yml` checks the image binary). Builder
  (`golang:1.26-alpine3.24`) and runtime (`alpine:3.24`) use the same Alpine release (same musl),
  and `apk` packages are pinned (`build-base`, `ca-certificates`, `tzdata`; the latter lets `TZ`
  select the container's time zone, checked by `docker.yml`). To bump: move both images to the
  same new Alpine release, look up the current package versions for that release (for example `apk
  policy build-base ca-certificates tzdata` in the new image, or the aports `x.y-stable` branch),
  update the pins, replace `tools/licenses/musl-COPYRIGHT` with the `COPYRIGHT` file of that
  release's musl version and update `muslVersion`/`muslAlpineRelease` in `tools/licenses/main.go`
  (its test fails until the Dockerfile and these agree), run `make licenses`, and confirm
  `hadolint Dockerfile` and `docker.yml` pass. A pin fails the build once Alpine drops that
  package revision, so bump when that happens.
- Third-party licenses: `THIRD_PARTY_LICENSES` is generated by `tools/licenses` (stdlib only) from
  the modules `go list -deps` reports for the six release platforms with cgo, plus the Go
  distribution's `LICENSE`/`PATENTS`, SQLite's public-domain dedication (from go-sqlite3's header)
  and musl's `COPYRIGHT`. It holds no module versions, so it changes only when a module is added or
  removed or a license text changes; `go test ./tools/licenses` fails when it, or the module list in
  `NOTICE`, is out of date. A module without a license file needs a reviewed entry in
  `readmeLicenses` (currently `mattn/go-localereader`). Release archives (`cd.yml`) and the image
  ship `LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES`.
- Supply chain (`cd.yml`): the `package` job (permissions `contents: read`, `id-token: write`,
  `attestations: write`) checks out the repository without persisting credentials, packages the
  archives with the license files, writes `checksums.txt` and attests the archives and checksums
  with SHA-pinned `actions/attest-build-provenance` (attestations need a public repository or
  GitHub Enterprise Cloud); the `release` job (tags only, `contents: write`, no OIDC) downloads
  those files and publishes them. Container image (GitHub Packages): the `build` job's Linux rows
  also build the runtime image (`--pull`, `--provenance=false`, OCI labels for source, revision and
  version), smoke-test it (`--version`, a database on a volume, license files) and upload it with
  `docker save`; the `image` job (tags only, after `release`; `packages: write` only, no checkout,
  no build, no OIDC) loads both images, checks each one's platform, pushes `<tag>-amd64` and
  `<tag>-arm64` and joins them with `docker buildx imagetools create` into
  `ghcr.io/<owner>/munus:<tag>` (plus `latest` for `vX.Y.Z` tags; a `+` in the tag becomes `_`);
  `image-attest` (`contents: read`, `id-token: write`, `attestations: write`) attests that index
  digest only, without pushing the attestation to the registry. Set no index annotations (N-042). The
  version string passed to the builds must match `^[0-9A-Za-z._+-]+$`. Linux builds use the
  Dockerfile's images with `docker build --pull` (latest Go 1.26 patch in `golang:1.26-alpine3.24`;
  `go version` is printed in the build log). Dependabot (`.github/dependabot.yml`) proposes weekly,
  grouped updates for GitHub Actions (SHA pins), Go modules and Docker images; Go minor releases of
  the `golang` image are ignored because they also change `go.mod` and CI. The smoke-test images
  `debian:11` and `alpine:3.24` in `cd.yml` and `docker.yml` are not tracked by Dependabot; move
  them together with the Alpine release.
- Smoke tests in steps with `shell: bash` (which adds `pipefail`) use `munus … | grep pattern >
  /dev/null`, not `grep -q`: `grep -q` exits at the first match and munus can then fail with
  SIGPIPE (about 5% of runs, N-038).

## Maintainability guidance
- Keep changes scoped; follow the nearest existing pattern (see `AGENTS.md`).
- Any change to import must keep `PlanImport` and `ApplyImport` on the same merge logic
  (`mergeVersion`) and preserve the ID rules above.
- Import reads the current tasks, writes the backup and replaces the tasks inside one storage
  transaction (`ReplaceAllTasksFunc`), so concurrent writes are never lost; the preview can still
  differ from the result if another process writes between plan and apply.
- `make check` (fmt-check, vet, test, race, lint) covers CI's vet, lint and test steps locally and
  adds formatting and race checks that CI does not run (CI's `go mod tidy` drift check has no
  target); keep them in step with `ci.yml`. CI, CD and
  the govulncheck job use the latest Go 1.26 patch release (`go-version: "1.26.x"`,
  `check-latest: true`); `go.mod` keeps the minimum version. Release builds do not use the setup-go
  cache.
  `make fuzz` runs the fuzz targets (deadline parsing, terminal sanitising, import invariants and
  export round trip); run it after changing those areas.
