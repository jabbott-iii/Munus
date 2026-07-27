package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jabbott-iii/Munus/internal"
)

// Character limits
const (
	MaxTitleLength       = 100
	MaxDescriptionLength = 500
)

var (
	title       string
	description string
	deadline    string
	listMode    bool
	showHelp    bool
)

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

	db, err := internal.NewDatabase("munus.db")
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

	if listMode {
		p := tea.NewProgram(internal.NewListModel(db), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			log.Fatalf("error running list view: %v", err)
		}
		return
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

	if len(title) > MaxTitleLength {
		fmt.Printf("Error: title exceeds maximum length of %d characters (current: %d)\n", MaxTitleLength, len(title))
		os.Exit(1)
	}

	if len(description) > MaxDescriptionLength {
		fmt.Printf("Error: description exceeds maximum length of %d characters (current: %d)\n", MaxDescriptionLength, len(description))
		os.Exit(1)
	}

	var deadlineTime = (*internal.Deadline)(nil)
	if deadline != "" {
		parsed, err := internal.ParseDeadline(deadline)
		if err != nil {
			log.Fatalf("invalid deadline format: %v", err)
		}
		deadlineTime = parsed
	}

	task := &internal.ItemModel{
		Title:       title,
		Description: description,
		Deadline:    (*deadlineTime),
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
