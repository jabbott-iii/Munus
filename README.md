## Features:
- 🗹 Create a task title and description that is stored in a database.
- 🗹 Create a deadline for tasks to stay on top of your project timeline.
- 🗹 Complete tasks to check them off, they will stay in a complete state until they are deleted for data retention.
- 🗹 TUI and CLI based on preference, the task list itself (including completions and deadlines) is TUI only.

## Planned Features:
-  P2P database sharing.
-  Create cli commands for tui only features.

## status: 
- 🗹 Compilation pass. 
- 🗹 Testing pass.

## Contributing Instructions:
- Create an issue to pitch an addition or change, pull requests with no corresponding issue will be denied.
- If the issue item is already existing, comment on it before making a pull request to address the issue item.
- Confirmation of a pitched concept is required on the applicable issue before making a pull request to modify the code base.
- License header must be maintained on every source file.


## Usage Information:

Usage:

    Munus [OPTIONS]

    Munus -t "Title" -d "Description" [-n DEADLINE]

Options:

    Munus        Run without arguments to enter the terminal user interface to input a task

    -t string    Title of the task (required, max 100 chars)

    -d string    Description of the task (required, max 500 chars)

    -n string    Deadline for the task

Deadline formats:

    - Absolute: YYYY-MM-DD HH:MM (e.g., 2025-11-16 14:30)

    - Relative units:

        • m: minutes (30m = 30 minutes from now)

        • h: hours (2h = 2 hours from now)

        • d: days (1d = 1 day from now)

        • w: weeks (2w = 2 weeks from now)

        • M: months (1M = 1 month from now)

    - Combinations: 2d 3h 30m (2days, 3hours, 30 minutes from now)

    -list, -l    List all tasks in the terminal user interface

    -help, -h    Show this help message

Examples:

    Munus -t "Meeting" -d "Team sync" -n "2025-11-20 14:00"

  or

    Munus -t "Quick fix" -d "Bug #123" -n "2h"

  or

    Munus -t "Project" -d "Milestone 1" -n "1w 2d"

## Go Compile:

Linux:

  -AMD64 (64-bit Intel/AMD): 

    GOOS=linux GOARCH=amd64 go build -o munus-linux-amd64

  -ARM64 (64-bit ARM / v8): 

    GOOS=linux GOARCH=arm64 go build -o munus-linux-arm64

macOS (Darwin):

  -AMD64 (Intel Macs): 

    GOOS=darwin GOARCH=amd64 go build -o munus-mac-amd64

  -ARM64 (Apple Silicon M1/M2/M3/M4): 

    GOOS=darwin GOARCH=arm64 go build -o munus-mac-arm64

Windows:

  -AMD64 (64-bit Intel/AMD): 
  
    GOOS=windows GOARCH=amd64 go build -o munus-windows-amd64.exe

  -ARM64 (64-bit ARM Windows): 
  
    GOOS=windows GOARCH=arm64 go build -o munus-windows-arm64.exe

## Install:

Download the appropriate binary for your platform below and make it executable:

Linux:

    - chmod +x munus-linux-amd64

  or

    - chmod +x munus-linux-arm64

  then

    - sudo mv munus-linux-amd64 /usr/local/bin/munus

  or

    - sudo mv munus-linux-arm64 /usr/local/bin/munus

macOS:

    - chmod +x munus-macos-arm64

  or

    - chmod +x munus-macos-amd64

  then

    - sudo mv munus-macos-arm64 /usr/local/bin/munus

  or

    - sudo mv munus-macos-amd64 /usr/local/bin/munus

Windows:

    - Download munus-windows-amd64.exe and add it to your PATH.