## Features:
- 🗹 Create a task title and description that is stored in a database.
- 🗹 Create a deadline for tasks to stay on top of your project timeline.
- 🗹 Complete tasks to check them off.
- 🗹 Delete tasks and completed tasks at will.
- 🗹 Data storage via sqlite.
- 🗹 Export data to .json and import it back.

## Planned Features:
-  Git integration.
-  Github task to issue.

## Install:

Download the appropriate binary for your platform below and make it executable:

Linux:
```
chmod +x munus-linux-amd64
```
```
sudo mv munus-linux-amd64 /usr/local/bin/munus
```
 or
```
chmod +x munus-linux-arm64
```
```
sudo mv munus-linux-arm64 /usr/local/bin/munus
```
macOS:
```
chmod +x munus-macos-arm64
```
```
sudo mv munus-macos-arm64 /usr/local/bin/munus
```
  or
```
chmod +x munus-macos-amd64
```
```
sudo mv munus-macos-amd64 /usr/local/bin/munus
```
Windows:
```
Download munus-windows-amd64.exe and add it to your PATH as munus.
```

## Usage Information:

Usage:
  munus [flags]
  munus [command]

Available Commands:
  add         Create a new task
  complete    Complete task
  completion  Generate the autocompletion script for the specified shell
  delete      Delete a task
  export      Export tasks to JSON
  help        Help about any command
  import      Import tasks from JSON
  list        List all tasks

Flags:
  -h, --help   help for munus
_______________________________________________________________________________

Create a new task

Usage:
  munus add [flags]

Examples:
	munus add -t "Meeting" -d "Team sync" -n "2025-11-20 14:00
	`				
	Deadline formats:
	- Absolute: YYYY-MM-DD HH:MM (e.g., 2025-11-16 14:30)
	- Relative units:
		• m: minutes (30m = 30 minutes from now)
		• h: hours (2h = 2 hours from now)
		• d: days (1d = 1 day from now)
		• w: weeks (2w = 2 weeks from now)
		• M: months (1M = 1 month from now)
	- Combinations: 2d 3h 30m (2days, 3hours, 30 minutes from now)

Flags:
  -n, --deadline string      Deadline for the task (formats: YYYY-MM-DD HH:MM | 2d 3h 30m | 1d)
  -d, --description string   Description of the task
  -h, --help                 help for add
  -t, --title string         Title of the task

_______________________________________________________________________________

List all tasks

Usage:
  munus list [flags]

Examples:
	munus list

Flags:
  -h, --help   help for list

_______________________________________________________________________________

Complete task

Usage:
  munus complete [task-id] [flags]

Examples:
	munus complete 12
	munus complete 12 --undo

Flags:
  -h, --help   help for complete
  -u, --undo   mark incomplete

_______________________________________________________________________________

Delete a task

Usage:
  munus delete [task-id] [flags]

Examples:
	munus delete 12

Flags:
  -h, --help   help for delete

_______________________________________________________________________________

Export tasks to a versioned JSON file for backup/migration.

Usage:
  munus export [flags]

Examples:
	munus export -f tasks.json
	munus export --stdout > tasks.json
	munus export --include-completed
	munus export --dry-run

Flags:
      --dry-run             Show what would be exported without writing
  -f, --file string         Output JSON file path
  -h, --help                help for export
  -i, --include-completed   Include completed tasks
      --pretty              Pretty-print JSON output (default true)
      --stdout              Write JSON to stdout

_______________________________________________________________________________

Import tasks from a versioned JSON export.

Usage:
  munus import [flags]

Examples:
	munus import -f tasks.json
	munus import -f tasks.json --mode replace --yes --backup
	munus import -f tasks.json --dry-run --strict
	munus import -f tasks.json --mode merge --on-conflict rename --id-strategy ict

Flags:
      --backup               Create backup before applying changes
      --dry-run              Validate and show plan without applying
  -f, --file string          Input JSON file path ('-' for stdin if implemented)
  -h, --help                 help for import
      --id-strategy string   ID policy: preserve|regenerate (default "preserve")
      --mode string          Import mode: merge|replace (default "merge")
      --on-conflict string   Conflict policy: skip|overwrite|rename (default "overwrite")
      --strict               Fail on unknown fields/invalid enums
  -y, --yes                  Skip confirmation prompts
  
## Run with Docker:

### Build the image

```bash
docker build -t munus:latest .
```

### Run interactively (recommended for TUI)

```bash
docker run --rm -it \
  -v munus-data:/app/data \
  munus:latest
```

This starts the TUI/CLI and persists your SQLite database in a Docker volume (`munus-data`).

### Run CLI mode with flags

```bash
docker run --rm -it \
  -v munus-data:/app/data \
  munus:latest -t "Meeting" -d "Team sync" -n "2h"
```

### Show help

```bash
docker run --rm munus:latest -h
```

## Install:

Download the appropriate binary for your platform below and make it executable:

Linux:
```
chmod +x munus-linux-amd64
```
```
sudo mv munus-linux-amd64 /usr/local/bin/munus
```
 or
```
chmod +x munus-linux-arm64
```
```
sudo mv munus-linux-arm64 /usr/local/bin/munus
```
macOS:
```
chmod +x munus-macos-arm64
```
```
sudo mv munus-macos-arm64 /usr/local/bin/munus
```
  or
```
chmod +x munus-macos-amd64
```
```
sudo mv munus-macos-amd64 /usr/local/bin/munus
```
Windows:
```
Download munus-windows-amd64.exe and add it to your PATH as munus.
```
