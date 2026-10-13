---
paths:
  - "pkg/ext-*.go"
---

# Input validation, deadlines, import and export

These files handle untrusted input. After any change here run `make fuzz` as well as
`make check`. Fuzz targets: `FuzzParseDeadline`, `FuzzSanitizeForTerminal`, `FuzzParseImportData`.

## Text and tags (`ext-text.go`)

- Limits are bytes, declared in `pkg/ui-form.go`: title `MaxTitleLength` (100), description
  `MaxDescriptionLength` (500). Validate through `validateTaskText` (length, UTF-8, control
  characters) and `isBlank`; do not add a second copy of the rules.
- Rejected on `add`, `edit`, the TUI form and `import --strict`: control characters (C0/C1, and
  the bidi controls U+202A–U+202E, U+2066–U+2069), invalid UTF-8, over-length text, blank titles
  (and blank descriptions outside import). Descriptions may contain line breaks and tabs.
- Default import does not reject: it strips control characters (CRLF → LF), shortens text to the
  limits at a rune boundary (`shortenToLimit`, counted as `Shortened`) and stores a then-blank
  title as `(untitled)`, so older exports and backups always restore.
- Tags: 1–32 letters, digits, `-` or `_`, stored lowercase, at most 10 per task (`normalizeTags`,
  `maxTagLength`, `maxTagsPerTask`).
- Anything written to a terminal goes through `sanitizeForTerminal`; error output through
  `terminalSafeWriter`.

## Deadlines (`ext-deadline.go`)

- Parse with an injected clock: `parseDeadlineAt(input, now)`; `ParseDeadline` only supplies
  `time.Now()`. Tests always pass a fixed `now` and location.
- `d`, `w` and `M` are calendar units added with `AddDate` (same clock time across DST); `h` and
  `m` are elapsed time added after them. Absolute input uses `deadlineInputLayout`
  (`YYYY-MM-DD HH:MM`) in the clock's location.
- Keep the bounds (`maxDeadlineMonths`, `maxRelativeDuration`) that stop overflow and unbounded
  work; parsing must never panic (`FuzzParseDeadline`).

## Import (`ext-export-import.go`)

- Order is fixed: options → size → decode → validation → storage. A rejected file never reaches
  storage.
- Limits: `maxImportFileSize` (32 MiB), `maxImportTasks` (50,000, enforced while
  `decodeExportBundle` streams), nothing but whitespace after the bundle, IDs from a file kept
  only up to `maxImportedTaskID` (1,000,000,000).
- IDs: existing tasks keep their IDs; `storedTaskID` keeps an ID in range or the canonical
  spelling of an existing task's ID; numeric IDs are canonicalised (`"01"` → `"1"`) before
  merging; everything else gets a new ID.
- Schema: exports write `exportSchemaVersion` (2); imports accept 1 and 2. Version 1 files get
  their status from `completed`, and merging one keeps existing tasks' tags and `doing` status.
  Exports carry an optional `completed_at`; import falls back to `updated_at`. A new schema
  version is a breaking change that needs approval and README and compatibility notes.
- `PlanImport` and `ApplyImport` share `mergeVersion`; a dry run must report exactly what the real
  import does. Keep them on the same logic.
- Import reads the current tasks, writes the optional backup and replaces the tasks inside one
  storage transaction (`ReplaceAllTasksFunc`); a cancelled context rolls it back.

## Export and backups

- Exports write a random temp file and rename it, `0600`, and refuse to overwrite the active
  database (compared through the file SQLite reports). `ExportToFile` checks the context once
  more before writing.
- Backups go to `~/.munus/backups` (directory `0700`, files `0600`) via `writeBackup`.
- Any user-visible change (flags, limits, output lines such as `Shortened: …`) goes into the
  README export/import sections in the same change.
