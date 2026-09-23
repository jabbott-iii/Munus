# History (append-only)

Do not edit or remove existing entries. Append new entries at the bottom.

## 2026-09-23 — Repository intelligence baseline
- Created `intel/` documents required by `AGENTS.md` (maint, map, cybersec, history, notes, plan).
- Baseline at `df2555d`: gofmt/vet clean, tests and race tests pass, 80.4% statement coverage.
- Recorded verified defects (deadline layout, CGO-less release builds, import ID renumbering)
  and security items SEC-001…SEC-004. No source code changed.

## 2026-09-23 — Second-pass verification
- Independent blind review plus re-reproduction against `df2555d` (no source changes).
- Confirmed all earlier findings; N-003 escalated (re-import duplicates tasks; `CompletedAt` lost).
- Added correctness notes N-008…N-024 (TUI list/paging/error state, month parsing, overflow,
  import option validation, CLI delete/logging) and security items SEC-005…SEC-008
  (terminal escape injection, import limits, export overwrite/symlink, parser memory DoS).
- `plan.md` re-prioritised (P-001…P-014).

## 2026-09-23 — Plan P-001…P-014 implemented (uncommitted)
- Deadline parsing: `15:04` layout, tokenising relative parser with bounds (fixes N-001, N-012, N-013, SEC-008).
- Import/export: stable task IDs, `completed_at` in export v1, deadline-aware merge, validated options,
  plan == apply, text validation/sanitising, 32 MiB cap, ID cap 2^31-1, atomic 0600 exports that refuse
  the active DB, unique 0600 backups in a 0700 dir (N-003/004/014/015/019, SEC-001/005/006/007/009).
- TUI list: all deadlined tasks, paging, error dismissal, cursor clamp, expand by ID, modal delete (N-008–011, N-021/022).
- CLI: lazy DB open, `--version`, `delete` not-found, shared `Confirm`, silent GORM logger (N-005/006/016/017).
- CI/CD/Docker: native per-OS CGO release builds (static Linux) with smoke tests, CI native build+run,
  SHA-pinned actions, gosec SARIF upload, codecov v5, non-root Docker image, `.dockerignore`.
- README rewritten against the code; test hygiene fixes; coverage 80.4% → 84.3%.
- Maintainer decisions recorded in `plan.md`. Independent review of the diff: findings fixed, or documented in `notes.md` where out of scope.
