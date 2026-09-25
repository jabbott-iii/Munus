# Active Plans & Follow-on Work

IDs reference `notes.md` (N-) and `cybersec.md` (SEC-). Per `CONTRIBUTING.md`, each change needs
an issue before a PR. Plan 1 (P-001…P-014) is complete and released as `v2.1.1`; plan 2 below is
the active plan. Plan 2 is implemented as an uncommitted change set (2026-09-24) awaiting maintainer
review, CI and a release.

## Plan 2 — drafted 2026-09-24 (target release: v2.2.0)

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
| P-030 | Medium | Conformance of plan-1 code with `golang.md`: replace the package-level `gormLogWriter` with an injected writer; handle or document every `_ =` error discard (report `db.Close()` failure in `main`, return the backup-dir `chmod` error); unexport `ValidateTaskText`, `MaxImportFileSize`, `DatabasePath` (unused outside `internal`); remove the `time.Sleep` in `TestNewID`. No behaviour change. | golang.md | gofmt/vet/tests green; no package-level mutable vars besides constants/regexps | Done (uncommitted) |
| P-029 | High | Context propagation: `Storage` methods, import/export/backup functions take `ctx context.Context` first (GORM `WithContext`); `main` uses `signal.NotifyContext` + `ExecuteContext` so Ctrl+C cancels CLI work; the TUI passes `context.Background()` per operation at its event boundary (never stored in models). Test mocks updated. | golang.md | Cancelled context aborts DB and import operations (tests); all callers pass ctx | Done (uncommitted) — see deviation below |
| P-015 | High | `export` includes completed tasks by default (CLI and TUI `alt+c` default). Add `--pending-only`; keep `-i/--include-completed` accepted as a deprecated no-op. Call out in release notes. | N-018 | `export` → `import --mode replace` round-trips every task; tests for CLI flag matrix and TUI default | Done (uncommitted) |
| P-016 | Medium | TUI import confirm: only `y` applies (Enter no longer confirms), `n`/`esc`/`q` cancel, `ctrl+c` quits. | N-023, notes Info | Enter, Enter on a replace preview leaves tasks untouched | Done (uncommitted) |
| P-017 | Medium | Deadline labels: an overdue task never shows "Due today!"; "Due today" means the same local calendar day; overdue/remaining days counted in calendar days. Label logic takes the current time and location as inputs (injected clock). | N-020 | Deterministic tests with fixed times at −5h, +23h, across midnight and a DST change (fixed zones via `time/tzdata`); no dependence on the machine's zone | Done (uncommitted) |
| P-018 | Low | TUI keeps the terminal size when switching form ↔ list (carry the last size; request `tea.WindowSize()` on switch). | N-024 | List created via `ctrl+l` has the real viewport size | Done (uncommitted) |
| P-019 | Low | `?` shows a full key-help panel (`showHelp` is currently never rendered). | notes Info | View changes on `?`; help lists every binding shown in README | Done (uncommitted) |
| P-020 | Low | `import -f -` reads stdin with the same 32 MiB cap and validation; `--mode replace` from stdin requires `--yes` (stdin cannot also answer the prompt). | notes Info | Pipe import works; oversized/invalid stdin rejected; replace without `--yes` errors | Done (uncommitted) |
| P-021 | Low | Merge import reads current tasks inside the write transaction (removes the lost-update window). | notes residual risk | Deterministic test: a write made between plan and apply is kept (hook, no sleeps); `go test -race` | Done (uncommitted) |

### Phase B — data model and features

| ID | Priority | Item | Ref | Acceptance | Status |
|---|---|---|---|---|---|
| P-022 | High | **Status + tags model.** `status` column (`todo`/`doing`/`done`) with `completed` kept in sync for compatibility and backfilled on migration; tags in `tags` + `task_tags` (many-to-many). Tag rules: 1–32 chars of letters, digits, `-`, `_`, stored lowercase, max 10 per task. Export schema **v2** adds `status` and `tags`; import accepts v1 (status derived from `completed`) and v2. `ExportPlan.Doing` populated. | N-007 | Migration test from a v2.1.1 database file; v1 and v2 import tests; v2 export golden test; tag validation tests | Done (uncommitted) |
| P-023 | High | **Edit tasks.** `munus edit <id>` with `--title/-t`, `--description/-d`, `--deadline/-n`, `--clear-deadline`, `--status`, `--tag`, `--untag`; only given fields change; same validation as `add`; unknown ID errors. `add --tag`; `complete` sets `done`, `--undo` sets `todo`. TUI: `u` opens the form pre-filled for the selected task; `s` cycles status. | new | CLI flag matrix tests; TUI edit round-trip; validation parity with `add` | Done (uncommitted) |
| P-024 | Medium | **List filters.** `munus list --pending`, `--completed`, `--overdue`, `--status <s>`, `--tag <t>` (combinable, AND semantics; `--pending`/`--completed` mutually exclusive); list output shows status and tags. TUI: `F` cycles all → pending → doing → overdue → done, `#` filters by tag; active filter shown in the header. | old README | Filter combination tests; TUI filter keeps cursor/paging valid | Done (uncommitted) |

Export v2 note: older Munus versions (≤ v2.1.1) reject version-2 files. Release notes must say so.

### Implementation notes (2026-09-24)
- **P-029 deviation (maintainer to confirm):** `main` runs `rootCmd.ExecuteContext(context.Background())`
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
| P-025 | Medium | Draft `CONTRIBUTING.md` from repo facts: issue-first rule, license header, `gofmt -s -w .`, Go 1.26 + C compiler (`CGO_ENABLED=1`), `make` targets, CI checks, PR expectations, `intel/` upkeep. Maintainer reviews before commit. | AGENTS.md | Every command in it verified to run | Drafted — maintainer review before commit |
| P-026 | Low | Makefile dev targets: `build`, `test`, `race`, `cover`, `vet`, `fmt`, `lint` (golangci-lint v2.13.2 when installed), `check` (all of them); keep release targets; update `make help`. | new | Each target runs locally; CI commands unchanged | Done (uncommitted) |
| P-027 | Low | windows/arm64 release: `cd.yml` entry on the `windows-11-arm` runner. Spike first: confirm whether the image has a usable C compiler; otherwise install a pinned, SHA-256-verified llvm-mingw toolchain. Smoke-run the binary; add `munus_windows_arm64.zip` to README. | notes | Tagged release publishes a working windows/arm64 zip | Implemented — unverified until a tagged release runs `cd.yml` (workflow patch) |
| P-028 | Low | Pin Alpine packages (`build-base`, `ca-certificates`) to the versions current at implementation; document the bump procedure in `maint.md`. | hadolint DL3018 | hadolint clean; Docker workflow green | Implemented — Docker build not run locally (Docker Hub blocked); verify in `docker.yml` |

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

Remaining before release: maintainer review (including `CONTRIBUTING.md` and the P-029 deviation),
apply the workflow patch, green CI/Docker/Security runs, then a tag to prove the windows/arm64 build.

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
