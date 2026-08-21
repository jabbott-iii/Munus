package internal

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func newTestDatabase(t *testing.T) *Database {
	t.Helper()

	db, err := NewDatabase(filepath.Join(t.TempDir(), "munus.db"))
	if err != nil {
		t.Fatalf("NewDatabase() error = %v", err)
	}

	return db
}

func TestNewAddCmdCreatesTask(t *testing.T) {
	db := newTestDatabase(t)
	cmd := NewAddCmd(db)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"-t", "Meeting", "-d", "Team sync", "-n", "2h"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	tasks, err := db.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "Meeting" {
		t.Fatalf("expected title %q, got %q", "Meeting", tasks[0].Title)
	}
	if tasks[0].Description != "Team sync" {
		t.Fatalf("expected description %q, got %q", "Team sync", tasks[0].Description)
	}
	if tasks[0].Deadline == nil {
		t.Fatal("expected deadline to be parsed")
	}
	if tasks[0].Completed {
		t.Fatal("expected new task to be incomplete")
	}
	if !strings.Contains(out.String(), "Task created successfully") {
		t.Fatalf("expected success message, got %q", out.String())
	}
}

func TestNewAddCmdRejectsLongTitle(t *testing.T) {
	db := newTestDatabase(t)
	cmd := NewAddCmd(db)
	cmd.SetArgs([]string{"-t", strings.Repeat("a", MaxTitleLength+1), "-d", "Team sync"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error for an overly long title")
	}
	if !strings.Contains(err.Error(), "title exceeds maximum length") {
		t.Fatalf("expected title length error, got %v", err)
	}
}

func TestNewListCmdPrintsTasks(t *testing.T) {
	db := newTestDatabase(t)
	if err := db.CreateTask(&ItemModel{Title: "Task 1", Description: "Demo", Completed: false}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	cmd := NewListCmd(db)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	printed := out.String()
	if !strings.Contains(printed, "Task 1") || !strings.Contains(printed, "Demo") {
		t.Fatalf("expected task details in output, got %q", printed)
	}
}

func TestNewRootCmdWiresSubcommands(t *testing.T) {
	db := newTestDatabase(t)
	root := NewRootCmd(db)

	for _, name := range []string{"add", "list", "export", "import"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("Find(%q) error = %v", name, err)
		}
		if cmd == nil || cmd.Name() != name {
			t.Fatalf("expected subcommand %q to be wired", name)
		}
	}
}
