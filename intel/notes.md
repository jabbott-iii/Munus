# Engineering Notes & Open Questions

## Plan 4 phase A baseline (2026-09-27; uncommitted change set on top of `97cf04b`, target `v2.2.2`)
Same toolchain as below (Go 1.26.8 + CGO, modules verified against `go.sum`): `gofmt -s -l .` clean;
`go vet ./...` clean (also `GOOS=windows`/`darwin`, CGO off); `go mod tidy` leaves `go.mod`/`go.sum`
unchanged; `go test ./...` passes (coverage `internal` 88.9%, root 25.0%, `tools/licenses` 90.1%,
88.7% total), also with `-short` and with umask 077; `go test -race -count=3 -shuffle=on ./...`
passes; `go run ./tools/licenses -check` passes; staticcheck, govulncheck v1.8.0 and gosec v2.29.0
(built from the SHA pinned in `security.yml`; 0 issues, the three new `#nosec` lines are G204, G304
and G306 with reasons) clean; hadolint 2.15.1 and actionlint 1.7.12 with shellcheck 0.11.0 clean;
`dependabot.yml` and the workflows validate against the SchemaStore schemas (check-jsonschema
0.38.2). Mutation check of `tools/licenses`: 16 targeted mutants, all caught. A dependency change
now invalidates a cached pass of the license test (reproduced before and after the fix). Static
build check with zig 0.16 (`zig cc -target x86_64-linux-musl`, musl 1.2.5) and the Dockerfile's
flags: "statically linked", no `INTERP`, `GNU_STACK` 8 MiB, add/list/complete/export/import/backup,
`TZ` and 20 concurrent `add`s work; a C and a cgo probe confirmed musl's 128 KiB default thread
stack and that `-z stack-size` raises the stacks Go's cgo threads get. The packaging loop was run
locally (archives contain the binary plus `LICENSE`, `NOTICE`, `THIRD_PARTY_LICENSES`; a missing
file fails the step). An independent review found no High or Medium issues; its Low findings were
fixed (test-cache invalidation, `GOWORK=off`/`GOFLAGS` and release tags for `go list`, OS-independent
ordering, image binary stack size, `go version` in the Linux build log with `docker build --pull`,
OIDC token kept out of the publishing job, N-038). Not run locally: the Docker build itself (no
daemon or registry access), Alpine's musl 1.2.6 build, the old-distro containers, macOS/Windows
test runs, golangci-lint, CodeQL, attestation and Dependabot — they need the GitHub runs.

## Plan 3 phases B–C baseline (2026-09-27; committed as `df70f29`, released in `v2.2.1`)
Same toolchain as below (Go 1.26.8 + CGO, modules verified against `go.sum`): `gofmt -s -l .` clean;
`go vet ./...` clean (also `GOOS=windows`/`darwin`, CGO off); `go mod tidy` leaves `go.mod`/`go.sum`
unchanged; `go test ./...` passes (coverage `internal` 88.9%, root 25.0%, 88.5% total), also with
`-short`, with umask 077 and under TZ UTC, America/Phoenix, America/Los_Angeles, Pacific/Chatham,
Asia/Kathmandu, Europe/London and America/St_Johns; `go test -race -count=3 -shuffle=on ./...`
passes; the two concurrency stress tests pass 5× in a row; staticcheck 2026.2.1 and govulncheck
v1.8.0 clean; `make fuzz` (30–45 s per target) clean; actionlint 1.7.12 with shellcheck 0.11.0 and
hadolint 2.15.1 clean. Mutation check: 22 targeted reversions, 19 caught (2 equivalent, 1 caught by
another test). Binary checks against v2.2.0: invalid flag values now print usage and create no
database; 6 parallel imports 101/150 → 0/150 "database is locked"; mixed concurrent commands
25/200 → 0/200; 3 parallel first opens 0/90 failures; `1d` keeps the clock time across DST;
`export -f -` writes standard output. Not run locally: golangci-lint, gosec, CodeQL, the Docker
build (including the `tzdata` pin), macOS/Windows test runs, the workflows themselves — all of
these ran green on GitHub for `v2.2.1` (maintainer-confirmed).

## Plan 3 phase A baseline (2026-09-27; committed as `c9eeb97`, released in `v2.2.0`)
Same toolchain as the review below (Go 1.26.8 + CGO, modules verified against `go.sum`):
`gofmt -s -l .` clean; `go vet ./...` clean (also `GOOS=windows`/`darwin`, CGO off); `go mod tidy`
leaves `go.mod`/`go.sum` unchanged; `go test ./...` passes (coverage `internal` 88.8%, root 25.0%,
88.4% total; was 87.7%), also with umask 077; `go test -race -count=3 -shuffle=on ./...` passes; tests
pass under TZ UTC, America/Phoenix, Pacific/Chatham, Asia/Kathmandu, Europe/London and
America/St_Johns; staticcheck 2026.2.1 and govulncheck v1.8.0 clean; `make fuzz` (3 targets, 45–60 s
each) clean. Mutation check: 27 targeted reversions of the fixes, 26 caught (the other is equivalent).
Binary-level re-runs of the review reproductions: SEC-013…SEC-017, N-028 and N-029 no longer
reproduce (details in `cybersec.md` and `plan.md`, "Implementation notes — phase A").
Import memory (SEC-016): the review's 2.4-million-task file is rejected in 0.1 s at 98 MB RSS (was
8.7 s / 1.96 GB for `--dry-run`); the largest accepted file (50,000 full-length tasks, 31 MB) needs
120 MB for `--dry-run` and 174 MB (3.6 s) to import.
Not run locally: golangci-lint, gosec, CodeQL, Docker build, macOS/Windows test runs.

## Review 2026-09-27 (HEAD `48b4da2`; plan 2 committed and marked done)
Validation on a copy of the tree with Go 1.26.8 + CGO (built from GitHub source; every module served
from a local file proxy and verified against `go.sum` by `go mod download`/`go mod verify`):
`gofmt -s -l .` clean; `go vet ./...` clean (also `GOOS=windows` and `GOOS=darwin` with CGO off);
`go mod tidy` leaves `go.mod`/`go.sum` unchanged; `go test ./...` passes (coverage `internal` 88.1%,
root 25.0%, 87.7% total); `go test -race -count=3 -shuffle=on ./...` passes; tests pass under TZ UTC,
America/Phoenix, Pacific/Chatham, Asia/Kathmandu, Europe/London and America/St_Johns; staticcheck
2026.2.1 clean (with `-checks all` only ST1000, missing package comment); govulncheck v1.8.0 (vulndb
snapshot 2026-09-24) reports no reachable vulnerabilities (see SEC-018 for Go 1.26.0). Fuzzing (60–120 s
each): `ParseDeadline`/`ParseRelativeTime`, `sanitizeForTerminal` and `parseImportData` (import
invariants and export round trip) found only the empty-title case (SEC-014).
Findings came from an independent blind review plus our own review; each was reproduced with the
HEAD binary or a test before being recorded: N-026–N-036 below and SEC-013–SEC-018 in `cybersec.md`.
They are planned as plan 3 in `plan.md` (P-031…P-045, draft awaiting maintainer decisions).
Not run locally: golangci-lint, gosec and CodeQL (run in CI), Docker build, macOS/Windows test runs,
the TUI in a real terminal (TUI behaviour was checked through the Bubble Tea models).

## Plan 2 baseline (2026-09-24, uncommitted change set on top of e32fd8a)
With Go 1.26.0 + CGO: `gofmt -s -l .` clean, `go vet ./...` clean, `go test ./...` passes, and
`go test -race -count=3 ./...` passes; tests also pass under TZ UTC, America/Los_Angeles, Asia/Kolkata,
Pacific/Chatham and Europe/London. Coverage: `internal` 88.1%, root package 25.0% (87.7% total; was
84.3%). staticcheck 2026.2.1, errcheck v1.20.0 and ineffassign v0.2.0 clean; actionlint 1.7.12 (with
shellcheck) and hadolint clean; `make check` (lint skipped) and `make build` pass; `go.mod`/`go.sum`
unchanged. Mutation check: 72 single-line mutations of plan-2 code; the tests catch 63. Of the rest,
6 change nothing observable (explicit tag-link deletes that the delete trigger also performs, the
silent GORM log writer, `add --tag` normalisation that `BeforeSave` repeats, and two storage calls
where a cancelled context still aborts the command before any write), 2 target code that no longer
exists and 1 did not compile.
End-to-end with the v2.1.1 binary and the new binary: a v2.1.1 database migrates with statuses
backfilled and permissions kept; v2.1.1 writes to a migrated DB stay consistent; v2 exports are
rejected by v2.1.1 ("unsupported import version: 2"); v1 exports from v2.1.1 merge as unchanged;
an up-to-date DB opens read-only (`file:…?mode=ro`); `--help` creates no DB.
Not run locally: golangci-lint (the local v2.5.0 is built with Go 1.25 and refuses a Go 1.26 module),
CodeQL, gosec, Docker build (Docker Hub blocked), macOS/Windows tests, `go vet` for Windows (Windows-only
modules not in the offline cache), the windows/arm64 release build (needs the `windows-11-arm` runner).

## Previous baseline (2026-09-23, uncommitted change set on top of df2555d)
With Go 1.26.0 + CGO: `gofmt -s -l .` clean, `go vet ./...` clean, `go test ./...` and
`go test -race ./...` pass (also under TZ America/Los_Angeles, America/Phoenix, Europe/London).
Coverage: root pkg 30.0%, `internal` 84.7% (84.3% total; was 80.4%). actionlint + shellcheck clean;
hadolint reports only the pre-existing DL3018 (unpinned `apk` versions).
Not run locally: golangci-lint (no go1.26-compatible build offline), CodeQL, gosec, Docker build
(Docker Hub blocked), macOS/Windows test runs, `go mod tidy` (module proxy blocked; `go.mod`/`go.sum`
unchanged and no new modules imported).
**GitHub, 2026-09-24:** on `e32fd8a` (pushed to `main`) CI passed on ubuntu/macos/windows (tidy check,
vet, golangci-lint v2.13.2, tests + coverage, native build smoke run), Docker (build + smoke tests) and
Security (CodeQL + gosec) passed. Release `v2.1.1` (tag on `e32fd8a`) was built and published by `cd.yml`;
gosec results appear in Code Scanning; the Codecov upload succeeds (maintainer-confirmed).

## Review status
Findings came from a first pass, an independent blind review, and an independent review of the
fix diff (incl. 27 + 11 single-line mutation checks). "Fixed" = implemented with a regression test
unless noted. Security items live in `cybersec.md`.

## Correctness notes

### High
- **N-001 Absolute deadline loses minutes — Fixed.** Layout `15:04` in code and test.
- **N-002 Release binaries cannot run — Fixed (pending first tagged release).** `cd.yml` builds
  natively per OS with CGO and smoke-runs each binary; CI builds and runs a native binary on every OS.
- **N-003 Import renumbers IDs, drops `CompletedAt`, duplicates on re-import — Fixed.** Numeric IDs
  (1..2^31-1) are kept, explicit-ID rows are inserted before auto-ID rows in one transaction,
  `completed_at` is exported/imported (fallback `updated_at`), and a merge that overwrites a task that
  stays completed keeps its original completion time. Re-importing the same file is a no-op.

### Medium
- **N-004 Import ignores deadline changes — Fixed** (`equalTask` compares deadlines).
- **N-005 DB created on any invocation — Fixed.** DB opens lazily; `--help`, `--version`, `help`,
  `completion` and flag errors create no file (required and grouped flag errors still do, N-033).
  Default location kept at `./munus.db` (maintainer decision); `MUNUS_DB_PATH` documented.
- **N-008 TUI hid deadlined tasks beyond 10 — Fixed** (all listed, soonest first; header no longer says "Top 10").
- **N-009 PgUp/PgDn selected an off-page task — Fixed** (cursor moves to first row of the new page).
- **N-010 Sticky list error — Fixed** (any key dismisses; successful reload clears; `c` with no
  selection is a no-op; a failed toggle is reverted in memory).
- **N-026 TUI `c`/`s` overwrite changes made elsewhere — Fixed in v2.2.0 (P-033).** `ToggleComplete` and
  `CycleStatus` send the list's in-memory copy to `UpdateTask`, which saves every column and replaces
  the tags. An edit made by another process while the TUI is open (e.g. `munus edit 3 --title New
  --tag urgent`) is silently reverted by the next `c` or `s` on that task (reproduced with a test on a
  file database). Now `Storage.SetTaskStatus` writes only the status columns; the TUI and `complete`
  use it (`TestListModelStatusKeysKeepConcurrentEdits`, `TestCompleteCmdWritesOnlyStatus`).

### Low
- **N-011 Cursor not clamped after reload — Fixed.**
- **N-012 Only `1M` worked — Fixed** (tokenising parser; `2M`, `12M 1d` work).
- **N-013 Duration overflow — Fixed** (bounds: months ≤ 1200, other units ≤ ~100 years).
- **N-014 Plan ≠ apply for `regenerate` — Fixed** (`PlanImport` uses `merge`; replace+regenerate honoured).
- **N-015 Import options unvalidated — Fixed** (rejected before reading/backup; example corrected).
- **N-016 `delete` success for missing IDs — Fixed** (checked before prompting; `DeleteTask` returns
  `ErrTaskNotFound`, which the TUI treats as already deleted). Prompt uses `Confirm` (`y`/`yes` any case, EOF = no).
- **N-017 GORM logs to stdout — Fixed** (silent logger; the writer is injected via `openDatabase`,
  `io.Discard` in production).
- **N-019 Backup naming/location — Fixed** (`os.UserHomeDir`, unique `CreateTemp` names).
- **N-021 Expanded state keyed by row — Fixed** (keyed by task ID).
- **N-022 Delete dialog + `i`/`x` — Fixed** (dialog is modal).
- **N-006 `main.version` not declared — Fixed** (`--version`, stamped by `cd.yml`).
- **N-018 Default export omits completed tasks — Fixed (P-015).** Export includes every task;
  `--pending-only` skips completed ones; `-i` is a deprecated no-op.
- **N-020 "Due today!" for tasks overdue < 24h — Fixed (P-017).** Calendar-day labels with an injected clock.
- **N-023 TUI replace import confirmable with Enter-Enter — Fixed (P-016).** Only `y` applies.
- **N-024 List created from the form has no window size — Fixed (P-018).**
- **N-025 Plan-2 review findings — Fixed** (12 findings from an independent review of the plan-2
  change set; see `plan.md`, "Implementation notes").
- **N-027 First open races between processes — Fixed in v2.2.1 (P-037).** GORM `AutoMigrate` and the
  check-then-create trigger installation are not atomic. Three `munus add` runs started together on a
  new database failed 10 of 120 times ("table `item_models` already exists", "trigger … already
  exists"); on a v2.1.1-style database without triggers, 5 of 120 parallel `list` runs failed. The
  failing command exits 1 without changing any data. Now a failed first migration attempt is retried
  inside a transaction that holds the write lock, and triggers use `CREATE TRIGGER IF NOT EXISTS`.
- **N-037 Concurrent writers fail with "database is locked" — Fixed in v2.2.1 (P-037).** Found while
  fixing N-027. Transactions started deferred, so a read-then-write transaction (import, and GORM's
  own write transactions) that could not upgrade its lock while another process wrote got
  SQLITE_BUSY at once instead of waiting: 6 parallel `munus import` processes on v2.2.0 failed 101 of
  150 times, mixed concurrent commands 25 of 200 (nothing is written by a failing command). The DSN
  now sets `_txlock=immediate`, so every transaction takes the write lock when it begins and others
  wait up to the busy timeout: 0 failures (`TestConcurrentImportProcessesAllApply`).
- **N-028 `add` accepts a whitespace-only title and description — Fixed in v2.2.0 (P-032).** The
  TUI form and `edit` reject them; P-023 promised the same validation.
- **N-029 `import --strict` accepts trailing data — Fixed in v2.2.0 (P-035).** `Decoder.Decode`
  read only the first JSON value, so `{…} {…}` or `{…}garbage` passed strict import. Both modes now
  reject anything but whitespace after the bundle.
- **N-030 Enter in the TUI form puts the cursor at the start of the next field — Fixed in v2.2.1
  (P-039).** Tab, ↓ and the Vim bindings put it at the end, so when editing a task, Enter then
  typing prepends to the existing description or deadline (reproduced with a model test).
- **N-031 Relative days and weeks ignore DST — Fixed in v2.2.1 (P-040; D-6).** `d` and `w` are fixed 24 h / 168 h,
  so `1d` entered just before a DST change lands an hour off the same clock time (and can show
  "Due today!" or "2 days left" where "Due tomorrow" is expected). `ParseRelativeTime` reads
  `time.Now()` itself, so this could not be tested deterministically. Now days and weeks are added
  with `AddDate`, the clock is passed in, and the tests use fixed clocks and zones.
- **N-032 Docker image has no time-zone data — Fixed in v2.2.1 (P-041; `docker.yml` checks it).** `alpine:3.24` ships no zoneinfo
  and the binary does not embed `time/tzdata`, so `-e TZ=…` falls back to UTC for deadline input and
  labels. Now the image installs pinned `tzdata`, the README documents `TZ`, and `docker.yml` checks it.

- **N-038 Smoke tests can fail with SIGPIPE — Fixed (plan 4 phase A, workflow patch).** `munus list
  | grep -q smoke` in steps with `shell: bash` (which adds `pipefail`) failed in 10 of 200 local runs:
  `grep -q` exits at the first match and munus, still writing, gets SIGPIPE. `cd.yml` and `ci.yml`
  now use `grep … > /dev/null`.
- **N-039 musl gives cgo threads a 128 KiB stack — Fixed (plan 4 phase A, P-047).** Go's cgo
  threads take `pthread_attr` defaults, which on musl are 128 KiB (glibc: 8 MiB from `RLIMIT_STACK`),
  so SQLite ran on small stacks in the Docker image since plan 2 and would have in the new static
  binaries. Both Dockerfile builds link with `-Wl,-z,stack-size=8388608`, which musl reads from
  `PT_GNU_STACK`; `cd.yml` and `docker.yml` check it.

### Info
- Fixed in plan 2: N-007 `ExportPlan.Doing` never populated (P-022); `?`/`h` toggle an unused
  `showHelp` (P-019); `import -f -` advertises stdin but fails (P-020); `ctrl+c`/`q` ignored at TUI
  import confirm stage (P-016).
- Fixed: unreachable `task == nil` check removed; `conn.DB()` error handled; DB closed on exit;
  merge placeholder IDs (`tsk_new_<n>`) are generated per merge and never collide with existing IDs
  (plan 2 removed the package-level counter).
- Test hygiene — Fixed: masked assertions (`TestParseDeadline`, `TestReplaceAll`,
  `TestTaskDeadlineCalculation`, `TestDeleteTaskCmd_NegativeID`), 23 silent `return`s → `t.Fatalf`,
  tests no longer write backups into the real home directory, DST-robust durations.
- Fixed in v2.2.1 (plan 3 phase B):
  - **N-033** Required and mutually exclusive flag errors (`add -t x` without `-d`, `list --pending
    --completed`) still create the database: cobra validates those flags after `PersistentPreRunE`.
    Now each command opens the database itself after validating its arguments and flag values (P-038).
  - **N-034** Runtime errors (e.g. `complete 999` → "task not found") printed the full usage text;
    now only argument and flag errors do (P-038, D-8).
  - **N-035** `export -f -` wrote a file named `-`; now it writes standard output and the TUI
    export refuses `-` (P-042).
  - **N-036** `ApplyImport` ignored `ImportConfig.DryRun`; `openDatabase` left the connection open
    when a migration failed; the TUI path prompt accepted only single-byte keys. All fixed (P-042;
    Alt+letter also no longer types into TUI text fields). Still open (deferred, needs a design):
    TUI import/export run synchronously in `Update`, so a slow path such as a FIFO hangs the
    interface.

## Behaviour changes to be aware of
- Export JSON v1 gains an optional `completed_at` field. Older Munus versions ignore it, but an
  older version importing a new export with `--strict` will reject the unknown field.
- Default import now strips control characters (and CRLF → LF) instead of storing them; `--strict`
  rejects them. `add`/the TUI form reject control characters and invalid UTF-8.
- Imported IDs above 2^31-1 are given new IDs.
- `delete` of an unknown ID now exits 1 with "task not found".
- Database open failures are reported as `Error: failed to initialize database: …` by cobra
  (previously a `log.Fatalf` line with a timestamp).
- Linux release binaries are statically linked (`sqlite_omit_load_extension,osusergo,netgo`).
- windows/arm64 was dropped in v2.1.1 (it never worked); plan 2 builds it again on the
  `windows-11-arm` runner with a pinned llvm-mingw toolchain.
- Plan 2: `export` includes completed tasks by default and writes schema v2 (`status`, `tags`), which
  v2.1.1 and older reject; `edit`, list filters, tags and the `doing` status are new; the TUI import
  confirm applies only on `y`; `complete --undo` reopens only completed tasks; `import --file -`
  reads stdin (`--mode replace` then needs `--yes`).
- Codecov v5 needs a `CODECOV_TOKEN` repository secret; without it uploads fail but CI does not.
- Plan 3 phase A (released in `v2.2.0`): import rejects files with more than 50,000 tasks,
  data after the export and `null` task entries; a blank title is imported as `(untitled)` (rejected
  by `--strict`); file IDs above 1,000,000,000 (was 2^31-1) get new IDs, while tasks already in the
  database keep theirs; bidi override/isolate characters count as control characters; `add` rejects
  blank titles and descriptions; `complete --undo` writes nothing for a task that is not done; an
  empty or truncated import reports "unexpected end of JSON input"; unknown stored statuses are
  repaired when the database is opened; `?` DSN and `file:` URI databases are created `0600`.
- Plan 3 phases B and C (released in `v2.2.1`): every transaction starts with `BEGIN IMMEDIATE`
  (concurrent writers wait instead of failing); argument and flag errors, including invalid flag
  values, print usage and never create the database, while runtime errors print only the error;
  relative `d`/`w` are calendar units; `export -f -` writes standard output; `ApplyImport` honours
  `DryRun`; Enter in the TUI form keeps the cursor at the end of the next field; the Docker image
  honours `TZ`; CI and releases use the latest Go 1.26 patch release and Security runs govulncheck.
- Plan 4 phase A (target `v2.2.2`, packaging only): release archives also contain `LICENSE`,
  `NOTICE` and `THIRD_PARTY_LICENSES` (extract them into a directory); Linux binaries are static musl
  builds from the Dockerfile (no glibc; no VCS stamp in `go version -m`); darwin/amd64 is built and
  smoke-tested on Intel macOS; archives carry build provenance attestations; the image has the
  license files in `/usr/share/licenses/munus/`; image and Linux binaries request 8 MiB thread
  stacks. `NOTICE` no longer lists module versions.

## Residual risks / open questions
(Plan 4 in `plan.md`, decided 2026-09-27, addresses the open ones below.)
- CI, Docker, Security and release (`v2.1.1`) workflows are validated on GitHub (2026-09-24); all
  of them were green again for `v2.2.0` (2026-09-27, maintainer-confirmed).
- Plan 4 phase A (uncommitted): darwin/amd64 now builds and smoke-runs on `macos-15-intel` (P-048);
  that runner image is supported until Fall 2027, so revisit the matrix before then. The new jobs
  (static Linux build via Docker on amd64/arm64, Debian 11/Alpine smoke runs, packaging with license
  files, provenance attestation, split release job) have not run on GitHub yet; attestations need the
  repository to stay public (or GitHub Enterprise Cloud).
- Merge import now reads, backs up and replaces tasks in one transaction (P-021); the plan shown
  before confirmation can still differ from the result if another process writes in between.
- Plan 2 (released in `v2.2.0`, including the first windows/arm64 build): the Alpine pins must be
  bumped when Alpine drops those package revisions (procedure in `maint.md`). v2 exports cannot be
  imported by v2.1.1 or older.
- Ctrl+C in CLI commands exits immediately (no graceful cancellation, see the P-029 deviation in
  `plan.md`). Final per D-17 (a), 2026-09-27: the process exits and SQLite rolls back an unfinished
  transaction, so a write is either complete or absent; no cancellation handling is planned.
- Plan 3 phases B–C: SQLite does not queue waiting writers fairly, so under heavy, sustained
  contention a writer can still exceed the 5 s busy timeout ("database is locked"); a user can raise
  it with `MUNUS_DB_PATH=…?_busy_timeout=…`. The `tzdata` pin must be bumped when Alpine 3.24 drops
  that package revision (procedure in `maint.md`).
- Plan 3 phase A: TUI `c`/`s` compute the next status from the list's copy, which may be stale
  (the write itself no longer overwrites other fields); `complete --undo` decides from a read made
  just before the write, so a task changed from done to `doing` in that window becomes `todo`.
  `databaseFilePath` skips DSNs with a custom (non-unix/win32) vfs, whose files sqlite may still
  create with the umask. A database file removed after a rejected DSN could in theory belong to a
  second process that opened the same file at that instant with a valid DSN.
- A database written by a pre-v2.1.1 binary after migration is kept consistent by triggers; any
  other external tool editing the database directly can still store tags that bypass validation
  (output is sanitised, SEC-010).
- The root package (`main.go`, `database_path.go`) has 25% statement coverage.
- Licensing (engineering notes, not legal advice; plan 4 phase A, P-046/P-047): archives and the
  image now carry the full texts of every compiled module, Go, SQLite and musl, and Linux binaries
  no longer contain glibc. Still not covered: Windows binaries statically link parts of the
  MinGW-w64 runtime and libgcc (runner gcc) or compiler-rt (llvm-mingw); most of that needs no
  notice (public domain, GCC runtime or LLVM exceptions), but some mingw-w64 CRT files (ZPL-2.1,
  BSD, gdtoa) do if they are linked in — check with a linker map (`-extldflags=-Wl,-Map=…`) and add
  what is needed. musl's `COPYRIGHT` points to per-file notices for some parts (TRE regex, parts of
  libm, crypt, Arm string functions); which of them end up in the binary was not reviewed. libgcc
  (GCC runtime exception) and fortify-headers (0BSD) in the Linux binaries need no notice. The
  image's Alpine base packages carry their own licenses and are not covered by
  `THIRD_PARTY_LICENSES`.
- The Linux release toolchain now comes from the floating `golang:1.26-alpine3.24` tag (pulled
  fresh; `go version` is logged) and images are pinned by tag, not digest; Dependabot does not track
  the `debian:11`/`alpine:3.24` smoke-test images.
- Existing DB files keep their permissions; only newly created ones are `0600`.
- Over-length text stored by pre-validation versions (via unchecked import) cannot be re-imported.
- `CONTRIBUTING.md` was empty in `e32fd8a`; plan 2 (P-025) drafts it from repository facts for
  maintainer review. `NOTICE` lists every third-party module compiled into the release binaries
  (all 29 in `go.mod`, three of them Windows-only), plus embedded SQLite, the Go standard library and
  (plan 4) musl; holders and licenses were read from each module's own license file (2026-09-24).
  Since plan 4 it names no versions, and `go test ./tools/licenses` fails when its module list or
  `THIRD_PARTY_LICENSES` no longer matches the compiled modules.

## Documentation drift (README) — resolved 2026-09-23
List flags, add examples, artifact names, Docker volume path/user, undocumented flags and
`MUNUS_DB_PATH`, CGO prerequisite, testing and structure sections. `AGENTS.md` referred to
`CONTRIBUTING.md ` with a trailing space; the maintainer fixed it in `97cf04b` (2026-09-27, P-058).
**Resolved (2026-09-27):** `intel/history.md` was removed in `48b4da2` while `AGENTS.md` still
requires it; at the maintainer's request a new `intel/history.md` was created (the previous record is in
git at `48b4da2^`).

## Local tooling notes
- The Cowork VM and cloud sandbox block `go.dev`/`proxy.golang.org`/Docker Hub; results above were
  produced by building Go 1.26.0 from GitHub source and resolving modules via git with a throwaway
  `-modfile` (repo `go.mod`/`go.sum` untouched).
- Plan 2: staticcheck, errcheck and ineffassign were built from GitHub sources and run on a copy of
  the tree (staticcheck does not accept `-modfile`); actionlint 1.7.12 is needed for the
  `windows-11-arm` runner label (1.7.7 rejects it).
- Plan 4: gosec was built from the commit `security.yml` pins (`deb54465…`, v2.29.0) without its AI
  autofix package (which alone pulls in the cloud SDKs); zig from PyPI (`ziglang`) stands in for a
  musl C compiler when no Docker daemon is available; `check-jsonschema` validates `dependabot.yml`
  and the workflows against the SchemaStore schemas.
