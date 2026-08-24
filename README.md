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

<img width="593" height="354" alt="image" src="https://github.com/user-attachments/assets/623a4653-bb13-4462-a63d-78475f2d1476" />

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
