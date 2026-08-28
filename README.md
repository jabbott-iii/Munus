<img width="812" height="542" alt="munusv2" src="https://github.com/user-attachments/assets/3ec01b8f-bfde-4fae-a114-78f50171dcf7" />

## Features:

- **Task Management**
  - Create tasks with title and description
  - Set deadlines for project timeline tracking
  - Mark tasks as complete
  - Delete tasks at any time

- **Data Storage & Export**
  - SQLite database for persistent storage
  - Export tasks to JSON format
  - Import tasks from JSON files

- **Interactive TUI**
  - Terminal user interface for task browsing and management
  - Launches automatically when run without subcommands

## Core CLI capabilities

Munus is organized into focused command groups:

- munus add — task creation and management
- munus list — display all tasks
- munus complete — mark tasks as finished
- munus delete — remove tasks
- munus export — export tasks to JSON
- munus import — import tasks from JSON

### add

- munus add --title "Title" --description "Description" — add a task with details
- munus add --title "Title" --deadline "2h" — create a task with a deadline

Examples:
- munus add --title "Feature Review" --description "Review new API endpoints"
- munus add --title "Bug Fix" --deadline "1d" --description "Fix login validation"
- munus add -t "Meeting" -d "Team sync" -n "2h"

### list

- munus list --pending — show only incomplete tasks
- munus list --completed — show only completed tasks

Examples:
- munus list
- munus list --pending
- munus list --completed

### complete

- munus complete [task-id] — mark a task as complete

Examples:
- munus complete 1
- munus complete 5

### delete

- munus delete [task-id] — remove a task

Examples:
- munus delete 1
- munus delete 3

### export

- munus export --file tasks-backup.json — export to a specific file

Examples:
- munus export --file my-tasks.json

### import

- munus import --file tasks-backup.json — restore tasks from JSON file

Examples:
- munus import --file my-tasks.json

### Interactive TUI

- Running munus with no subcommand launches the terminal UI for interactive task management.

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

## Docker

### Build
```bash
docker build -t munus:latest .
```

### Run (interactive)
```bash
docker run --rm -it munus:latest
```

### Persist data
```bash
mkdir -p ~/.munus
docker run --rm -it \
  -v ~/.munus:/root/.munus \
  munus:latest
```

### CLI usage
```bash
docker run --rm -it munus:latest --help
docker run --rm -it -v ~/.munus:/root/.munus munus:latest add "Example task"
docker run --rm -it -v ~/.munus:/root/.munus munus:latest list
```
