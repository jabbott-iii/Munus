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
	fmt.Println("  munus")
	fmt.Println("  munus add -t \"Title\" -d \"Description\" [-n DEADLINE]")
	fmt.Println("  munus list")
	fmt.Println("  munus export [flags]")
	fmt.Println("  munus import [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  munus        Run without subcommands to enter the terminal user interface")
	fmt.Println("  add          Create a task from the CLI")
	fmt.Println("  list         List all tasks")
	fmt.Println("  export       Export tasks to JSON")
	fmt.Println("  import       Import tasks from JSON")
	fmt.Println()
	fmt.Println("Add command flags:")
	fmt.Printf("  -t, --title string          Title of the task (required, max %d chars)\n", MaxTitleLength)
	fmt.Printf("  -d, --description string    Description of the task (required, max %d chars)\n", MaxDescriptionLength)
	fmt.Println("  -n, --deadline string       Deadline for the task")

	deadlineHelp := FormatDeadlineHelp()
	lines := strings.SplitSeq(deadlineHelp, "\n")
	for line := range lines {
		if line != "" {
			fmt.Println("                              ", line)
		}
	}

	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  munus add -t \"Meeting\" -d \"Team sync\" -n \"2025-11-20 14:00\"")
	fmt.Println("  munus add -t \"Quick fix\" -d \"Bug #123\" -n \"2h\"")
	fmt.Println("  munus list")
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
