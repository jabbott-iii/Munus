## status: 
- compiled success, needs testing

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

    Munus -t "Quick fix" -d "Bug #123" -n "2h"

    Munus -t "Project" -d "Milestone 1" -n "1w 2d"
