# Active Plans & Follow-on Work

IDs reference `notes.md` (N-) and `cybersec.md` (SEC-). Per `CONTRIBUTING.md`, each change needs
an issue before a PR. Plan 1 (P-001…P-014) is complete and released as `v2.1.1`. Plan 2 below is
complete: phases A, B and C were committed to `main` (`68d60fa`…`44c6452`) and marked done by the
maintainer on 2026-09-27. Plan 3 below is the active plan: it addresses the findings of the 2026-09-27
review (`notes.md` N-026…, `cybersec.md` SEC-013…). The maintainer decided (D-1) to hold `v2.2.0`
until plan 3 phase A lands, so `v2.2.0` = plan 2 + plan 3 phase A; phases B and C target `v2.2.1`.
Phase A (and P-043) is implemented as an uncommitted change set (2026-09-27) awaiting maintainer
review and CI.

## Plan 3 — drafted and decided 2026-09-27 (phase A → v2.2.0; phases B and C → v2.2.1)

Source: the 2026-09-27 review of `48b4da2` (`notes.md` N-026…N-036, `cybersec.md` SEC-013…SEC-018);
every item was reproduced with the HEAD binary or a test. Checked against `v2.1.1`: SEC-014, SEC-015,
SEC-016, SEC-017, N-026 (`c` key) and N-030 already exist in the released version; the stored-status
path of SEC-013, N-026 (`s` key) and the trigger half of N-027 are new in plan 2.

### Maintainer decisions (2026-09-27)

| ID | Decision | Options | Chosen |
|---|---|---|---|
| D-1 | Release order | (a) tag `v2.2.0` now and ship plan 3 as `v2.2.1`; (b) hold `v2.2.0` until phase A lands | **(b)**: `v2.2.0` ships with plan 3 phase A; phases B and C follow as `v2.2.1` |
| D-2 | Import task cap (SEC-016) | a per-file limit on the number of tasks | **50,000 tasks**, enforced while decoding |
| D-3 | Files that already contain an empty title (SEC-014) | default import (a) rejects with the task index, or (b) imports it as `(untitled)`; `--strict` rejects either way | **(b)** |
| D-4 | ID rule (SEC-015) | (a) always keep IDs of tasks already in the database, cap only IDs read from a file; (b) also lower the cap | **(b)**: existing IDs are always kept and the cap for IDs from a file is lowered to 1,000,000,000 (value chosen during implementation: after importing the largest allowed ID, more than a billion further tasks still get IDs below 2^31; maintainer to confirm) |
| D-5 | Go toolchain for CI/CD (SEC-018) | (a) `go-version: "1.26.x"` + `check-latest: true` in `ci.yml`/`cd.yml`; (b) a `toolchain` line in `go.mod` (also switches contributors' toolchains) | **(a)** (`go mod tidy` with Go 1.26.8 leaves `go.mod` unchanged, verified 2026-09-27) |
| D-6 | Relative days and weeks (N-031) | (a) calendar arithmetic (`1d` = same clock time tomorrow); (b) keep fixed 24 h / 168 h and document it | **(a)** |
| D-7 | Unicode format characters (SEC-013, optional part) | (a) reject/replace bidi embedding, override and isolate controls (U+202A–U+202E, U+2066–U+2069) only; (b) all category Cf; (c) none | **(a)** |
| D-8 | Usage text on errors (N-034) | (a) usage only for argument and flag errors; (b) unchanged | **(a)** |

Go rules: the plan-2 rules from `intel/golang.md` apply unchanged (context first, no global mutable state,
wrapped errors, deterministic table-driven tests, no new dependencies). Changing the internal `Storage`
interface is allowed for P-033; test fakes are updated with it. Every item ships with regression tests,
README/`intel` updates where behaviour changes, a `history.md` entry, and green CI. Workflow and
`Makefile` changes are delivered as a patch (the linked-computer bridge cannot write
`.github/workflows/` or the `Makefile`).

### Phase A — data integrity and security (land first)

| ID | Priority | Item | Ref | Acceptance | Status |
|---|---|---|---|---|---|
| P-031 | High | **Stable IDs above the cap.** An ID is kept when it is in 1..1,000,000,000 (`maxImportedTaskID`, lowered from 2^31-1 per D-4) or is the canonical spelling of a task already in the database; numeric file IDs are canonicalised before merging. `PlanImport` and `ApplyImport` stay on the same merge logic. | SEC-015 | Import the largest allowed ID, `add`, empty merge import, export, re-import (merge and replace): IDs unchanged, no duplicates, plan == apply; `TestImportHugeIDGetsNewDatabaseID` still passes; file IDs above the cap get new IDs | Done (uncommitted) |
| P-032 | High | **Non-empty titles everywhere.** One shared check (title non-empty after trimming) used by `add`, `edit`, the TUI form and import; import applies it after stripping control characters. `add` also rejects a whitespace-only description, like the TUI form. Existing files per D-3. | SEC-014, N-028 | Control-only and whitespace-only titles: `--strict` rejects with the task index, default import per D-3; `add -t "  "` rejected; property test: every file import accepts exports to a file that re-imports with `--strict`; P-043 fuzz target passes without its empty-title exception | Done (uncommitted) |
| P-033 | Medium | **Status changes without stale writes.** New `Storage.SetTaskStatus(ctx, id, status, now)` changes only `status`, `completed`, `completed_at` and `updated_at` in one transaction, with `setStatus` transition rules, and returns `ErrTaskNotFound`; TUI `c`/`s` and CLI `complete` use it. | N-026 | Edit title and tags through a second handle, then TUI `c`, TUI `s` and `munus complete`: edits kept; deleted task → reload, no error shown; `complete --undo` still leaves `doing` alone; completing a done task keeps `completed_at` | Done (uncommitted) |
| P-034 | Medium | **Sanitise status and error output.** `PrintList` prints a sanitised status (derived from `completed` when unknown); the migration repairs unknown statuses, writing only when needed; `main` prints errors itself (`SilenceErrors`) through `sanitizeForTerminal`, keeping the `Error: ` line on stderr; the TUI form error line is sanitised; bidi controls per D-7. | SEC-013 | Tests with ESC/BEL in `status` and a trigger `RAISE` message: no control characters in `list` output, `complete` error output or the form view; migration test for an unknown status; an up-to-date DB still opens read-only; binary check `munus list \| od -c` shows no `033` | Done (uncommitted) |
| P-035 | Low | **Bounded, exact import decoding.** Decode the `tasks` array with a streaming `json.Decoder` and stop at the task cap (D-2) before merging or any storage call; reject trailing data after the bundle in both modes. Same limits for files, stdin and the TUI. | SEC-016, N-029 | Cap + 1 tasks rejected with a clear error and no storage call; `{…} {…}` and `{…}garbage` rejected with and without `--strict`; `--strict` still rejects unknown fields; peak RSS for a maximal accepted file recorded in `notes.md` | Done (uncommitted) |
| P-036 | Low | **Owner-only DSN database files.** A database file this process creates through a `file:` URI or a `?` DSN is tightened to `0600`; existing files keep their mode. | SEC-017 | Non-Windows tests for both DSN forms (new file `0600`) and for an existing `0644` file (unchanged) | Done (uncommitted) |

### Implementation notes — phase A (2026-09-27)
- Implemented in the order P-043 → P-031 → P-032 → P-033 → P-034 → P-035 → P-036. Details beyond the
  table: `add` also rejects a whitespace-only description (as the TUI form does); `complete --undo`
  writes nothing for a task that is not done; the root command sets `terminalSafeWriter` as its error
  writer and `main` writes a database-close error through it; import rejects a `null` task entry;
  empty or truncated import data reports "unexpected end of JSON input" (not `EOF`); the migration keeps
  a `doing` stored in another case; a database file pre-created for a DSN that the driver then rejects
  (for example `?_busy_timeout=abc`) is removed again. `make fuzz` runs the three fuzz targets.
- An independent review of the change set found no High issues and no regression of SEC-001…SEC-017;
  its findings were fixed: the CLI `complete` test did not cover concurrent edits (now guarded by
  triggers), a non-canonical spelling of an existing ID could be written twice, `complete --undo`
  could overwrite a task changed to `doing` in between, `databaseFilePath` differed from sqlite for a
  leading `?`, invalid or `%00` escapes, repeated `mode` and file-backed `vfs` values, truncated files
  reported `EOF`, and two tests were weak (unchecked errors, no Windows drive-letter case).
- Mutation check: 27 targeted reversions of the phase A fixes; the tests catch 26. The remaining one
  (re-saving the row read inside `SetTaskStatus`'s own transaction) changes nothing observable.
- Measured (HEAD binary vs change set): the review's 32 MiB file of 2.4 million tasks took 8.7 s and
  1.96 GB RSS for `--dry-run`; it is now rejected in 0.1 s at 98 MB. The largest accepted file
  (50,000 full-length tasks, 31 MB) needs 120 MB for `--dry-run` and 174 MB (3.6 s) to import.

### Phase B — robustness and behaviour fixes

| ID | Priority | Item | Ref | Acceptance | Status |
|---|---|---|---|---|---|
| P-037 | Medium | **Concurrent first open.** Triggers use `CREATE TRIGGER IF NOT EXISTS`; schema creation and migration are safe when several processes open a new or older database at once (take the write lock only when a migration is needed and re-check under it, so an up-to-date DB still opens read-only). | N-027 | The review's reproduction (3 parallel `add` × 40 new DBs; 3 parallel `list` × 40 trigger-less DBs) has 0 failures; read-only open test passes; `go test -race` | Proposed |
| P-038 | Low | **CLI error handling.** `PersistentPreRunE` runs `ValidateRequiredFlags` and `ValidateFlagGroups` before opening the database; usage is printed only for argument and flag errors (D-8). | N-033, N-034 | `add -t x` and `list --pending --completed` create no DB file; `complete 999` prints one error line; help, version and completion tests still pass | Proposed |
| P-039 | Low | **Form cursor.** Enter moves to the end of the next field, as Tab and ↓ do. | N-030 | Model test: edit form, Enter, type → text appended | Proposed |
| P-040 | Low | **Calendar relative deadlines.** `d`/`w` use calendar arithmetic (D-6); the parser takes the current time as an input (`ParseDeadline` wraps it with `time.Now()`). | N-031 | Deterministic tests across both 2026 US DST changes with fixed zones (`time/tzdata`); SEC-008 bounds tests unchanged | Proposed |
| P-041 | Low | **Docker time zones.** Pinned `tzdata` in the runtime image (same Alpine release as the other pins); README documents `-e TZ=…`. | N-032 | hadolint clean; `docker.yml` smoke test with `TZ` set stores a deadline with the expected offset (workflow patch) | Proposed |
| P-042 | Info | **Small fixes.** `export -f -` writes to standard output (like `import -f -` reads it); `ApplyImport` honours `DryRun` (plan only, no write); `openDatabase` closes the connection when a migration fails; the TUI path prompt accepts multi-byte characters. Deferred: moving TUI import/export off the `Update` path (needs a design; P-035 bounds the worst case). | N-035, N-036 | Tests for each; `export -f - \| import -f - --dry-run` round-trips | Proposed |

### Phase C — tooling, CI/CD and hygiene

| ID | Priority | Item | Ref | Acceptance | Status |
|---|---|---|---|---|---|
| P-043 | Low | **Fuzz targets in the repository.** `FuzzParseDeadline`, `FuzzSanitizeForTerminal` and `FuzzParseImportData` (import invariants plus export round trip), with seed corpora that run in `go test`; `make fuzz` runs each for `FUZZTIME` (default 30s). | review | `go test ./...` runs the seeds; each target runs 60 s clean once P-032 lands | Done (uncommitted), with phase A; the `make fuzz` target is delivered as `makefile-fuzz.patch` (apply with `git apply`) |
| P-044 | Low | **CI/CD hardening (workflow patch).** Go toolchain per D-5; `cache: false` for the `cd.yml` build job; optional govulncheck job in `security.yml` with the action pinned by SHA. | SEC-018 | CI and CD logs show the patched Go version; govulncheck clean; actionlint clean | Proposed |
| P-045 | Low | **Ignore rules.** `.gitignore` adds `munus-export-*.json`, `*.db-journal`, `*.db-wal`, `*.db-shm`; `.dockerignore` adds `**/*.db-wal`, `**/*.db-shm`. | SEC-018 | `git check-ignore` matches each pattern; Docker build context excludes them | Proposed |

### Security requirements for plan 3
- No closed SEC item may regress (SEC-001…SEC-012), in particular terminal sanitising (SEC-005, SEC-010),
  import limits (SEC-006), the ID cap (SEC-009) and database file permissions (SEC-001).
- Import keeps its order of checks: options → size cap → decode (task cap, trailing data) → validation →
  storage.
- SEC-013…SEC-018 are closed only after their documented validation (`AGENTS.md`); new workflow
  actions are pinned by SHA; new findings get SEC-019+.

### Validation for every item
As plan 2 (`gofmt -s`, `go vet ./...`, `go test ./...`, `go test -race ./...`, staticcheck locally,
golangci-lint v2.13.2 in CI), plus: the review's binary-level reproduction for each item, the P-043 fuzz
targets for 60 s each, and `GOOS=windows`/`GOOS=darwin go vet ./...` with CGO off. Commands that cannot
be run are listed as not run.

### Suggested order
Phase A and P-043 are done (uncommitted): maintainer review → commit → green CI, Docker and Security
runs → tag `v2.2.0` (plan 2 + plan 3 phase A; its `cd.yml` run is also the first windows/arm64 build,
P-027). Then P-037 → P-038…P-042 → P-044, P-045 → release `v2.2.1`.
Release notes for `v2.2.0` must list, besides the plan-2 changes: import rejects files over 50,000
tasks and data after the export; blank titles import as `(untitled)` (`--strict` rejects them); file
IDs above 1,000,000,000 get new IDs; bidi override characters are rejected like control characters;
`add` rejects blank titles and descriptions. Release notes for `v2.2.1`: `d`/`w` are calendar-based
(D-6) and runtime errors no longer print usage (D-8).

## Plan 2 — drafted 2026-09-24, completed 2026-09-27 (target release: v2.2.0)

Maintainer decisions (2026-09-24):
- Features in scope: task editing, list filters, and a status + tags data model.
- `munus export` will include completed tasks by default.
- Maintenance in scope: developer Makefile targets, a windows/arm64 release, pinned Alpine packages.
- `CONTRIBUTING.md` will be drafted from verified repository facts and reviewed before commit.
- Not selected: moving TUI model structs out of `database.go` (deferred). Still in force from plan 1:
  default DB location stays `./munus.db`; export refuses only the active DB.

Amended 2026-09-24 after reviewing `intel/golang.md` (maintainer-approved): added P-029 (context
propagation) and P-030 (conformance of plan-1 code); P-017 and P-021 tests must be deterministic;
validation adds `staticcheck` locally where buildable.

Go rules for all plan-2 code (from `intel/golang.md`): I/O operations take `context.Context` first
(never stored in structs); no global mutable state; errors handled or discarded only with a documented
reason, wrapped with `%w` where callers classify them; new identifiers unexported unless needed outside
`internal`; doc comments on exported identifiers; table-driven, deterministic tests using `t.Helper`,
`t.TempDir`, `t.Cleanup`, no sleeps and no dependence on the machine's time zone; no new dependencies.

Every item ships with regression tests, README/`intel` updates where behaviour changes, and green CI.
Workflow changes must be delivered as a patch (the linked-computer bridge cannot write
`.github/workflows/`).

### Phase A — defects and small behaviour changes (low risk, land first)

| ID | Priority | Item | Ref | Acceptance | Status |
|---|---|---|---|---|---|
| P-030 | Medium | Conformance of plan-1 code with `golang.md`: replace the package-level `gormLogWriter` with an injected writer; handle or document every `_ =` error discard (report `db.Close()` failure in `main`, return the backup-dir `chmod` error); unexport `ValidateTaskText`, `MaxImportFileSize`, `DatabasePath` (unused outside `internal`); remove the `time.Sleep` in `TestNewID`. No behaviour change. | golang.md | gofmt/vet/tests green; no package-level mutable vars besides constants/regexps | Done |
| P-029 | High | Context propagation: `Storage` methods, import/export/backup functions take `ctx context.Context` first (GORM `WithContext`); `main` uses `signal.NotifyContext` + `ExecuteContext` so Ctrl+C cancels CLI work; the TUI passes `context.Background()` per operation at its event boundary (never stored in models). Test mocks updated. | golang.md | Cancelled context aborts DB and import operations (tests); all callers pass ctx | Done — see deviation below |
| P-015 | High | `export` includes completed tasks by default (CLI and TUI `alt+c` default). Add `--pending-only`; keep `-i/--include-completed` accepted as a deprecated no-op. Call out in release notes. | N-018 | `export` → `import --mode replace` round-trips every task; tests for CLI flag matrix and TUI default | Done |
| P-016 | Medium | TUI import confirm: only `y` applies (Enter no longer confirms), `n`/`esc`/`q` cancel, `ctrl+c` quits. | N-023, notes Info | Enter, Enter on a replace preview leaves tasks untouched | Done |
| P-017 | Medium | Deadline labels: an overdue task never shows "Due today!"; "Due today" means the same local calendar day; overdue/remaining days counted in calendar days. Label logic takes the current time and location as inputs (injected clock). | N-020 | Deterministic tests with fixed times at −5h, +23h, across midnight and a DST change (fixed zones via `time/tzdata`); no dependence on the machine's zone | Done |
| P-018 | Low | TUI keeps the terminal size when switching form ↔ list (carry the last size; request `tea.WindowSize()` on switch). | N-024 | List created via `ctrl+l` has the real viewport size | Done |
| P-019 | Low | `?` shows a full key-help panel (`showHelp` is currently never rendered). | notes Info | View changes on `?`; help lists every binding shown in README | Done |
| P-020 | Low | `import -f -` reads stdin with the same 32 MiB cap and validation; `--mode replace` from stdin requires `--yes` (stdin cannot also answer the prompt). | notes Info | Pipe import works; oversized/invalid stdin rejected; replace without `--yes` errors | Done |
| P-021 | Low | Merge import reads current tasks inside the write transaction (removes the lost-update window). | notes residual risk | Deterministic test: a write made between plan and apply is kept (hook, no sleeps); `go test -race` | Done |

### Phase B — data model and features

| ID | Priority | Item | Ref | Acceptance | Status |
|---|---|---|---|---|---|
| P-022 | High | **Status + tags model.** `status` column (`todo`/`doing`/`done`) with `completed` kept in sync for compatibility and backfilled on migration; tags in `tags` + `task_tags` (many-to-many). Tag rules: 1–32 chars of letters, digits, `-`, `_`, stored lowercase, max 10 per task. Export schema **v2** adds `status` and `tags`; import accepts v1 (status derived from `completed`) and v2. `ExportPlan.Doing` populated. | N-007 | Migration test from a v2.1.1 database file; v1 and v2 import tests; v2 export golden test; tag validation tests | Done |
| P-023 | High | **Edit tasks.** `munus edit <id>` with `--title/-t`, `--description/-d`, `--deadline/-n`, `--clear-deadline`, `--status`, `--tag`, `--untag`; only given fields change; same validation as `add`; unknown ID errors. `add --tag`; `complete` sets `done`, `--undo` sets `todo`. TUI: `u` opens the form pre-filled for the selected task; `s` cycles status. | new | CLI flag matrix tests; TUI edit round-trip; validation parity with `add` | Done |
| P-024 | Medium | **List filters.** `munus list --pending`, `--completed`, `--overdue`, `--status <s>`, `--tag <t>` (combinable, AND semantics; `--pending`/`--completed` mutually exclusive); list output shows status and tags. TUI: `F` cycles all → pending → doing → overdue → done, `#` filters by tag; active filter shown in the header. | old README | Filter combination tests; TUI filter keeps cursor/paging valid | Done |

Export v2 note: older Munus versions (≤ v2.1.1) reject version-2 files. Release notes must say so.

### Implementation notes (2026-09-24)
- **P-029 deviation (accepted with the phase A sign-off, 2026-09-27):** `main` runs `rootCmd.ExecuteContext(context.Background())`
  instead of `signal.NotifyContext`. Trapping SIGINT would stop Ctrl+C from ending blocking prompts
  (`Confirm` reads stdin, which a context cannot interrupt) and the TUI already handles `ctrl+c`, so the
  default Ctrl+C behaviour (the process exits; SQLite rolls back an unfinished transaction) is kept.
  Every command passes `cmd.Context()` to storage and import/export, so a cancelled context aborts
  before any write (tested with `ExecuteContext`).
- An independent review of the change set found 12 issues, all fixed with regression tests: merging a
  v1 file now keeps existing tags and `doing`; migration writes only when needed (an up-to-date DB opens
  read-only); SQLite triggers keep `status` and tag links consistent when v2.1.1 writes to a migrated
  DB; the TUI form edits by rune (UTF-8 safe); `complete --undo` leaves `doing` tasks alone;
  `UpdateTask` returns `ErrTaskNotFound` instead of re-creating a task deleted elsewhere; `edit`
  validates only changed fields (legacy text stays editable); tags are sanitised on output (SEC-010);
  `--strict` help matches behaviour; the Docker builder and runtime use the same Alpine release;
  `/munus` (`make build` output) is git-ignored; docs drift corrected.
- Security requirements met: new input uses `validateTaskText`/`normalizeTags`, output goes through
  `sanitizeForTerminal`, stdin imports use the 32 MiB cap, the llvm-mingw download is pinned by
  version and SHA-256, and the migration keeps DB file permissions (`TestNewDatabaseMigratesV211Database`).

### Phase C — tooling and documentation

| ID | Priority | Item | Ref | Acceptance | Status |
|---|---|---|---|---|---|
| P-025 | Medium | Draft `CONTRIBUTING.md` from repo facts: issue-first rule, license header, `gofmt -s -w .`, Go 1.26 + C compiler (`CGO_ENABLED=1`), `make` targets, CI checks, PR expectations, `intel/` upkeep. Maintainer reviews before commit. | AGENTS.md | Every command in it verified to run | Done — reviewed and committed in `68d60fa` |
| P-026 | Low | Makefile dev targets: `build`, `test`, `race`, `cover`, `vet`, `fmt`, `lint` (golangci-lint v2.13.2 when installed), `check` (all of them); keep release targets; update `make help`. | new | Each target runs locally; CI commands unchanged | Done |
| P-027 | Low | windows/arm64 release: `cd.yml` entry on the `windows-11-arm` runner. Spike first: confirm whether the image has a usable C compiler; otherwise install a pinned, SHA-256-verified llvm-mingw toolchain. Smoke-run the binary; add `munus_windows_arm64.zip` to README. | notes | Tagged release publishes a working windows/arm64 zip | Done — `cd.yml` entry committed; the first windows/arm64 zip is published by the `v2.2.0` tag |
| P-028 | Low | Pin Alpine packages (`build-base`, `ca-certificates`) to the versions current at implementation; document the bump procedure in `maint.md`. | hadolint DL3018 | hadolint clean; Docker workflow green | Done |

### Security requirements for plan 2
- New input (edited fields, tags, stdin imports) goes through the existing validation and
  sanitising (`validateTaskText`, unexported by P-030; `sanitizeForTerminal`; 32 MiB cap); no closed SEC item may regress.
- The llvm-mingw download is pinned by version and SHA-256; new workflow actions are SHA-pinned.
- The schema migration must not loosen database file permissions.
- New findings get a SEC-010+ entry in `cybersec.md`.

### Validation for every item
`gofmt -w -s` on modified files, `go vet ./...`, `go test ./...`, `go test -race ./...` (P-021, P-029
and any concurrent code), `staticcheck ./...` locally where it can be built, golangci-lint v2.13.2 in
CI, `go mod tidy` only if dependencies change (none planned). Commands that cannot be run are listed
as not run.

### Suggested order
P-030 → P-029 → Phase A (P-015 → P-021) → P-022 → P-023 → P-024 → Phase C, then release v2.2.0.
If the export default change (P-015) is treated as breaking for scripted users, release as v3.0.0.

Remaining before release: per plan 3 decision D-1, `v2.2.0` waits for plan 3 phase A; then tag
`v2.2.0` (its `cd.yml` run is the first windows/arm64 build, P-027) with release notes covering the
export default change (P-015) and export schema v2 (≤ v2.1.1 rejects it) as well as the plan 3 phase A
changes.

## Completed — Plan 1 (released in v2.1.1)

Maintainer decisions (2026-09-23): native per-OS CGO release builds (not a pure-Go driver); keep
`./munus.db` as the default DB location; export refuses only the active DB (other files may be
overwritten); leave `CONTRIBUTING.md`/`NOTICE` untouched (superseded for `CONTRIBUTING.md` by
plan 2, P-025).

| ID | Priority | Item | Ref | Status |
|---|---|---|---|---|
| P-001 | High | Working release binaries: per-OS CGO builds, smoke runs, CI runs a built binary | N-002 | Done — CI green; release `v2.1.1` built, smoke-tested and published by `cd.yml` |
| P-002 | High | Fix absolute deadline layout (`15:04`) in code and test | N-001 | Done |
| P-003 | High | Import integrity: stable IDs, `CompletedAt`, `--id-strategy`, deadline compare, option validation before backup, plan == apply | N-003, N-004, N-014, N-015 | Done |
| P-004 | Medium | TUI list fixes | N-008–N-011, N-021, N-022 | Done |
| P-005 | Medium | Don't create DB for `--help`; document `MUNUS_DB_PATH` (default location kept) | N-005 | Done |
| P-006 | Medium | Import/export hardening: control chars, limits, safe temp file, refuse active DB | SEC-005–007 | Done |
| P-007 | Medium | README corrections | notes | Done |
| P-008 | Medium | CI/CD security: pin actions to SHAs, upload gosec SARIF | SEC-002, SEC-003 | Done — SEC-002 and SEC-003 closed |
| P-009 | Low | Deadline parser: multi-month values, bounds/overflow | N-012, N-013, SEC-008 | Done |
| P-010 | Low | CLI polish: `delete` missing IDs, consistent prompt, silent GORM logger, `--version` | N-016, N-017, N-006 | Done |
| P-011 | Low | Dockerfile: non-root, `.dockerignore`, drop `sqlite-libs`, no hard-coded `GOARCH` | SEC-004 | Done — Docker workflow green (SEC-004 closed) |
| P-012 | Low | File permissions 0600/0700; backups via `os.UserHomeDir()` with unique names | SEC-001, N-019 | Done |
| P-013 | Low | Test hygiene | notes "Info" | Done |
| P-014 | Low | Remove `cryptare-*` builds in CI; bump codecov action | notes | Done — Codecov upload confirmed |
