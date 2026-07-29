/*
Copyright 2026 Joseph Anthony Abbott III

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	"fmt"
	"strings"
)

func PrintHelp() {
	fmt.Println("Munus - A task manager")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  Munus [OPTIONS]")
	fmt.Println("  Munus -t \"Title\" -d \"Description\" [-n DEADLINE]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  Munus    Run without arguments to enter the terminal user interface to input a task")
	fmt.Printf("  -t string    Title of the task (required, max 100 chars)\n")
	fmt.Printf("  -d string    Description of the task (required, max 500 chars)\n")
	fmt.Println("  -n string    Deadline for the task")

	deadlineHelp := FormatDeadlineHelp()
	lines := strings.SplitSeq(deadlineHelp, "\n")
	for line := range lines {
		if line != "" {
			fmt.Println("              ", line)
		}
	}

	fmt.Println("  -list, -l    List all tasks in a terminal user interface")
	fmt.Println("  -help, -h    Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  Munus -t \"Meeting\" -d \"Team sync\" -n \"2025-11-20 14:00\"")
	fmt.Println("  Munus -t \"Quick fix\" -d \"Bug #123\" -n \"2h\"")
	fmt.Println("  Munus -t \"Project\" -d \"Milestone 1\" -n \"1w 2d\"")
}

// FormatDeadlineHelp returns a help string explanation the deadline formats
func FormatDeadlineHelp() string {
	return `Deadline formats:
	- Absolute: YYYY-MM-DD HH:MM (e.g., 2025-11-16 14:30)
	- Relative units:
		• m: minutes (30m = 30 minutes from now)
		• h: hours (2h = 2 hours from now)
		• d: days (1d = 1 day from now)
		• w: weeks (2w = 2 weeks from now)
		• M: months (1M = 1 month from now)
	- Combinations: 2d 3h 30m (2days, 3hours, 30 minutes from now)`
}
