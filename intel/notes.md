# Engineering Notes & Open Questions

## Current baseline (2026-09-23, uncommitted change set on top of df2555d)
With Go 1.26.0 + CGO: `gofmt -s -l .` clean, `go vet ./...` clean, `go test ./...` and
`go test -race ./...` pass (also under TZ America/Los_Angeles, America/Phoenix, Europe/London).
Coverage: root pkg 30.0%, `internal` 84.7% (84.3% total; was 80.4%). actionlint + shellcheck clean;
hadolint reports only the pre-existing DL3018 (unpinned `apk` versions).
Not run locally: golangci-lint (no go1.26-compatible build offline), CodeQL, gosec, Docker build
(Docker Hub blocked), macOS/Windows test runs, `go mod tidy` (module proxy blocked; `go.mod`/`go.sum`
unchanged and no new modules imported).

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
- **N-017 GORM logs to stdout — Fixed** (silent logger writing to `gormLogWriter`).
- **N-019 Backup naming/location — Fixed** (`os.UserHomeDir`, unique `CreateTemp` names).
- **N-021 Expanded state keyed by row — Fixed** (keyed by task ID).
- **N-022 Delete dialog + `i`/`x` — Fixed** (dialog is modal).
- **N-006 `main.version` not declared — Fixed** (`--version`, stamped by `cd.yml`).
- **N-018 Default export omits completed tasks — Open (not in plan).** Behaviour unchanged; README now states it.
- **N-020 "Due today!" for tasks overdue < 24h — Open (not in plan).**
- **N-023 TUI replace import confirmable with Enter-Enter — Open (not in plan).**
- **N-024 List created from the form has no window size — Open (not in plan).**

### Info
- Open (not in plan): N-007 `ExportPlan.Doing` never populated; `?`/`h` toggle an unused `showHelp`;
  `import -f -` advertises stdin but fails; `ctrl+c`/`q` ignored at TUI import confirm stage.
- Fixed: unreachable `task == nil` check removed; `conn.DB()` error handled; DB closed on exit;
  `newID()` now uses a counter (no clock-resolution collisions).
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
- windows/arm64 is no longer published (it never worked; would need an arm64 MinGW/zig toolchain).
- Codecov v5 needs a `CODECOV_TOKEN` repository secret; without it uploads fail but CI does not.

## Residual risks / open questions
- CI/CD, Docker and Code Scanning changes are lint-validated only until they run on GitHub.
- darwin/amd64 is cross-compiled on the arm64 macOS runner and not smoke-run (no Rosetta assumption).
- Merge import snapshots tasks outside the write transaction (pre-existing lost-update risk with
  concurrent writers).
- Existing DB files keep their permissions; only newly created ones are `0600`.
- Over-length text stored by pre-validation versions (via unchecked import) cannot be re-imported.
- `CONTRIBUTING.md` and `NOTICE` are currently empty in the working tree (maintainer's change, left
  untouched on request); `AGENTS.md` asks for `CONTRIBUTING.md` to be maintained and consistent with
  this folder (CGO prerequisite not added there).

## Documentation drift (README) — resolved 2026-09-23
List flags, add examples, artifact names, Docker volume path/user, undocumented flags and
`MUNUS_DB_PATH`, CGO prerequisite, testing and structure sections. `AGENTS.md` still refers to
`CONTRIBUTING.md ` with a trailing space.

## Local tooling notes
- The Cowork VM and cloud sandbox block `go.dev`/`proxy.golang.org`/Docker Hub; results above were
  produced by building Go 1.26.0 from GitHub source and resolving modules via git with a throwaway
  `-modfile` (repo `go.mod`/`go.sum` untouched).
