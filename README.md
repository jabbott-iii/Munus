## Features:
- 🗹 Create a task title and description that is stored in a database.
- 🗹 Create a deadline for tasks to stay on top of your project timeline.
- 🗹 Complete tasks to check them off.
- 🗹 Delete tasks and completed tasks at will.
- 🗹 Data storage via sqlite.

## Planned Features:
-  Git integration.
-  Github task to issue.
-  File export and import.

## Usage Information:

Usage:

    munus [OPTIONS]

    munus -t "Title" -d "Description" [-n DEADLINE]

Options:

    munus        Run without arguments to enter the terminal user interface to input a task

    -t string    Title of the task (required, max 100 chars)

    -d string    Description of the task (required, max 500 chars)

    -n string    Deadline for the task
    
    -list, -l    List all tasks in the terminal user interface

    -help, -h    Show this help message

Deadline formats:

    - Absolute: YYYY-MM-DD HH:MM (e.g., 2025-11-16 14:30)

    - Relative units:

        • m: minutes (30m = 30 minutes from now)

        • h: hours (2h = 2 hours from now)

        • d: days (1d = 1 day from now)

        • w: weeks (2w = 2 weeks from now)

        • M: months (1M = 1 month from now)

    - Combinations: 2d 3h 30m (2days, 3hours, 30 minutes from now)

Examples:
```
munus -t "Meeting" -d "Team sync" -n "2025-11-20 14:00"
```
```    
munus -t "Quick fix" -d "Bug #123" -n "2h"
```
```
munus -t "Project" -d "Milestone 1" -n "1w 2d"
```
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
