package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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
	flag.StringVar(&title, "title", "", "Title of the todo")
	flag.StringVar(&title, "t", "", "Title of the todo")

	flag.StringVar(&description, "description", "", "Description of the todo")
	flag.StringVar(&description, "d", "", "Description of the todo")

	flag.StringVar(&deadline, "deadline", "", "Deadline for the todo")
	flag.StringVar(&deadline, "n", "", "Deadline for the todo")

	flag.BoolVar(&listMode, "list", false, "List all todos")
	flag.BoolVar(&listMode, "l", false, "List all todos")

	flag.BoolVar(&showHelp, "help", false, "Show help")
	flag.BoolVar(&showHelp, "h", false, "Show help")
}

func main() {
	flag.Parse()

	if showHelp {
		printHelp()
		os.Exit(0)
	}

	dbPath, err := getDBPath()
	if err != nil {
		log.Fatal("Failed to get database path:", err)
	}

	store, err := storage.NewBoltStorage(dbPath)
	if err != nil {
		log.Fatal("Failed to initialize storage:", err)
	}
	defer store.Close()

	if listMode {
		p := tea.NewProgram(ui.NewListModel(store), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			log.Fatal("Error running list view:", err)
		}
		return
	}

	if title == "" && description == "" {
		p := tea.NewProgram(ui.NewFormModel(store), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			log.Fatal("Error running form view:", err)
		}
		return
	}

	if title == "" || description == "" {
		fmt.Println("Error: Both title (-t) and description (-d) are required")
		printHelp()
		os.Exit(1)
	}

	if len(title) > MaxTitleLength {
		fmt.Printf("Error: Title exceeds maximum length of %d characters (current: %d)\n", MaxTitleLength, len(title))
		os.Exit(1)
	}

	if len(description) > MaxDescriptionLength {
		fmt.Printf("Error: Description exceeds maximum length of %d characters (current: %d)\n", MaxDescriptionLength, len(description))
		os.Exit(1)
	}

	var deadlineTime *time.Time
	if deadline != "" {
		parsed, err := utils.ParseDeadline(deadline)
		if err != nil {
			log.Fatal("Invalid deadline format: ", err)
		}
		deadlineTime = parsed
	}

	task := internal.Munus{
		ID:          generateID(),
		Title:       title,
		Description: description,
		Deadline:    deadlineTime,
		CreatedAt:   time.Now(),
		Completed:   false,
	}

	if err := store.SaveTodo(&task); err != nil {
		log.Fatal("Failed to save todo:", err)
	}

	fmt.Printf("✔ Todo created successfully!\n")
	fmt.Printf("Title: %s\n", task.Title)
	if deadlineTime != nil {
		fmt.Printf("Deadline: %s\n", deadlineTime.Format("2006-01-02 15:04"))
	}
}

func printHelp() {
	fmt.Println("Munus - A task manager")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  Munus [OPTIONS]")
	fmt.Println("  Munus -t \"Title\" -d \"Description\" [-n DEADLINE]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Printf("  -t string    Title of the task (required, max %d chars)\n", MaxTitleLength)
	fmt.Printf("  -d string    Description of the task (required, max %d chars)\n", MaxDescriptionLength)
	fmt.Println("  -n string    Deadline for the task")

	deadlineHelp := utils.FormatDeadlineHelp()
	lines := strings.SplitSeq(deadlineHelp, "\n")
	for line := range lines {
		if line != "" {
			fmt.Println("              ", line)
		}
	}
	fmt.Println("  -list, -l    List all todos")
	fmt.Println("  -help, -h    Show this help message")
	fmt.Println()
	fmt.Println("Interactive Mode:")
	fmt.Println(" Run without arguments to enter interactive mode")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  Munus -t \"Meeting\" -d \"Team sync\" -n \"2025-11-20 14:00\"")
	fmt.Println("  Munus -t \"Quick fix\" -d \"Bug #123\" -n \"2h\"")
	fmt.Println("  Munus -t \"Project\" -d \"Milestone 1\" -n \"1w 2d\"")
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

/* func getDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	dataDir := filepath.Join(home, ".local", "share", "munus")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}

	return filepath.Join(dataDir, "doit.db"), nil
}
*/
