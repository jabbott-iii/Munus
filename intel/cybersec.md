# Security Requirements & Issue Tracker

## Requirements
- No credentials, tokens or production data in the repo or these documents.
- Do not weaken input validation (title ≤ 100, description ≤ 500 chars; positive task IDs;
  strict-mode JSON import), confirmation prompts on destructive actions, or CI security scans.
- Security-sensitive changes require human review (`AGENTS.md`).
- Tag names are validated (1–32 letters, digits, `-`, `_`; at most 10 per task) and, like all
  stored text, pass through `sanitizeForTerminal` before reaching the terminal. Error text printed by
  the root command goes through `terminalSafeWriter`; statuses shown or exported are always
  `todo`/`doing`/`done`.
- Control characters include the bidi embedding/override/isolate controls (U+202A–U+202E,
  U+2066–U+2069).
- Imports: at most 32 MiB and 50,000 tasks, no data after the bundle, blank titles rejected by
  `--strict`; IDs from a file are kept only up to 1,000,000,000, IDs of existing tasks always.
- Tools downloaded by workflows are pinned by version and verified by SHA-256 before use
  (llvm-mingw in `cd.yml`); actions stay pinned to commit SHAs.
- Release integrity (plan 4 phase A): archives and `checksums.txt` get a build provenance
  attestation from a job that cannot write to the repository (`contents: read`, `id-token: write`,
  `attestations: write`); only the tag-only publishing job has `contents: write`, and it has no OIDC
  token. Checkouts in release jobs do not persist credentials. The version string passed to the
  builds must match `^[0-9A-Za-z._+-]+$`. Linux release builds pull the Dockerfile's builder image
  fresh (latest Go 1.26 patch) and log `go version`, so the SEC-018 check still applies to them.
- Dependency updates (Dependabot) go through the same review and checks as any pull request; they
  are never merged automatically.

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
- **Status:** Closed
- **Affected component:** `internal/ext-export-import.go` (`readImportSource`, alert #4), `internal/database.go` (`createPrivateDatabaseFile`, alert #5)
- **Risk:** None (false positives). G304 flags files opened from a variable path, which matters when the path comes from someone other than the person running the program (for example a web request). Here the import path is the one the user passes to `--file` or types at the TUI prompt, and the database path is the user's own `MUNUS_DB_PATH` (default `./munus.db`); Munus runs with that user's permissions, so the user cannot reach anything they could not already open. Imports stay capped at 32 MiB and validated (SEC-005, SEC-006); the database file is created with `O_EXCL` and `0600`, so an existing file is never opened (SEC-001).
- **Required remediation:** Suppress G304 on those two calls only, with the reason in the source (`// #nosec G304 -- …`). `filepath.Clean`, which G304 accepts as sanitising, was rejected: it does not restrict the path, only silences the rule, and can change which file a path such as `link/../x` opens. `os.Root` does not fit because users may import from, or keep the database in, any directory.
- **Validation:** gosec v2.29.0 (the version pinned in `security.yml`) run locally with CI's arguments reports 0 issues and 3 suppressions, and its SARIF output is empty; `go vet` and all tests pass; Code Scanning marks alerts #4 and #5 fixed after the next Security run on `main`.
- **Resolution:** Closed 2026-09-27. In-source suppressions committed in `44c6452`; local gosec validation passed (2026-09-24) and the maintainer confirmed successful GitHub results for the Security run on `main` and Code Scanning alerts #4 and #5.

### SEC-013 — Terminal escape sequences from a stored status or a SQLite error message
- **Status:** Closed
- **Affected component:** `internal/logic-cli.go` (`PrintList` `-Status:` line), `internal/ui-form.go` (form error line), CLI error output (cobra's `Error:` line), `internal/database.go` (`migrateStatusAndTags` does not repair unknown statuses)
- **Risk:** Low. Same class as SEC-005/SEC-010. `munus list` prints the stored `status` without `sanitizeForTerminal`, and SQLite error text (for example a trigger's `RAISE(ABORT, …)` message) reaches the terminal raw through cobra's `Error:` line and the TUI form. A crafted database can therefore emit control sequences; the default database is `./munus.db` in the current directory, so running Munus inside a downloaded or cloned directory that contains one is enough. Verified 2026-09-27 with the HEAD binary: a status of `"\x1b]0;PWNED\x07"` (`munus list`) and a trigger raising the same text (`munus complete 1`) both put `033` in the output (`od -c`). An unknown stored status also makes `edit` of any other field fail with "invalid status". Related (display spoofing, not control sequences): Unicode format characters such as U+202E and U+200B pass `validateTaskText` and `sanitizeForTerminal`.
- **Required remediation:** Sanitise the status in `PrintList` (or print the status derived from `completed` when the stored one is unknown); have the migration repair statuses other than `todo`/`doing`/`done`; sanitise error text where it is printed (main: `SilenceErrors` and write `sanitizeForTerminal(err.Error(), true)` to stderr; the TUI form error line). Optionally treat Unicode format characters (category Cf) like control characters.
- **Validation:** Tests that store ESC/BEL in `status` and install a trigger raising ESC/BEL, asserting that `list` output, the `complete` error output and the form view contain no control characters; a migration test for an unknown status; `munus list | od -c` shows no `033`.
- **Resolution:** 2026-09-27 (plan 3 phase A, P-034): committed in `c9eeb97` and released in `v2.2.0` with green CI, Docker and Security runs (maintainer-confirmed). `itemStatus`/`taskStatusOf` return only `todo`/`doing`/`done`; opening a database repairs unknown statuses (writing only when needed); the root command writes errors through `terminalSafeWriter` (also used for the database-close error in `main`); the TUI form error line is sanitised; bidi controls are treated as control characters (D-7). Validated by `TestPrintListShowsOnlyKnownStatuses`, `TestNewDatabaseRepairsUnknownStatuses`, `TestRootCommandSanitizesErrorOutput`, `TestFormModelErrorIsSanitized`, `TestMarshalBundleExportsKnownStatuses`, `TestBidiControlsAreControlCharacters`, `TestTerminalSafeWriter`, mutation checks, and the binary: `list` and the trigger error of `complete` contain no ESC or BEL (the HEAD binary emitted one ESC).

### SEC-014 — Import accepts titles that become empty, so later exports and backups cannot be restored
- **Status:** Closed
- **Affected component:** `internal/ext-export-import.go` (`parseImportData`), `internal/ext-text.go` (`validateTaskText`), `internal/logic-cli.go` (`add`)
- **Risk:** Low–Medium (integrity and availability of backups). The "title is required" check runs before control characters are stripped, so a title such as `"\u0008"` or `"\r\n"` is stored as an empty string. Every later export and `--backup` file then contains an empty title, and import rejects such a file as a whole (`tasks[0].title is required`), including `--mode replace` restores. Verified 2026-09-27 with the HEAD binary; also found independently by fuzzing `parseImportData`. Related: `munus add` accepts a whitespace-only title and description, which the TUI and `edit` reject.
- **Required remediation:** Reject a title that is empty after stripping (and after trimming) in `parseImportData`, and give `add`, `edit`, the TUI form and import one shared non-empty check. Decide how files already containing an empty title should import (for example reject with the task index, or import with a placeholder title) so affected backups can still be restored.
- **Validation:** Tests: control-only and whitespace-only titles are rejected by import (default and `--strict`) and by `add`; a property test that every import-accepted file exports to a file that re-imports with `--strict`; the fuzz target used in the 2026-09-27 review passes without its empty-title exception.
- **Resolution:** 2026-09-27 (plan 3 phase A, P-032): committed in `c9eeb97` and released in `v2.2.0` with green CI, Docker and Security runs (maintainer-confirmed). Import checks for a blank title after stripping control characters: `--strict` rejects it, default import stores `(untitled)` (D-3), so files that already contain an empty title restore; `add` rejects blank titles and descriptions. Validated by `TestParseImportDataBlankTitles`, `TestBlankTitleImportKeepsExportsRestorable`, `TestReadImportFileMissingTitle`, `TestAddCmdRejectsBlankTitleOrDescription`, `FuzzParseImportData` (60 s, no empty-title exception) and the binary: after importing a control-only title, the export re-imports with `--strict`.

### SEC-015 — Tasks with IDs above the preservation cap are renumbered by every import (gap in SEC-009)
- **Status:** Closed
- **Affected component:** `internal/ext-export-import.go` (`itemFromTask`, `parseTaskID`, `replaceAllFunc`)
- **Risk:** Low–Medium (integrity). Import keeps IDs up to 2^31-1, which moves SQLite's AUTOINCREMENT counter to that value, so tasks created afterwards get IDs above the cap. Each later import passes the current tasks back through `itemFromTask`, which treats those IDs as placeholders and assigns new ones; re-importing an export then creates duplicates. Verified 2026-09-27 with the HEAD binary: import `{"id":"2147483647"}`, `add` → ID 2147483648; an empty merge import renumbers it to 2147483649 while reporting it unchanged; importing an export taken afterwards creates a duplicate (3 tasks instead of 2).
- **Required remediation:** Apply the cap only to IDs read from an import file and never remap the IDs of tasks already in the database (for example keep any positive ID present in the current tasks). Consider a lower cap so a crafted file cannot move the counter close to it.
- **Validation:** Test: import ID 2^31-1, add a task, run a merge import and re-import an export; IDs are unchanged and no duplicates appear. `TestImportHugeIDGetsNewDatabaseID` (SEC-009) still passes.
- **Resolution:** 2026-09-27 (plan 3 phase A, P-031): committed in `c9eeb97` and released in `v2.2.0` with green CI, Docker and Security runs (maintainer-confirmed). `storedTaskID` keeps the canonical ID of any task already in the database and caps IDs that are new to it at 1,000,000,000 (D-4); numeric file IDs are canonicalised before merging. Validated by `TestImportKeepsExistingIDsAboveCap`, `TestImportIDCapBoundary`, `TestStoredTaskID`, `TestImportHugeIDGetsNewDatabaseID` and the binary: import ID 1,000,000,000, `add`, empty merge import, export, re-import → IDs unchanged, `unchanged=2`, no duplicates.

### SEC-016 — The 32 MiB import cap does not bound the number of tasks
- **Status:** Closed
- **Affected component:** `internal/ext-export-import.go` (`parseImportData`, `mergeVersion`), `internal/ui-list.go` (TUI import runs inside `Update`)
- **Risk:** Low (local availability). A 32 MiB file of minimal tasks holds about 2.4 million tasks. Verified 2026-09-27 with the HEAD binary: `import --dry-run` peaked at about 2 GB RSS (11 s); the applied import took about 128 s at about 2.5 GB RSS and grew the database to 220 MB. In the TUI this work runs synchronously, so the interface freezes. A file received from someone else can exhaust memory on a small machine.
- **Required remediation:** Cap the number of tasks per import (value to be decided by the maintainer) and reject larger files before merging or writing; optionally lower the byte cap to match.
- **Validation:** Test that a file above the task cap is rejected with a clear error before any storage call; measure peak RSS for a maximal accepted file.
- **Resolution:** 2026-09-27 (plan 3 phase A, P-035): committed in `c9eeb97` and released in `v2.2.0` with green CI, Docker and Security runs (maintainer-confirmed). `decodeExportBundle` streams the task list and rejects more than 50,000 tasks (D-2) and any data after the bundle before merging or any storage call. Validated by `TestParseImportDataTaskCap`, `TestParseImportDataRejectsTrailingData`, `TestDecodeExportBundleMatchesUnmarshal` and measurement: the review's 2.4-million-task file is rejected in 0.1 s at 98 MB RSS (was 8.7 s / 1.96 GB for `--dry-run`); the largest accepted file (50,000 full-length tasks, 31 MB) imports in 3.6 s at 174 MB.

### SEC-017 — Database paths given as a URI or with parameters are created with default permissions (gap in SEC-001)
- **Status:** Closed
- **Affected component:** `internal/database.go` (`createPrivateDatabaseFile`)
- **Risk:** Low. `createPrivateDatabaseFile` skips `file:` URIs and paths containing `?`, so sqlite creates those files with the process umask. Verified 2026-09-27 with umask 022: `MUNUS_DB_PATH=file:/tmp/x/uri.db` and `…/param.db?_busy_timeout=5000` produce `-rw-r--r--`, while a plain path produces `-rw-------`.
- **Required remediation:** Tighten a database file this process created from a DSN to `0600` (for example compare existence before and after opening, using the file reported by `pragma_database_list`), or document that DSN paths keep the umask.
- **Validation:** Non-Windows tests that a new database opened through a `file:` URI and through a `?` DSN is `0600`, and that existing files keep their mode.
- **Resolution:** 2026-09-27 (plan 3 phase A, P-036): committed in `c9eeb97` and released in `v2.2.0` with green CI, Docker and Security runs (maintainer-confirmed). `createPrivateDatabaseFile` pre-creates `0600` files with `O_EXCL` for `?` DSNs and `file:` URIs too, at the path `databaseFilePath` derives with go-sqlite3's and sqlite's rules; a pre-created file is removed if the driver then rejects the DSN; existing files keep their mode. Validated by `TestDatabaseFilePath`, `TestNewDatabaseDSNFilesAreOwnerOnly` (also checks sqlite opened the pre-created file) and the binary with umask 022: `file:` and `?_busy_timeout` databases are `0600`, and an invalid DSN leaves no file.

### SEC-018 — CI/CD and repository hygiene hardening
- **Status:** Closed
- **Affected component:** `go.mod`, `.github/workflows/ci.yml`, `.github/workflows/cd.yml`, `.gitignore`, `.dockerignore`
- **Risk:** Info. (a) `setup-go` reads `go 1.26.0` from `go.mod` (there is no `toolchain` line), so CI and release binaries are built with Go 1.26.0. govulncheck v1.8.0 against a vulndb snapshot from 2026-09-24, run with `GOVERSION=go1.26.0`, lists 8 standard-library advisories in imported packages (`net/url`, `net`, `os`, `internal/syscall/unix`; all fixed by 1.26.6) and finds none reachable from Munus code; with Go 1.26.8 it finds none. (b) The release build job restores the setup-go cache (`cache: true`), which could feed a poisoned module or build cache into release binaries. (c) `.gitignore` does not exclude `munus-export-*.json` (the default export file, written to the current directory such as the repository root) or SQLite side files (`*.db-journal`, `*.db-wal`, `*.db-shm`), and `.dockerignore` lacks `*.db-wal`/`*.db-shm`, so task data could be committed or sent in a build context.
- **Required remediation:** Per plan 3 decision D-5 (a): set `go-version: "1.26.x"` with `check-latest: true` in `ci.yml` and `cd.yml` (no `go.mod` change); set `cache: false` for the release build job; add the ignore patterns (plan 3, P-044 and P-045). Workflow changes are delivered as a patch.
- **Validation:** CI and CD logs show the patched Go version; govulncheck reports no standard-library findings; `git check-ignore` matches `munus-export-20260927.json` and `munus.db-wal`.
- **Resolution:** Closed 2026-09-27 (plan 3 phase C, P-044 and P-045): committed as `df70f29` and released in `v2.2.1`; the maintainer confirmed all CI, CD, Docker and Security runs green, including the new govulncheck job. `ci.yml`, `cd.yml` and the govulncheck job use `go-version: "1.26.x"` with `check-latest: true` (D-5 a), so setup-go installs the latest Go 1.26 patch release; the `cd.yml` build job uses `cache: false`; `security.yml` runs `go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` (version-pinned, verified by the Go checksum database, `contents: read`); `.gitignore` ignores `munus-export-*.json`, `*.db-journal`, `*.db-wal`, `*.db-shm` and `.dockerignore` the SQLite side files and export files. Local validation: actionlint 1.7.12 with shellcheck clean, CRLF line endings kept, `git check-ignore` matches every pattern, govulncheck v1.8.0 clean with Go 1.26.8.
