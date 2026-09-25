# Engineering Notes & Open Questions

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
  `completion` and flag errors create no file. Default location kept at `./munus.db` (maintainer
  decision); `MUNUS_DB_PATH` documented.
- **N-008 TUI hid deadlined tasks beyond 10 — Fixed** (all listed, soonest first; header no longer says "Top 10").
- **N-009 PgUp/PgDn selected an off-page task — Fixed** (cursor moves to first row of the new page).
- **N-010 Sticky list error — Fixed** (any key dismisses; successful reload clears; `c` with no
  selection is a no-op; a failed toggle is reverted in memory).

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

## Residual risks / open questions
- CI, Docker, Security and release (`v2.1.1`) workflows are validated on GitHub (2026-09-24).
  darwin/amd64 is built but not smoke-run in `cd.yml` (cross-compiled on the arm64 runner).
- darwin/amd64 is cross-compiled on the arm64 macOS runner and not smoke-run (no Rosetta assumption).
- Merge import now reads, backs up and replaces tasks in one transaction (P-021); the plan shown
  before confirmation can still differ from the result if another process writes in between.
- Plan 2 (uncommitted): the windows/arm64 build and the pinned Alpine packages are unverified until
  `cd.yml`/`docker.yml` run; the pins must be bumped when Alpine drops those package revisions
  (procedure in `maint.md`). v2 exports cannot be imported by v2.1.1 or older.
- Ctrl+C exits immediately (no graceful cancellation, see the P-029 deviation in `plan.md`).
- A database written by a pre-v2.1.1 binary after migration is kept consistent by triggers; any
  other external tool editing the database directly can still store tags that bypass validation
  (output is sanitised, SEC-010).
- The root package (`main.go`, `database_path.go`) has 25% statement coverage.
- Release archives (`cd.yml`) and the Docker image contain only the binary, not `LICENSE` or
  `NOTICE`; the MIT, BSD and Apache-2.0 licenses of the bundled modules ask for their notices to
  accompany binary distributions. Linux release binaries also link glibc statically; its LGPL
  terms for static linking have not been reviewed.
- Existing DB files keep their permissions; only newly created ones are `0600`.
- Over-length text stored by pre-validation versions (via unchecked import) cannot be re-imported.
- `CONTRIBUTING.md` was empty in `e32fd8a`; plan 2 (P-025) drafts it from repository facts for
  maintainer review. `NOTICE` lists every third-party module compiled into the release binaries
  (all 29 in `go.mod`, three of them Windows-only), plus embedded SQLite and the Go standard library;
  holders and licenses were read from each module's own license file (2026-09-24). Update it when
  `go.mod` changes.

## Documentation drift (README) — resolved 2026-09-23
List flags, add examples, artifact names, Docker volume path/user, undocumented flags and
`MUNUS_DB_PATH`, CGO prerequisite, testing and structure sections. `AGENTS.md` still refers to
`CONTRIBUTING.md ` with a trailing space.

## Local tooling notes
- The Cowork VM and cloud sandbox block `go.dev`/`proxy.golang.org`/Docker Hub; results above were
  produced by building Go 1.26.0 from GitHub source and resolving modules via git with a throwaway
  `-modfile` (repo `go.mod`/`go.sum` untouched).
- Plan 2: staticcheck, errcheck and ineffassign were built from GitHub sources and run on a copy of
  the tree (staticcheck does not accept `-modfile`); actionlint 1.7.12 is needed for the
  `windows-11-arm` runner label (1.7.7 rejects it).
