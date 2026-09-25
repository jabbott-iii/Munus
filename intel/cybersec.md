# Security Requirements & Issue Tracker

## Requirements
- No credentials, tokens or production data in the repo or these documents.
- Do not weaken input validation (title ≤ 100, description ≤ 500 chars; positive task IDs;
  strict-mode JSON import), confirmation prompts on destructive actions, or CI security scans.
- Security-sensitive changes require human review (`AGENTS.md`).
- Tag names are validated (1–32 letters, digits, `-`, `_`; at most 10 per task) and, like all
  stored text, pass through `sanitizeForTerminal` before reaching the terminal.
- Tools downloaded by workflows are pinned by version and verified by SHA-256 before use
  (llvm-mingw in `cd.yml`); actions stay pinned to commit SHAs.

## Issues

### SEC-001 — User data files created world-readable
- **Status:** Closed
- **Affected component:** `internal/ext-export-import.go` (`ExportToFile`, `writeBackup`), SQLite file created by `NewDatabase`
- **Risk:** Low. Exports and backups are written `0644` and the backup dir `0755`; the DB file is created with default umask (observed `0644`). On shared hosts other users can read task data. Re-verified 2026-09-23 (umask 022: DB/export `-rw-r--r--`, backup dir `drwxr-xr-x`).
- **Required remediation:** Write exports/backups with `0600`, backup dir `0700`; consider tightening DB file permissions after creation.
- **Validation:** Unit test asserting file mode on non-Windows; manual `ls -l` check.
- **Resolution:** 2026-09-23 (uncommitted change set). New DB files are pre-created `0600` (existing DB files untouched); exports use `os.CreateTemp` (`0600`) + rename; backups use `os.CreateTemp` in `~/.munus/backups` created/tightened to `0700`. Validated by `TestNewDatabaseCreatesOwnerOnlyFile`, `TestExportToFileOverwritesOtherFilesWithOwnerOnlyPermissions`, `TestWriteBackupIsUniqueAndOwnerOnly`, `TestWriteBackupTightensExistingDirectory` and a binary check (`ls -l` → `-rw-------`, `drwx------`). Windows ACLs not changed.

### SEC-002 — Third-party GitHub Actions not pinned (one tracks `@master`)
- **Status:** Closed
- **Affected component:** `.github/workflows/security.yml` (`securego/gosec@master`), all workflows (actions on mutable major tags)
- **Risk:** Medium. A compromised or changed upstream action runs with the workflow's token; `cd.yml` has `contents: write`.
- **Required remediation:** Pin third-party actions to full commit SHAs (at minimum `securego/gosec`, `softprops/action-gh-release`, `golangci/golangci-lint-action`, `codecov/codecov-action`).
- **Validation:** Workflow review; workflows still pass.
- **Resolution:** Closed 2026-09-24: validated by green CI, Docker and Security workflow runs on `e32fd8a` (push to `main`). Implementation: all third-party and GitHub actions pinned to full commit SHAs (tag in comment); release `contents: write` scoped to the release job only.

### SEC-003 — gosec results are discarded
- **Status:** Closed
- **Affected component:** `.github/workflows/security.yml`
- **Risk:** Low–Medium. gosec runs with `-no-fail` and writes `results.sarif`, but no step uploads it, so findings are never surfaced.
- **Required remediation:** Add `github/codeql-action/upload-sarif` for `results.sarif` (keeping `-no-fail` is acceptable if findings are triaged in Code Scanning).
- **Validation:** gosec alerts visible in the repo's Code Scanning tab.
- **Resolution:** Closed 2026-09-24: Security workflow (CodeQL + gosec + SARIF upload) passed on `e32fd8a` and the maintainer confirmed gosec results appear in Code Scanning. `github/codeql-action/upload-sarif` (pinned) uploads `results.sarif` with category `gosec`; gosec pinned to v2.29.0.

### SEC-004 — Container runs as root
- **Status:** Closed
- **Affected component:** `Dockerfile`
- **Risk:** Low. Interactive local tool, but running as root widens impact of any bug writing to mounted volumes. Also: no `.dockerignore`, so `.git/` and local `*.db` files are sent in the build context and copied into the builder stage (not the final image).
- **Required remediation:** Add a non-root user owning `/app/data` and `USER` directive; add `.dockerignore` excluding `.git`, `*.db`, `.idea`; update README volume guidance.
- **Validation:** `docker run --rm munus:latest --help` succeeds; `id` in container is non-root; persisted DB writable; build context excludes ignored paths.
- **Resolution:** Closed 2026-09-24: Docker workflow run 35920166266 on `e32fd8a` built the image and passed the `--help` and DB-on-named-volume smoke tests running as UID 10001 (bind-mount `--user` path not exercised in CI). Dockerfile runs as UID 10001 owning `/app/data`, `HOME=/app/data`, no `sqlite-libs`, no hard-coded `GOARCH`; `.dockerignore` added; README bind-mount guidance uses `--user`; `docker.yml` adds a DB-on-volume smoke test.

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

### SEC-010 — Terminal escape-sequence injection through tag names
- **Status:** Closed
- **Affected component:** `internal/logic-cli.go` (`PrintList`), `internal/ui-list.go` (`RenderTask`, `filterLabel`)
- **Risk:** Low. Found by the independent review of the plan-2 change set; never released. Tags are validated on save, but tag names already in the database (written by another tool editing the SQLite file) were printed raw by `munus list` and the TUI, so control sequences could reach the terminal (same class as SEC-005).
- **Required remediation:** Pass tag names through `sanitizeForTerminal` wherever they are written to the terminal.
- **Validation:** Tests store a tag containing ESC/BEL directly in the database and assert the CLI list output, the TUI row and the TUI filter label contain no control characters; mutation check removing each sanitiser call fails a test.
- **Resolution:** 2026-09-24 (uncommitted plan-2 change set). `PrintList`, `RenderTask` and `filterLabel` sanitise tag names. Validated by `TestPrintListSanitizesStoredTags` and `TestRenderTaskAndFilterLabelSanitizeTags`; the three single-line mutations removing the calls are each caught.

### SEC-011 — gosec G302 on the backup directory `chmod` (Code Scanning alert #6)
- **Status:** Closed
- **Affected component:** `internal/ext-export-import.go` (`writeBackup`)
- **Risk:** None (false positive). gosec rule G302 expects every `os.Chmod` mode to be `0600` or less, but this call sets `~/.munus/backups` to `0700`: a directory needs its execute bit to be entered, so `0700` is its owner-only mode and `0600` would break backups. The call is the SEC-001 remediation that tightens a backup directory created with broader permissions, so it must stay.
- **Required remediation:** Keep the `0700` chmod and suppress G302 on that statement only, with the reason in the source (`// #nosec G302 -- …`), instead of dismissing the alert or excluding G302 from the scan.
- **Validation:** gosec v2.29.0 (the version pinned in `security.yml`) run locally with CI's arguments reports no G302 and one suppression; `TestWriteBackupTightensExistingDirectory` still passes; Code Scanning marks alert #6 fixed after the next Security run on `main`.
- **Resolution:** Closed 2026-09-24. In-source suppression committed in `b2e354a`; local gosec validation passed and Code Scanning shows alert #6 as "closed as fixed" on `main`.

### SEC-012 — gosec G304 on user-chosen file paths (Code Scanning alerts #4 and #5)
- **Status:** In Progress
- **Affected component:** `internal/ext-export-import.go` (`readImportSource`, alert #4), `internal/database.go` (`createPrivateDatabaseFile`, alert #5)
- **Risk:** None (false positives). G304 flags files opened from a variable path, which matters when the path comes from someone other than the person running the program (for example a web request). Here the import path is the one the user passes to `--file` or types at the TUI prompt, and the database path is the user's own `MUNUS_DB_PATH` (default `./munus.db`); Munus runs with that user's permissions, so the user cannot reach anything they could not already open. Imports stay capped at 32 MiB and validated (SEC-005, SEC-006); the database file is created with `O_EXCL` and `0600`, so an existing file is never opened (SEC-001).
- **Required remediation:** Suppress G304 on those two calls only, with the reason in the source (`// #nosec G304 -- …`). `filepath.Clean`, which G304 accepts as sanitising, was rejected: it does not restrict the path, only silences the rule, and can change which file a path such as `link/../x` opens. `os.Root` does not fit because users may import from, or keep the database in, any directory.
- **Validation:** gosec v2.29.0 (the version pinned in `security.yml`) run locally with CI's arguments reports 0 issues and 3 suppressions, and its SARIF output is empty; `go vet` and all tests pass; Code Scanning marks alerts #4 and #5 fixed after the next Security run on `main`.
- **Resolution:** 2026-09-24 (uncommitted). In-source suppressions added; local validation done. Close once alerts #4 and #5 show as fixed on GitHub.
