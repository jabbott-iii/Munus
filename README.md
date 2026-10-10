# Munus

Munus is a terminal task manager for people who live in the shell. It offers a
scriptable CLI and an interactive terminal UI (TUI), and stores tasks in a local
SQLite database.

## Features:

- **Task Management**
  - Create tasks with title and description
  - Set deadlines for project timeline tracking, as an absolute time
    (`YYYY-MM-DD HH:MM`) or relative to now (`30m`, `2h`, `1d`, `1w`, `1M`, or
    combinations such as `2d 3h 30m`); days, weeks and months are calendar units, so `1d` is
    the same clock time tomorrow
  - Track status: to do, in progress (`doing`) or done
  - Tag tasks (for example `work`, `home`) and filter by tag
  - Edit a task's title, description, deadline, status or tags
  - Mark tasks as complete (and undo it)
  - Delete tasks at any time

- **Data Storage & Export**
  - SQLite database for persistent storage, kept in your per-user data directory so you see
    the same tasks wherever you run `munus`
  - Export tasks to versioned JSON (all tasks by default)
  - Import tasks from JSON files or standard input (merge or replace), with optional backups;
    task IDs, statuses and tags are preserved across export and import

- **Interactive TUI**
  - Terminal user interface for task browsing and management
  - Launches automatically when run without subcommands
  - Optional Vim-style keybindings (`--vim`)

## Use cases

- Keep a personal to-do list with deadlines in the terminal: `munus add -t "Pay invoice" -d "Vendor X" -n "3d"`.
- Review what is due next in the TUI, complete or delete tasks with a few keystrokes.
- Track work in progress and filter it: `munus edit 3 --status doing`, then `munus list --status doing --tag work`.
- Back up tasks to JSON and restore or move them to another machine with `munus export` / `munus import`.

## Install:

Download the archive for your platform from the project's GitHub Releases page,
optionally verify it against `checksums.txt`, extract it, and put the binary on
your `PATH`. Release archives are named `munus_<os>_<arch>`:

| Platform | Archive |
|---|---|
| Linux x86-64 | `munus_linux_amd64.tar.gz` |
| Linux ARM64 | `munus_linux_arm64.tar.gz` |
| macOS Apple Silicon | `munus_darwin_arm64.tar.gz` |
| macOS Intel | `munus_darwin_amd64.tar.gz` |
| Windows x86-64 | `munus_windows_amd64.zip` |
| Windows ARM64 | `munus_windows_arm64.zip` |

Each archive contains the binary (`munus_<os>_<arch>`, `.exe` on Windows) together with
`LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES` (the license texts of the third-party
software compiled into it). The Linux binaries are statically linked against musl, so they
do not depend on the system's C library.

Linux / macOS (example for Linux x86-64):
```
sha256sum --ignore-missing -c checksums.txt
```
```
mkdir munus && tar -xzf munus_linux_amd64.tar.gz -C munus
```
```
chmod +x munus/munus_linux_amd64
```
```
sudo mv munus/munus_linux_amd64 /usr/local/bin/munus
```
Windows:
```
Extract munus_windows_amd64.zip (or munus_windows_arm64.zip) and add the .exe to your PATH as munus.exe.
```

Optional: archives and `checksums.txt` of releases built by the current release workflow
(`v3.0.0` and later) carry a GitHub build provenance attestation, which the
[GitHub CLI](https://cli.github.com/) can verify:
```
gh attestation verify munus_linux_amd64.tar.gz --repo jabbott-iii/Munus
```

Releases after `v3.0.1` also publish a container image for Linux (amd64 and arm64) to GitHub
Packages; see [Docker](#docker).

### Build from source

Prerequisites:

- Go 1.26 or newer (see `go.mod`)
- A C compiler (gcc or clang; on Windows, MinGW-w64) — the SQLite driver
  (`mattn/go-sqlite3`) requires cgo, so builds must use `CGO_ENABLED=1`.
  A binary built with `CGO_ENABLED=0` compiles but cannot open its database.

```bash
CGO_ENABLED=1 go build -o munus .
```

The Linux release binaries come from the Dockerfile's `static` stage (fully static, built
with musl in Alpine); with Docker (BuildKit) you can build the same binary for your
machine's architecture:

```bash
docker build --target static --output type=local,dest=dist .
```

## Core CLI capabilities

Munus is organized into focused command groups:

- munus add — task creation and management
- munus edit — change an existing task
- munus list — display tasks, optionally filtered
- munus complete — mark tasks as finished (or unfinished)
- munus delete — remove tasks
- munus export — export tasks to JSON
- munus import — import tasks from JSON
- munus --version — print the version

### add

Title (`-t`) and description (`-d`) are required; the deadline (`-n`) and tags (`--tag`) are
optional. Titles are limited to 100 bytes and descriptions to 500 bytes (UTF-8, so non-ASCII text
fits fewer characters), and neither may be blank. Control characters (descriptions may contain
line breaks and tabs), bidirectional control characters and invalid UTF-8 are rejected. Tags are 1–32
letters, digits, `-` or `_`, stored lowercase, at most 10 per task. Relative deadlines count
from now: `d`, `w` and `M` are calendar days, weeks and months (the same clock time, also across a
daylight-saving change), `h` and `m` are elapsed hours and minutes.

An invalid argument or flag value is reported with the command's usage and never creates or
changes the database; other errors (for example an unknown task ID) print only the error.

- munus add --title "Title" --description "Description" — add a task with details
- munus add --title "Title" --description "Description" --deadline "2h" — create a task with a deadline

Examples:
- munus add --title "Feature Review" --description "Review new API endpoints"
- munus add --title "Bug Fix" --deadline "1d" --description "Fix login validation"
- munus add -t "Meeting" -d "Team sync" -n "2h"
- munus add -t "Launch" -d "Go live" -n "2026-11-16 14:30"
- munus add -t "Groceries" -d "Milk, eggs" --tag home --tag errands

### edit

Only the fields you pass change; the same validation as `add` applies.

- munus edit [task-id] --title "New title" (`-t`), --description "..." (`-d`), --deadline "2d" (`-n`)
- munus edit [task-id] --clear-deadline — remove the deadline
- munus edit [task-id] --status todo|doing|done (`-s`)
- munus edit [task-id] --tag work --untag home — add and remove tags

Examples:
- munus edit 3 --status doing
- munus edit 3 --deadline "2026-12-01 09:00" --tag urgent

### list

- munus list — show all tasks with their status (`TODO`, `DOING`, `OVERDUE`, `DONE`), ID, deadline and tags.
  Deadlines are shown in local time as `YYYY-MM-DD HH:MM` (the format `--deadline` accepts), followed for
  unfinished tasks that are overdue or at most three days away by a label such as `(Overdue)`,
  `(Overdue by 2 days)`, `(Due today!)`, `(Due tomorrow)` or `(3 days left)`; a task without a deadline
  shows `none`
- munus list --pending — only tasks that are not done (`--completed` shows only done tasks)
- munus list --overdue — only unfinished tasks past their deadline
- munus list --status todo|doing|done (`-s`) — only tasks with that status
- munus list --tag work — only tasks with that tag (repeat `--tag` to require several)

Filters combine: a task must match all of them.

### complete

- munus complete [task-id] — mark a task as complete (status `done`)
- munus complete [task-id] --undo — reopen a completed task (status `todo`); a task in progress keeps `doing`

Examples:
- munus complete 1
- munus complete 5 --undo

### delete

- munus delete [task-id] — remove a task after a `[y/N]` confirmation; an unknown ID is reported as an error

Examples:
- munus delete 1
- munus delete 3

### export

By default, export writes pretty-printed JSON (schema version 2, including status and tags) to
`munus-export-YYYYMMDD.json` in the current directory and includes every task, completed ones too.
Export refuses to overwrite the active database file.

- munus export --file tasks-backup.json — export to a specific file
- munus export --pending-only — skip completed tasks
- munus export --stdout — write JSON to standard output (`--file -` does the same)
- munus export --dry-run — show how many tasks would be exported, by status

`-i/--include-completed` is still accepted for older scripts but no longer changes anything.
Munus v2.1.1 and older cannot import version 2 files.

Examples:
- munus export --file my-tasks.json
- munus export --stdout > tasks.json

### import

Import reads a JSON export (schema version 1 or 2) of at most 32 MiB and 50,000 tasks; a file
with anything but whitespace after the export is rejected. Tasks already in the database keep their IDs, and numeric
task IDs from the file are kept up to 1,000,000,000 (a larger ID gets a new one). Version 1 files get
their status from `completed`, and merging one keeps the tags (and `doing` status) of tasks that
already exist. Control characters are removed from imported text, and a task whose title is then
blank is imported as `(untitled)`. Text longer than the limits (titles 100 bytes, descriptions 500
bytes), for example in exports of data saved by older versions, is shortened to fit at a character
boundary; the import plan and result report how many tasks in the file were shortened
(`Shortened: …`, `shortened=…`).

- munus import --file tasks-backup.json — merge tasks from a JSON file (default `--mode merge`)
- munus import --file - — read the export from standard input (`--mode replace` then needs `--yes`, except with `--dry-run`)
- munus import --file tasks-backup.json --skip-existing — keep local tasks when imported IDs collide
- munus import --file tasks-backup.json --on-conflict skip|overwrite|rename — choose how changed tasks with the same ID are handled (default `overwrite`)
- munus import --file tasks-backup.json --id-strategy regenerate — import every task as a new task with a new ID (default `preserve`)
- munus import --file tasks-backup.json --mode replace — replace all local tasks (asks for confirmation unless `--yes`)
- munus import --file tasks-backup.json --backup — write a backup of current tasks to `~/.munus/backups/` first
- munus import --file tasks-backup.json --dry-run — show the import plan without changing anything
- munus import --file tasks-backup.json --strict — reject unknown fields, unknown statuses, control characters, over-length text, blank titles and a `completed` flag that contradicts `status`

Examples:
- munus import --file my-tasks.json
- munus import --file my-tasks.json --skip-existing
- munus import --file my-tasks.json --mode replace --yes --backup
- munus export --stdout | munus import --file - --dry-run

### Interactive TUI

- Running munus with no subcommand launches the terminal UI for interactive task management.
- By default, Munus keeps the existing non-Vim behavior: the TUI opens the new-task form, `tab`/`shift+tab` and `↑`/`↓` move between fields or tasks where applicable, `ctrl+l` switches to the task list, and `ctrl+c` quits.
- In the task list: `e` expands a task, `c` toggles completion, `s` cycles the status (todo → doing → done), `u` edits the selected task, `d` opens the delete prompt and `y` confirms it, `n` creates a new task, `r` refreshes, `pgup`/`pgdown` (or `b`/`f`) change page, `x` exports and `i` imports.
- `F` cycles the list filter (all → pending → in progress → overdue → done) and `#` cycles a tag filter; the active filter is shown next to the title. `?` (or `h`) shows every key binding.
- In the edit form, `Enter` on the last field saves and `esc` returns to the list without saving. Clearing the deadline field removes the deadline.
- The import preview applies only when you press `y`; `n`, `esc` or `q` cancel.
- Exports, import previews and imports run in the background, so a slow file (for example on a
  network share) does not freeze the interface; while one runs, `esc` cancels it and `ctrl+c`
  quits. A cancelled import changes nothing; an import or export that finished anyway is reported
  in the status line.
- `c` and `s` decide on the task's current status in the database, so a change made by another
  `munus` process in the meantime is taken into account: `c` completes a task shown as open
  (nothing changes if it is already done) and reopens a task shown as done only if it is still
  done; `s` moves the current status one step on.
- Run `munus --vim` to enable Vim-style TUI behavior.
- With `--vim` enabled:
  - Munus opens the task list first so list navigation acts like the primary dashboard.
  - The list accepts `j`/`k` to move, `g`/`G` to jump to the top/bottom, `l` to expand the selected task, `d` to open delete confirmation, and either `y` or consecutive `dd` to confirm deletion. `n` or `Esc` cancels the delete prompt.
  - Forms start in insert mode so task titles, descriptions, and import/export paths still accept literal text input.
  - Press `Esc` in a form to switch to normal mode, then use `j`/`k` to move between fields, `h` or `Esc` to return to the list, `l` or `Enter` to advance/submit, and `i`/`a`/`o` to return to insert mode. `o` moves to the next field before re-entering insert mode.

## Configuration

| Setting | Default | Description |
|---|---|---|
| `MUNUS_DB_PATH` | `munus.db` in your data directory (see [Database location](#database-location)) | Path of the SQLite database file to use instead (the directory must exist), e.g. `export MUNUS_DB_PATH="$HOME/tasks/munus.db"`. `MUNUS_DB_PATH=munus.db` keeps one database per directory, as Munus v3.0.0 and older did. |
| `_busy_timeout` in `MUNUS_DB_PATH` | 15000 (15 s) | How long a command waits, in milliseconds, while another `munus` process is writing, before it fails with "database is locked", e.g. `export MUNUS_DB_PATH="$HOME/tasks/munus.db?_busy_timeout=30000"` (only with `MUNUS_DB_PATH`). |
| `--vim` | off | Enable Vim-style keybindings in the TUI. |

On Linux and macOS, new database, export and backup files are created readable
only by their owner (mode `0600`), and the data and backup directories are owner-only
(mode `0700`).
Import backups are written to `.munus/backups/` under your home directory.

### Database location

Unless `MUNUS_DB_PATH` is set, Munus keeps its database in a per-user data directory, which it
creates the first time a command opens the database (`--help` and `--version` create nothing):

| OS | Default database |
|---|---|
| Linux and other Unix systems | `$XDG_DATA_HOME/munus/munus.db`, or `~/.local/share/munus/munus.db` when `XDG_DATA_HOME` is not set (a relative `XDG_DATA_HOME` is ignored) |
| macOS | `~/Library/Application Support/munus/munus.db` |
| Windows | `%LOCALAPPDATA%\munus\munus.db` |

If the location cannot be determined (for example `HOME` is not set), commands that need the
database fail with an error asking you to set `MUNUS_DB_PATH`.

Munus v3.0.0 and older used `munus.db` in the current directory. When `MUNUS_DB_PATH` is not set
and a command finds a `munus.db` in the current directory, it prints a note on standard error and
uses the new location; the old file is not read or changed. The TUI shows the same note at the
top of its screens. To keep its tasks, either:

- keep using the old file: set `MUNUS_DB_PATH` to its path (or `MUNUS_DB_PATH=munus.db` to keep
  one database per directory), or
- copy its tasks into the new database; they get new IDs and existing tasks are kept:

  ```bash
  MUNUS_DB_PATH=./munus.db munus export --stdout | munus import --file - --id-strategy regenerate
  ```

  PowerShell:

  ```powershell
  $env:MUNUS_DB_PATH = ".\munus.db"; munus export --file old-tasks.json
  Remove-Item Env:MUNUS_DB_PATH; munus import --file old-tasks.json --id-strategy regenerate
  Remove-Item old-tasks.json
  ```

  Then delete or move the old `munus.db` so the note stops. If the new database is still empty,
  you can instead move the old file to the path shown in the note, replacing the empty database
  there; this keeps the task IDs.

## Docker

The image stores its database at `/app/data/munus.db` (`MUNUS_DB_PATH`) and runs as
an unprivileged user (UID 10001). Mount `/app/data` to keep your tasks. `LICENSE`,
`NOTICE` and `THIRD_PARTY_LICENSES` are in `/usr/share/licenses/munus/`.

### Pull from GitHub Packages

Releases after `v3.0.1` also publish the image to GitHub Packages (GitHub Container Registry)
for linux/amd64 and linux/arm64. Pull it by version, without the `v` of the release tag, or as
`latest`:
```bash
docker pull ghcr.io/jabbott-iii/munus:X.Y.Z
docker run --rm -it -v munus-data:/app/data ghcr.io/jabbott-iii/munus:latest
```

| Tag | Points to |
|---|---|
| `X.Y.Z` | That release (linux/amd64 and linux/arm64) |
| `X.Y` | The most recently published stable release of that minor version |
| `latest` | The most recently published stable release |
| `X.Y.Z-amd64`, `X.Y.Z-arm64` | One platform of that release |

Pre-releases (tags such as `v3.1.0-rc.1`) get only their own version tags (`3.1.0-rc.1`,
`3.1.0-rc.1-amd64` and `3.1.0-rc.1-arm64`) and never move `X.Y` or `latest`. In the image,
`munus --version` reports the release tag, for example `munus version v3.1.0`. The `v3.0.2`
image was published before this scheme, as `v3.0.2`, `v3.0.2-amd64` and `v3.0.2-arm64`.

Optional: the multi-platform tags (`X.Y.Z`, `X.Y`, `latest`) carry a build provenance
attestation, which the [GitHub CLI](https://cli.github.com/) can verify; the per-platform tags
have none of their own:
```bash
gh attestation verify oci://ghcr.io/jabbott-iii/munus:X.Y.Z --repo jabbott-iii/Munus
```

The examples below use a locally built `munus:latest`; to use the published image, replace it
with `ghcr.io/jabbott-iii/munus:<tag>`.

### Build
```bash
docker build -t munus:latest .
```

### Run (interactive)
```bash
docker run --rm -it munus:latest
```

### Persist data

With a named volume:
```bash
docker run --rm -it -v munus-data:/app/data munus:latest
```

With a host directory (created owner-only; run as your own user so it stays writable):
```bash
mkdir -p -m 700 ~/.munus
docker run --rm -it \
  --user "$(id -u):$(id -g)" \
  -v ~/.munus:/app/data \
  munus:latest
```

### Time zone

Deadlines are read and shown in the container's local time zone, which is UTC unless you set
`TZ` (the image includes the time-zone database):
```bash
docker run --rm -it -e TZ=America/Phoenix -v munus-data:/app/data munus:latest
```

### CLI usage
```bash
docker run --rm -it munus:latest --help
docker run --rm -it -v munus-data:/app/data munus:latest add --title "Example task" --description "Created from Docker"
docker run --rm -it -v munus-data:/app/data munus:latest list
```

## Testing and quality checks

Run from the repository root (cgo must be enabled, see [Build from source](#build-from-source)):

```bash
make check             # fmt-check, vet, test, race and golangci-lint (if installed)
make cover             # tests with a coverage summary
make fuzz              # run each fuzz target for FUZZTIME (default 30s)
make licenses          # regenerate THIRD_PARTY_LICENSES after dependency changes
```

`go test ./...` includes a check (`tools/licenses`) that fails when `THIRD_PARTY_LICENSES`
or the module list in `NOTICE` no longer matches the modules compiled into the binaries;
run `make licenses` (or `go run ./tools/licenses`) and update `NOTICE` to fix it.

The same checks without `make`:

```bash
gofmt -s -l .          # formatting (CONTRIBUTING requires gofmt -s -w . before PRs)
go vet ./...
go test ./...
go test -race ./...
```

CI (`.github/workflows/ci.yml`) runs a `go mod tidy` drift check, `go vet`, golangci-lint v2.13.2,
the tests with coverage and a native build smoke test on Linux, macOS and Windows; the formatting
check and the race detector run only locally (`make check`). The Docker
workflow also builds the static Linux binary for amd64 and arm64 and runs it on the runner,
on Debian 11 and on Alpine.

## Project structure

```
main.go                     entry point
pkg/                        package pkg: CLI commands, TUI models, storage (tasks, status, tags), database
                            location (MUNUS_DB_PATH or the per-user data directory), deadlines, import/export
tools/licenses/             generates and checks THIRD_PARTY_LICENSES
THIRD_PARTY_LICENSES        license texts of the third-party software in the binaries (generated)
.github/workflows/          CI, release (CD), Docker and security workflows
.github/dependabot.yml      weekly, grouped dependency updates
Dockerfile                  container image and static Linux release binary
intel/                      maintainer notes: architecture, security tracker, plans
```
