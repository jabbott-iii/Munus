# Security Requirements & Issue Tracker

## Requirements
- No credentials, tokens or production data in the repo or these documents.
- Do not weaken input validation (title ≤ 100, description ≤ 500 chars; positive task IDs;
  strict-mode JSON import), confirmation prompts on destructive actions, or CI security scans.
- Security-sensitive changes require human review (`AGENTS.md`).

## Issues

### SEC-001 — User data files created world-readable
- **Status:** Closed
- **Affected component:** `internal/ext-export-import.go` (`ExportToFile`, `writeBackup`), SQLite file created by `NewDatabase`
- **Risk:** Low. Exports and backups are written `0644` and the backup dir `0755`; the DB file is created with default umask (observed `0644`). On shared hosts other users can read task data. Re-verified 2026-09-23 (umask 022: DB/export `-rw-r--r--`, backup dir `drwxr-xr-x`).
- **Required remediation:** Write exports/backups with `0600`, backup dir `0700`; consider tightening DB file permissions after creation.
- **Validation:** Unit test asserting file mode on non-Windows; manual `ls -l` check.
- **Resolution:** 2026-09-23 (uncommitted change set). New DB files are pre-created `0600` (existing DB files untouched); exports use `os.CreateTemp` (`0600`) + rename; backups use `os.CreateTemp` in `~/.munus/backups` created/tightened to `0700`. Validated by `TestNewDatabaseCreatesOwnerOnlyFile`, `TestExportToFileOverwritesOtherFilesWithOwnerOnlyPermissions`, `TestWriteBackupIsUniqueAndOwnerOnly`, `TestWriteBackupTightensExistingDirectory` and a binary check (`ls -l` → `-rw-------`, `drwx------`). Windows ACLs not changed.

### SEC-002 — Third-party GitHub Actions not pinned (one tracks `@master`)
- **Status:** In Progress
- **Affected component:** `.github/workflows/security.yml` (`securego/gosec@master`), all workflows (actions on mutable major tags)
- **Risk:** Medium. A compromised or changed upstream action runs with the workflow's token; `cd.yml` has `contents: write`.
- **Required remediation:** Pin third-party actions to full commit SHAs (at minimum `securego/gosec`, `softprops/action-gh-release`, `golangci/golangci-lint-action`, `codecov/codecov-action`).
- **Validation:** Workflow review; workflows still pass.
- **Resolution:** All third-party and GitHub actions pinned to full commit SHAs (tag in comment); release `contents: write` scoped to the release job only. Pending validation: workflows must run green on GitHub (actionlint + shellcheck pass locally).

### SEC-003 — gosec results are discarded
- **Status:** In Progress
- **Affected component:** `.github/workflows/security.yml`
- **Risk:** Low–Medium. gosec runs with `-no-fail` and writes `results.sarif`, but no step uploads it, so findings are never surfaced.
- **Required remediation:** Add `github/codeql-action/upload-sarif` for `results.sarif` (keeping `-no-fail` is acceptable if findings are triaged in Code Scanning).
- **Validation:** gosec alerts visible in the repo's Code Scanning tab.
- **Resolution:** `github/codeql-action/upload-sarif` (pinned) uploads `results.sarif` with category `gosec`; gosec pinned to v2.29.0. Pending validation: gosec alerts visible in Code Scanning after a GitHub run.

### SEC-004 — Container runs as root
- **Status:** In Progress
- **Affected component:** `Dockerfile`
- **Risk:** Low. Interactive local tool, but running as root widens impact of any bug writing to mounted volumes. Also: no `.dockerignore`, so `.git/` and local `*.db` files are sent in the build context and copied into the builder stage (not the final image).
- **Required remediation:** Add a non-root user owning `/app/data` and `USER` directive; add `.dockerignore` excluding `.git`, `*.db`, `.idea`; update README volume guidance.
- **Validation:** `docker run --rm munus:latest --help` succeeds; `id` in container is non-root; persisted DB writable; build context excludes ignored paths.
- **Resolution:** Dockerfile runs as UID 10001 owning `/app/data`, `HOME=/app/data`, no `sqlite-libs`, no hard-coded `GOARCH`; `.dockerignore` added; README bind-mount guidance uses `--user`; `docker.yml` adds a DB-on-volume smoke test. Pending validation: image build/run (Docker Hub is blocked from the analysis sandbox; hadolint shows only the pre-existing DL3018).

### SEC-005 — Terminal escape-sequence injection from imported task text
- **Status:** Closed
- **Affected component:** `internal/logic-cli.go` (`PrintList`), `internal/ui-list.go` (`RenderTask`, delete dialog), `internal/ext-export-import.go` (`readImportFile`)
- **Risk:** Low–Medium. Titles/descriptions from an imported JSON file are stored and later written raw to the terminal. A crafted file shared by someone else can emit control sequences (retitle terminal, clear screen, OSC 52 clipboard writes on supporting terminals). Verified: `\x1b]0;PWNED\x07` round-trips byte-for-byte through `import` → `list`.
- **Required remediation:** Reject or strip C0/C1 control characters (except perhaps `\n`/`\t` in descriptions) on import and on `add`/form input, and/or sanitise at render time.
- **Validation:** Unit test importing a title with ESC/BEL is rejected or stored sanitised; `munus list | od -c` shows no `033`.
- **Resolution:** 2026-09-23. `ValidateTaskText` rejects control characters and invalid UTF-8 on `add`, the TUI form and `import --strict`; default import strips them; all terminal output of task text and TUI error text goes through `sanitizeForTerminal`; duplicate-ID import errors are quoted. Validated by `TestValidateTaskText`, `TestReadImportFileHandlesUnsafeText`, `TestPrintListSanitizesStoredText`, `TestRenderTaskSanitizesStoredText`, `TestDeleteDialogSanitizesTitle`, `TestImportDuplicateIDErrorIsQuoted`, and a binary check (`munus list | od -c` shows no `033` even for a legacy row containing ESC).

### SEC-006 — Import bypasses input limits and has no size cap
- **Status:** Closed
- **Affected component:** `internal/ext-export-import.go` (`readImportFile`)
- **Risk:** Low. CLI/TUI enforce title ≤ 100 and description ≤ 500, but import only checks non-empty title (verified: 5000-char title stored). `os.ReadFile` reads an arbitrarily large file into memory.
- **Required remediation:** Apply `MaxTitleLength`/`MaxDescriptionLength` during import validation; cap import file size (e.g. via `io.LimitReader`).
- **Validation:** Tests for over-length fields and oversized file both fail with clear errors.
- **Resolution:** 2026-09-23. Import enforces `MaxTitleLength`/`MaxDescriptionLength` and a 32 MiB read cap (`io.LimitReader`). Validated by `TestReadImportFileHandlesUnsafeText` (over-length cases) and `TestReadImportFileRejectsOversizedFile`.

### SEC-007 — Export overwrites arbitrary files and follows a symlinked temp file
- **Status:** Closed
- **Affected component:** `internal/ext-export-import.go` (`ExportToFile`), TUI export overlay
- **Risk:** Low. Export silently replaces any existing file, including the live DB (verified: `export -f $DB` → next run "file is not a database"). It writes to a predictable `<path>.tmp` with `os.WriteFile`, which follows a pre-existing symlink (verified: symlink target overwritten, then symlink renamed into place). Relevant in shared writable directories.
- **Required remediation:** Create the temp file with `os.CreateTemp` in the target directory (0600), then rename; refuse the active DB path. *(Amended 2026-09-23 by maintainer decision: overwriting other existing files remains allowed, so the originally proposed `--force` requirement is not adopted.)*
- **Validation:** Tests: pre-existing `.tmp` symlink not followed; exporting onto DB path rejected; other files still overwritten with `0600`.
- **Resolution:** 2026-09-23. Per maintainer decision, export still overwrites ordinary files but refuses the active database (compared by `os.SameFile` against the file sqlite reports in `pragma_database_list`, so URI/parameterised DSNs are covered) and writes via an exclusive random temp file + rename (no symlink following). Validated by `TestExportToFileRefusesActiveDatabase`, `TestExportRefusesDatabaseOpenedWithParameters`, `TestExportToFileDoesNotFollowPlantedTmpSymlink` and a binary check.

### SEC-008 — Unbounded deadline input can exhaust memory
- **Status:** Closed
- **Affected component:** `internal/ext-deadline.go` (`ParseRelativeTime`)
- **Risk:** Low (local, self-inflicted; import does not parse deadline strings). A month count like `99999999999M` drives a loop that appends one byte per month before validation (independent review observed `fatal error: out of memory` under a 3 GB ulimit). Related overflow: large unit values wrap `time.Duration` (N-013).
- **Required remediation:** Bound numeric values (e.g. months ≤ 1200, total duration ≤ ~100 years) before building strings or multiplying; replace the string-length check.
- **Validation:** Tests for huge month/day values return an error promptly.
- **Resolution:** 2026-09-23. `ParseRelativeTime` bounds months (≤ 1200) and other units (≤ ~100 years) before any allocation or multiplication; `ParseTimeUnit` rejects overflow. Validated by `TestParseRelativeTime` (`99999999999M`, `1201M`, `213504d`, cumulative limits) and `TestParseTimeUnit`.

### SEC-009 — Imported task IDs could exhaust the AUTOINCREMENT counter
- **Status:** Closed
- **Affected component:** `internal/ext-export-import.go` (`parseTaskID`), `Database.ReplaceAllTasks`
- **Risk:** Medium (introduced and caught during review of the ID-preservation fix, never released). Preserving an imported ID such as `9223372036854775807` would push SQLite's `sqlite_sequence` to its maximum so every later insert fails with "database or disk is full", even after the task is deleted.
- **Required remediation:** Only preserve imported numeric IDs within a safe range; assign new IDs otherwise.
- **Validation:** Test that a file with IDs ≥ 2^31 imports with new IDs and later inserts still succeed.
- **Resolution:** 2026-09-23. IDs are preserved only in `1..2^31-1`; larger IDs are treated as placeholders. Validated by `TestImportHugeIDGetsNewDatabaseID` (mutation-checked).
