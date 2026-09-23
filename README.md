<img width="812" height="542" alt="munusv2" src="https://github.com/user-attachments/assets/3ec01b8f-bfde-4fae-a114-78f50171dcf7" />

# Munus

Munus is a terminal task manager for people who live in the shell. It offers a
scriptable CLI and an interactive terminal UI (TUI), and stores tasks in a local
SQLite database.

## Features:

- **Task Management**
  - Create tasks with title and description
  - Set deadlines for project timeline tracking, as an absolute time
    (`YYYY-MM-DD HH:MM`) or relative to now (`30m`, `2h`, `1d`, `1w`, `1M`, or
    combinations such as `2d 3h 30m`)
  - Mark tasks as complete (and undo it)
  - Delete tasks at any time

- **Data Storage & Export**
  - SQLite database for persistent storage
  - Export tasks to versioned JSON
  - Import tasks from JSON files (merge or replace), with optional backups;
    task IDs are preserved across export and import

- **Interactive TUI**
  - Terminal user interface for task browsing and management
  - Launches automatically when run without subcommands
  - Optional Vim-style keybindings (`--vim`)

## Use cases

- Keep a personal to-do list with deadlines in the terminal: `munus add -t "Pay invoice" -d "Vendor X" -n "3d"`.
- Review what is due next in the TUI, complete or delete tasks with a few keystrokes.
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

Linux / macOS (example for Linux x86-64):
```
sha256sum --ignore-missing -c checksums.txt
```
```
tar -xzf munus_linux_amd64.tar.gz
```
```
chmod +x munus_linux_amd64
```
```
sudo mv munus_linux_amd64 /usr/local/bin/munus
```
Windows:
```
Extract munus_windows_amd64.zip and add munus_windows_amd64.exe to your PATH as munus.exe.
```

### Build from source

Prerequisites:

- Go 1.26 or newer (see `go.mod`)
- A C compiler (gcc or clang; on Windows, MinGW-w64) — the SQLite driver
  (`mattn/go-sqlite3`) requires cgo, so builds must use `CGO_ENABLED=1`.
  A binary built with `CGO_ENABLED=0` compiles but cannot open its database.

```bash
CGO_ENABLED=1 go build -o munus .
```

## Core CLI capabilities

Munus is organized into focused command groups:

- munus add — task creation and management
- munus list — display all tasks
- munus complete — mark tasks as finished (or unfinished)
- munus delete — remove tasks
- munus export — export tasks to JSON
- munus import — import tasks from JSON
- munus --version — print the version

### add

Title (`-t`) and description (`-d`) are required; the deadline (`-n`) is optional.
Titles are limited to 100 characters and descriptions to 500; control characters
are rejected.

- munus add --title "Title" --description "Description" — add a task with details
- munus add --title "Title" --description "Description" --deadline "2h" — create a task with a deadline

Examples:
- munus add --title "Feature Review" --description "Review new API endpoints"
- munus add --title "Bug Fix" --deadline "1d" --description "Fix login validation"
- munus add -t "Meeting" -d "Team sync" -n "2h"
- munus add -t "Launch" -d "Go live" -n "2026-11-16 14:30"

### list

- munus list — show all tasks with their status (`TODO`, `OVERDUE`, `DONE`), ID and deadline

### complete

- munus complete [task-id] — mark a task as complete
- munus complete [task-id] --undo — mark a task as incomplete

Examples:
- munus complete 1
- munus complete 5 --undo

### delete

- munus delete [task-id] — remove a task after a `[y/N]` confirmation; an unknown ID is reported as an error

Examples:
- munus delete 1
- munus delete 3

### export

By default, export writes pretty-printed JSON to `munus-export-YYYYMMDD.json` in the
current directory and skips completed tasks. Export refuses to overwrite the active
database file.

- munus export --file tasks-backup.json — export to a specific file
- munus export --include-completed — include completed tasks (`-i`)
- munus export --stdout — write JSON to standard output
- munus export --dry-run — show how many tasks would be exported

Examples:
- munus export --file my-tasks.json --include-completed
- munus export --stdout > tasks.json

### import

Import reads a JSON export (schema version 1). Numeric task IDs from the file are kept.

- munus import --file tasks-backup.json — merge tasks from a JSON file (default `--mode merge`)
- munus import --file tasks-backup.json --skip-existing — keep local tasks when imported IDs collide
- munus import --file tasks-backup.json --on-conflict skip|overwrite|rename — choose how changed tasks with the same ID are handled (default `overwrite`)
- munus import --file tasks-backup.json --id-strategy regenerate — import every task as a new task with a new ID (default `preserve`)
- munus import --file tasks-backup.json --mode replace — replace all local tasks (asks for confirmation unless `--yes`)
- munus import --file tasks-backup.json --backup — write a backup of current tasks to `~/.munus/backups/` first
- munus import --file tasks-backup.json --dry-run — show the import plan without changing anything
- munus import --file tasks-backup.json --strict — reject files with unknown fields

Examples:
- munus import --file my-tasks.json
- munus import --file my-tasks.json --skip-existing
- munus import --file my-tasks.json --mode replace --yes --backup

### Interactive TUI

- Running munus with no subcommand launches the terminal UI for interactive task management.
- By default, Munus keeps the existing non-Vim behavior: the TUI opens the new-task form, `tab`/`shift+tab` and `↑`/`↓` move between fields or tasks where applicable, `ctrl+l` switches to the task list, and `ctrl+c` quits.
- In the task list: `e` expands a task, `c` toggles completion, `d` opens the delete prompt and `y` confirms it, `n` creates a new task, `r` refreshes, `pgup`/`pgdown` (or `b`/`f`) change page, `x` exports and `i` imports.
- Run `munus --vim` to enable Vim-style TUI behavior.
- With `--vim` enabled:
  - Munus opens the task list first so list navigation acts like the primary dashboard.
  - The list accepts `j`/`k` to move, `g`/`G` to jump to the top/bottom, `l` to expand the selected task, `d` to open delete confirmation, and either `y` or consecutive `dd` to confirm deletion. `n` or `Esc` cancels the delete prompt.
  - Forms start in insert mode so task titles, descriptions, and import/export paths still accept literal text input.
  - Press `Esc` in a form to switch to normal mode, then use `j`/`k` to move between fields, `h` or `Esc` to return to the list, `l` or `Enter` to advance/submit, and `i`/`a`/`o` to return to insert mode. `o` moves to the next field before re-entering insert mode.

## Configuration

| Setting | Default | Description |
|---|---|---|
| `MUNUS_DB_PATH` | `munus.db` in the current directory | Path of the SQLite database file. Set it to use one database regardless of where you run `munus`, e.g. `mkdir -p ~/.munus && export MUNUS_DB_PATH="$HOME/.munus/munus.db"` (the directory must exist). |
| `--vim` | off | Enable Vim-style keybindings in the TUI. |

On Linux and macOS, new database, export and backup files are created readable
only by their owner (mode `0600`).
Import backups are written to `.munus/backups/` under your home directory.

## Docker

The image stores its database at `/app/data/munus.db` (`MUNUS_DB_PATH`) and runs as
an unprivileged user (UID 10001). Mount `/app/data` to keep your tasks.

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

With a host directory (run as your own user so the directory stays writable):
```bash
mkdir -p ~/.munus
docker run --rm -it \
  --user "$(id -u):$(id -g)" \
  -v ~/.munus:/app/data \
  munus:latest
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
gofmt -s -l .          # formatting (CONTRIBUTING requires gofmt -s -w . before PRs)
go vet ./...
go test ./...
go test -race ./...
```

CI (`.github/workflows/ci.yml`) additionally runs `go mod tidy` drift checks,
golangci-lint v2.13.2 and a native build smoke test on Linux, macOS and Windows.

## Project structure

```
main.go, database_path.go   entry point and MUNUS_DB_PATH handling
internal/                   CLI commands, TUI models, storage, deadlines, import/export
.github/workflows/          CI, release (CD), Docker and security workflows
Dockerfile                  container image
intel/                      maintainer notes: architecture, security tracker, plans
```
