# History (append-only)

Do not edit or remove existing entries. Append new entries at the bottom.

This file was recreated on 2026-09-27. The previous history file (entries 2026-09-23 to 2026-09-24,
covering the intel baseline, plan 1, release `v2.1.1` and the plan 2 implementation) was removed in
`48b4da2` and is still available with `git show 48b4da2^:intel/history.md`. The first entries below are
backfilled from `git log` for the changes made after that file's last entry.

## 2026-09-24 — Plan 2 committed and pushed (backfilled from git)
- `68d60fa`: plan 2 (P-015…P-030) committed: status + tags model with migration and triggers,
  `munus edit`, list filters, export schema v2, stdin import, TUI edit/status/help, context propagation,
  `golang.md` conformance, Makefile dev targets, `CONTRIBUTING.md`, windows/arm64 release job, pinned
  Alpine packages.
- `2e79130`: Windows test failure fixed (`internal/database_test.go`); `57e42fb`: README image removed;
  `61ea890`: `NOTICE` updated with third-party license notices.
- `b2e354a`, `44c6452`: in-source gosec suppressions with documented reasons for G302 (SEC-011,
  Code Scanning alert #6) and G304 (SEC-012, alerts #4 and #5). `main` on GitHub at `44c6452`.

## 2026-09-26 — Previous history file removed (backfilled from git)
- `48b4da2` deleted `intel/history.md` (local commit, not yet pushed at the 2026-09-27 review).

## 2026-09-27 — Plan 2 and SEC-012 closed
- Maintainer marked plan 2 phases A, B and C done; `plan.md` statuses updated (P-029 deviation
  recorded as accepted; the first windows/arm64 zip for P-027 is produced by the `v2.2.0` tag).
- SEC-012 closed after the maintainer confirmed successful GitHub results.

## 2026-09-27 — Test and security review of `48b4da2`
- Validation with Go 1.26.8 and `go.sum`-verified modules: gofmt, `go vet` (linux, windows, darwin),
  `go mod tidy`, tests (87.7% coverage), race ×3 shuffled, six time zones, staticcheck and govulncheck
  clean; fuzzing of deadline parsing, terminal sanitising and import.
- An independent blind review plus reproduction with the HEAD binary recorded SEC-013…SEC-018 in
  `cybersec.md` and N-026…N-036 in `notes.md` (all Open). No source code changed.

## 2026-09-27 — History file recreated and plan 3 drafted
- New `intel/history.md` (this file) at the maintainer's request, resolving the `AGENTS.md` drift noted
  in `notes.md`.
- Plan 3 (P-031…P-045, proposed target `v2.2.1`) drafted in `plan.md` from the review findings, with
  eight maintainer decisions (D-1…D-8) open.

## 2026-09-27 — Plan 3 decided and phase A implemented (uncommitted)
- Maintainer decisions D-1…D-8 recorded in `plan.md`: `v2.2.0` waits for plan 3 phase A; 50,000-task
  import cap; blank titles import as `(untitled)`; existing IDs always kept and the file-ID cap lowered
  to 1,000,000,000; CI toolchain via setup-go `1.26.x` + `check-latest`; calendar `d`/`w`; bidi controls
  treated as control characters; usage only for argument/flag errors.
- Phase A (P-031…P-036) and P-043 implemented with regression tests: stable IDs above the cap,
  non-empty titles, `Storage.SetTaskStatus` for status-only changes, sanitised status and error
  output, streaming import decode with the task cap and trailing-data rejection, owner-only DSN
  database files, three fuzz targets and `make fuzz`. SEC-013…SEC-017 closed pending maintainer
  review and CI; N-026, N-028 and N-029 fixed.
- An independent review of the change set found no High issues; its findings were fixed. README,
  `CONTRIBUTING.md`, `maint.md`, `map.md`, `notes.md` and `cybersec.md` updated.

## 2026-09-27 — v2.2.0 released
- The plan 3 phase A change set was committed as `c9eeb97` (with the `make fuzz` Makefile patch
  applied) and the new history file as `ad5c231`; CI, Docker and Security workflows passed.
- Release `v2.2.0` (tag on `ad5c231`) was built, smoke-tested and published by `cd.yml`, including the
  first windows/arm64 zip (P-027). Plan 2 and plan 3 phase A are released; phases B and C target
  `v2.2.1`.

## 2026-09-27 — Plan 3 phases B and C implemented (uncommitted)
- P-037…P-042, P-044 and P-045 implemented with regression tests: `BEGIN IMMEDIATE` for every
  transaction and a locked migration retry (fixes N-027 and the newly found N-037, concurrent writers
  failing with "database is locked"); each command opens the database only after validating its
  arguments and flags (usage only for those errors); Enter keeps the form cursor at the end;
  calendar `d`/`w` with an injected clock; pinned `tzdata` in the Docker image; `export -f -` to
  standard output, `ApplyImport` dry runs, connection closed on failed setup, rune-based TUI path
  input; CI/CD on the latest Go 1.26 patch release, no release build cache, a govulncheck job, and
  new ignore rules.
- An independent review found no High or Medium issues; its findings were fixed. SEC-018 is In
  Progress until the workflows run on GitHub; N-027 and N-030…N-037 fixed (FIFO paths in the TUI
  still block the interface).

