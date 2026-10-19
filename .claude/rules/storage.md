---
paths:
  - "pkg/database.go"
  - "pkg/database_test.go"
  - "pkg/database_path.go"
  - "pkg/database_path_test.go"
---

# Storage and database location

## Opening and transactions

- `main.go` builds a deferred database (`NewDeferredDatabaseAt(LocateDatabase())`); the first
  open creates the directory (`ensurePrivateDir`), the file and the schema. Help, version and
  argument errors must never reach it.
- The DSN gets `_txlock=immediate` unless it sets `_txlock`, so every transaction starts with
  `BEGIN IMMEDIATE`; `withBusyTimeout` adds a 15 s busy timeout unless the DSN sets
  `_busy_timeout` or `_timeout`. Keep both, and keep the README "Configuration" row in step.
- Every multi-step write is a single transaction. An interrupted process must never leave partial
  data; that is why Ctrl+C keeps Go's default behaviour.
- GORM's logger stays silent (`io.Discard` via `openDatabase`). Errors are returned, never printed:
  stdout must stay clean for `export --stdout` and the TUI.

## Schema and migration

- Migration runs first without a transaction (an up-to-date database is only read) and, if that
  fails, again inside one. Every step must be idempotent and write only when something needs to
  change; triggers use `CREATE TRIGGER IF NOT EXISTS`.
- Schema changes are additive: new columns need defaults; never drop or rename columns.
- Older binaries (pre-1.0.0 builds up to v2.1.1) know nothing of `status` or tags.
  `consistencyTriggers` keep a migrated database consistent if an older binary writes to it; do
  not remove them.

## Data rules

- **Status** is `todo`, `doing` or `done`. `completed` is true exactly when the status is `done`;
  `completed_at` is set exactly when `completed` is true. `ItemModel.BeforeSave` enforces this;
  change status with `setStatus`. Opening a database repairs unknown stored statuses, and
  `itemStatus`/`taskStatusOf` only ever return the three known values.
- **Status-only changes** (`complete`, TUI `c`/`s`) go through
  `Storage.UpdateTaskStatus(ctx, id, next, now)`: it reads the stored status and writes
  `next(stored)` in one transaction, touching only `status`, `completed`, `completed_at` and
  `updated_at`. `next` runs inside the transaction and must not block.
- **Missing tasks:** `UpdateTask` and `DeleteTask` return `ErrTaskNotFound`; never re-create a task
  that disappeared.
- **IDs** are stable. The table uses `AUTOINCREMENT`; `ReplaceAllTasks` inserts explicit-ID rows
  before auto-ID rows in one transaction.
- **Tags** live in `tags` (unique lowercase names) and `task_tags`. Writes replace a task's links
  and prune unused tags; `loadTags` returns them sorted. Names come from `normalizeTags`.

## Files and location

- New database files are pre-created `0600` with `O_EXCL` for plain paths, `?` DSNs and `file:`
  URIs (`databaseFilePath` mirrors go-sqlite3's rules; a pre-created file is removed again if the
  driver rejects the DSN). Existing files are not chmod-ed. Never loosen any permission.
- `MUNUS_DB_PATH` wins when set and non-empty; the user owns that path. Otherwise the database is
  `munus.db` in the per-user data directory (`defaultDataDir`): `$XDG_DATA_HOME/munus` (absolute
  only) or `~/.local/share/munus`, `~/Library/Application Support/munus` on macOS,
  `%LOCALAPPDATA%\munus` on Windows.
- `resolveDatabaseLocation` has no side effects and takes its inputs as parameters; failures are
  carried in `DatabaseLocation.Err` and reported on first open with a hint to set `MUNUS_DB_PATH`.
- A legacy `./munus.db` (pre-1.0.0 builds up to v3.0.0) only produces a notice; it is never
  opened, moved or changed. Backups stay in `~/.munus/backups`.
- Any change here means updating README "Configuration" and "Database location" in the same change.
