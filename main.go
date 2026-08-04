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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jabbott-iii/Munus/internal"
)

// Character limits
const (
	MaxTitleLength       = 100
	MaxDescriptionLength = 500
)

// cli flag variables
var (
	title       string
	description string
	deadline    string
	listMode    bool
	showHelp    bool
)

// cli flags
func init() {
	flag.StringVar(&title, "title", "", "Title of the task")
	flag.StringVar(&title, "t", "", "Title of the task")

	flag.StringVar(&description, "description", "", "Description of the task")
	flag.StringVar(&description, "d", "", "Description of the task")

	flag.StringVar(&deadline, "deadline", "", "Deadline for the task")
	flag.StringVar(&deadline, "n", "", "Deadline for the task")

	flag.BoolVar(&listMode, "list", false, "List all tasks")
	flag.BoolVar(&listMode, "l", false, "List all tasks")

	flag.BoolVar(&showHelp, "help", false, "Show help")
	flag.BoolVar(&showHelp, "h", false, "Show help")
}

func main() {
	flag.Parse()

	if showHelp {
		internal.PrintHelp()
		os.Exit(0)
	}

	// sqlite db creation / use
	db, err := internal.NewDatabase("munus.db")
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

	// printlist data
	taskId := []internal.ItemModel{
		{ID: 1, Title: "Task 1", Description: "Demo", Completed: false},
	}
	
	if listMode {
		internal.PrintList(taskId)
		os.Exit(0)
	}

	// No CLI args: open interactive create form
	if title == "" && description == "" {
		p := tea.NewProgram(internal.NewFormModel(db), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			log.Fatalf("error running form view: %v", err)
		}
		return
	}

	// Partial CLI input is invalid
	if title == "" || description == "" {
		fmt.Println("Error: both title (-t) and description (-d) are required")
		internal.PrintHelp()
		os.Exit(1)
	}

	// title length enforcement
	if len(title) > MaxTitleLength {
		fmt.Printf("Error: title exceeds maximum length of %d characters (current: %d)\n", MaxTitleLength, len(title))
		os.Exit(1)
	}

	// description length enforcement
	if len(description) > MaxDescriptionLength {
		fmt.Printf("Error: description exceeds maximum length of %d characters (current: %d)\n", MaxDescriptionLength, len(description))
		os.Exit(1)
	}

	// deadline enforcement
	var deadlineTime *time.Time
	if deadline != "" {
		parsed, err := internal.ParseDeadline(deadline)
		if err != nil {
			log.Fatalf("invalid deadline format: %v", err)
		}
		deadlineTime = parsed
	}

	// task item
	task := &internal.ItemModel{
		Title:       title,
		Description: description,
		Deadline:    deadlineTime,
		Completed:   false,
	}

	if err := db.CreateTask(task); err != nil {
		log.Fatalf("failed to save task: %v", err)
	}

	fmt.Println("✔ Task created successfully!")
	fmt.Printf("Title: %s\n", task.Title)
	if task.Deadline != nil {
		fmt.Printf("Deadline: %s\n", task.Deadline.Format("2006-01-02 15:04"))
	}
}
