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
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// ============================================ MockDatabase ============================================

// Match the production database type used by the CLI commands.
type MockModel = Database

func NewMockModel() *MockModel {
	db, err := NewDatabase(":memory:")
	if err != nil {
		panic(err)
	}
	return db

}

// ============================================ GetTaskStatus Tests ============================================

func TestGetTaskStatus_Completed(t *testing.T) {
	task := &ItemModel{
		ID:        1,
		Title:     "Done Task",
		Completed: true,
	}
	status := GetTaskStatus(task)
	if status != "✓ DONE" {
		t.Errorf("expected '✓ DONE', got %q", status)
	}
}

func TestGetTaskStatus_Overdue(t *testing.T) {
	pastTime := time.Now().Add(-1 * time.Hour)
	task := &ItemModel{
		ID:        1,
		Title:     "Overdue Task",
		Completed: false,
		Deadline:  &pastTime,
	}
	status := GetTaskStatus(task)
	if status != "⚠ OVERDUE" {
		t.Errorf("expected '⚠ OVERDUE', got %q", status)
	}
}

func TestGetTaskStatus_Todo(t *testing.T) {
	futureTime := time.Now().Add(24 * time.Hour)
	task := &ItemModel{
		ID:        1,
		Title:     "Todo Task",
		Completed: false,
		Deadline:  &futureTime,
	}
	status := GetTaskStatus(task)
	if status != "○ TODO" {
		t.Errorf("expected '○ TODO', got %q", status)
	}
}

func TestGetTaskStatus_NoDeadline(t *testing.T) {
	task := &ItemModel{
		ID:        1,
		Title:     "Todo Task",
		Completed: false,
		Deadline:  nil,
	}
	status := GetTaskStatus(task)
	if status != "○ TODO" {
		t.Errorf("expected '○ TODO', got %q", status)
	}
}

// ============================================ PrintList Tests ============================================

func TestPrintList_Empty(t *testing.T) {
	buf := &bytes.Buffer{}
	PrintList(buf, []*ItemModel{})
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %q", buf.String())
	}
}

func TestPrintList_MultipleTasks(t *testing.T) {
	buf := &bytes.Buffer{}
	tasks := []*ItemModel{
		{
			ID:          1,
			Title:       "Task 1",
			Description: "Desc 1",
			Completed:   false,
		},
		{
			ID:          2,
			Title:       "Task 2",
			Description: "Desc 2",
			Completed:   true,
		},
	}
	PrintList(buf, tasks)

	output := buf.String()
	if !strings.Contains(output, "Task 1") || !strings.Contains(output, "Task 2") {
		t.Errorf("output missing tasks: %s", output)
	}
}

// ============================================ NewAddCmd Tests ============================================

func TestAddCmd_Success(t *testing.T) {
	db := NewMockModel()
	cmd := NewAddCmd(db)

	// Set flags
	cmd.SetArgs([]string{"-t", "Test Task", "-d", "Test Description"})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !strings.Contains(buf.String(), "✔ Task created successfully!") {
		t.Errorf("expected success message, got %s", buf.String())
	}

	tasks, _ := db.ListTasks()
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

func TestAddCmd_MissingTitle(t *testing.T) {
	db := NewMockModel()
	cmd := NewAddCmd(db)

	cmd.SetArgs([]string{"-d", "Test Description"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for missing title")
	}
}

func TestAddCmd_MissingDescription(t *testing.T) {
	db := NewMockModel()
	cmd := NewAddCmd(db)

	cmd.SetArgs([]string{"-t", "Test Task"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for missing description")
	}
}

func TestAddCmd_TitleExceedsMaxLength(t *testing.T) {
	db := NewMockModel()
	cmd := NewAddCmd(db)

	longTitle := strings.Repeat("a", MaxTitleLength+1)
	cmd.SetArgs([]string{"-t", longTitle, "-d", "Description"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for title exceeding max length")
	}
}

func TestAddCmd_DescriptionExceedsMaxLength(t *testing.T) {
	db := NewMockModel()
	cmd := NewAddCmd(db)

	longDesc := strings.Repeat("a", MaxDescriptionLength+1)
	cmd.SetArgs([]string{"-t", "Title", "-d", longDesc})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for description exceeding max length")
	}
}

func TestAddCmd_WithDeadline(t *testing.T) {
	db := NewMockModel()
	cmd := NewAddCmd(db)

	cmd.SetArgs([]string{"-t", "Task", "-d", "Desc", "-n", "1d"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	tasks, _ := db.ListTasks()
	if len(tasks) != 1 || tasks[0].Deadline == nil {
		t.Errorf("expected task with deadline")
	}
}

func TestAddCmd_InvalidDeadline(t *testing.T) {
	db := NewMockModel()
	cmd := NewAddCmd(db)

	cmd.SetArgs([]string{"-t", "Task", "-d", "Desc", "-n", "invalid-deadline"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for invalid deadline")
	}
}

// ============================================ NewListCmd Tests ============================================

func TestListCmd_NoTasks(t *testing.T) {
	db := NewMockModel()
	cmd := NewListCmd(db)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected empty output for no tasks, got %s", buf.String())
	}
}

func TestListCmd_WithTasks(t *testing.T) {
	db := NewMockModel()

	// Add some tasks
	err := db.CreateTask(&ItemModel{
		Title:       "Task 1",
		Description: "Desc 1",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	err = db.CreateTask(&ItemModel{
		Title:       "Task 2",
		Description: "Desc 2",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	cmd := NewListCmd(db)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err = cmd.Execute()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Task 1") || !strings.Contains(output, "Task 2") {
		t.Errorf("expected tasks in output, got %s", output)
	}
}

// ============================================ DeleteTaskCmd Tests ============================================

func TestDeleteTaskCmd_InvalidID(t *testing.T) {
	db := NewMockModel()
	cmd := DeleteTaskCmd(db)

	cmd.SetArgs([]string{"invalid"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for invalid task ID")
	}
}

func TestDeleteTaskCmd_NegativeID(t *testing.T) {
	db := NewMockModel()
	cmd := DeleteTaskCmd(db)

	// "--" makes cobra treat -5 as the task-id argument rather than a flag.
	cmd.SetArgs([]string{"--", "-5"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()

	if err == nil || !strings.Contains(err.Error(), "must be a positive integer") {
		t.Errorf("expected positive-integer error for negative task ID, got %v", err)
	}
}

func TestDeleteTaskCmd_UserCancels(t *testing.T) {
	db := NewMockModel()
	err := db.CreateTask(&ItemModel{
		Title:       "Task",
		Description: "Desc",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	cmd := DeleteTaskCmd(db)
	cmd.SetArgs([]string{"1"})

	// Simulate user input: "n" (cancel)
	stdin := strings.NewReader("n\n")
	cmd.SetIn(stdin)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err = cmd.Execute()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !strings.Contains(buf.String(), "Delete cancelled") {
		t.Errorf("expected cancellation message, got %s", buf.String())
	}

	// Verify task still exists
	tasks, _ := db.ListTasks()
	if len(tasks) != 1 {
		t.Errorf("expected task to still exist")
	}
}

func TestDeleteTaskCmd_Success(t *testing.T) {
	db := NewMockModel()
	err := db.CreateTask(&ItemModel{
		Title:       "Task",
		Description: "Desc",
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	cmd := DeleteTaskCmd(db)
	cmd.SetArgs([]string{"1"})

	// Simulate user input: "y" (confirm)
	stdin := strings.NewReader("y\n")
	cmd.SetIn(stdin)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err = cmd.Execute()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if !strings.Contains(buf.String(), "deleted") {
		t.Errorf("expected deletion message, got %s", buf.String())
	}

	// Verify task was deleted
	tasks, _ := db.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("expected no tasks after deletion")
	}
}

// ============================================ CompleteTaskCmd Tests ============================================

func TestCompleteTaskCmd_InvalidID(t *testing.T) {
	db := NewMockModel()
	cmd := CompleteTaskCmd(db)

	cmd.SetArgs([]string{"invalid"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for invalid task ID")
	}
}

func TestCompleteTaskCmd_TaskNotFound(t *testing.T) {
	db := NewMockModel()
	cmd := CompleteTaskCmd(db)

	cmd.SetArgs([]string{"999"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for non-existent task")
	}
}

func TestCompleteTaskCmd_CompleteTask(t *testing.T) {
	db := NewMockModel()
	err := db.CreateTask(&ItemModel{
		Title:       "Task",
		Description: "Desc",
		Completed:   false,
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	cmd := CompleteTaskCmd(db)
	cmd.SetArgs([]string{"1"})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err = cmd.Execute()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	task, _ := db.GetTaskByID(1)
	if !task.Completed {
		t.Errorf("expected task to be completed")
	}
	if task.CompletedAt == nil {
		t.Errorf("expected CompletedAt to be set")
	}
}

func TestCompleteTaskCmd_UndoComplete(t *testing.T) {
	db := NewMockModel()
	now := time.Now()
	err := db.CreateTask(&ItemModel{
		Title:       "Task",
		Description: "Desc",
		Completed:   true,
		CompletedAt: &now,
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	cmd := CompleteTaskCmd(db)
	cmd.SetArgs([]string{"1", "--undo"})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err = cmd.Execute()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	task, _ := db.GetTaskByID(1)
	if task.Completed {
		t.Errorf("expected task to be incomplete")
	}
	if task.CompletedAt != nil {
		t.Errorf("expected CompletedAt to be nil")
	}
}

// ============================================ Confirm Tests ============================================

func TestConfirm_UserConfirmsWithY(t *testing.T) {
	cmd := NewRootCmd(NewMockModel())
	stdin := strings.NewReader("y\n")
	cmd.SetIn(stdin)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	ok, err := Confirm(cmd, "Confirm? ")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !ok {
		t.Errorf("expected user to confirm with 'y'")
	}
}

func TestConfirm_UserConfirmsWithYes(t *testing.T) {
	cmd := NewRootCmd(NewMockModel())
	stdin := strings.NewReader("yes\n")
	cmd.SetIn(stdin)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	ok, err := Confirm(cmd, "Confirm? ")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !ok {
		t.Errorf("expected user to confirm with 'yes'")
	}
}

func TestConfirm_UserDeniesWithN(t *testing.T) {
	cmd := NewRootCmd(NewMockModel())
	stdin := strings.NewReader("n\n")
	cmd.SetIn(stdin)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	ok, err := Confirm(cmd, "Confirm? ")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if ok {
		t.Errorf("expected user to deny confirmation")
	}
}

func TestConfirm_UserDeniesWithNo(t *testing.T) {
	cmd := NewRootCmd(NewMockModel())
	stdin := strings.NewReader("no\n")
	cmd.SetIn(stdin)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	ok, err := Confirm(cmd, "Confirm? ")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if ok {
		t.Errorf("expected user to deny confirmation")
	}
}

func TestConfirm_CaseInsensitive(t *testing.T) {
	cmd := NewRootCmd(NewMockModel())
	stdin := strings.NewReader("YES\n")
	cmd.SetIn(stdin)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	ok, err := Confirm(cmd, "Confirm? ")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !ok {
		t.Errorf("expected user to confirm with 'YES'")
	}
}

// ============================================ NewRootCmd Tests ============================================

func TestNewRootCmd_HasSubcommands(t *testing.T) {
	db := NewMockModel()
	cmd := NewRootCmd(db)

	subcommands := cmd.Commands()
	expectedCmd := map[string]bool{
		"add":      false,
		"list":     false,
		"delete":   false,
		"complete": false,
		"export":   false,
		"import":   false,
	}

	for _, subsumed := range subcommands {
		expectedCmd[subsumed.Name()] = true
	}

	for cmdName, found := range expectedCmd {
		if !found {
			t.Errorf("expected subcommand %q not found", cmdName)
		}
	}

	if flag := cmd.Flags().Lookup("vim"); flag == nil {
		t.Fatal("expected --vim flag to be registered on root command")
	}
}

func TestImportCmdSkipExistingReportsSkippedIDs(t *testing.T) {
	db := NewMockModel()
	if err := db.CreateTask(&ItemModel{Title: "Existing Task", Description: "Local"}); err != nil {
		t.Fatalf("failed to seed task: %v", err)
	}

	now := time.Now()
	filePath := writeTestImportBundle(t, ExportBundle{
		Version:    1,
		ExportedAt: now,
		Tasks: []TaskDTO{
			{
				ID:          "1",
				Title:       "Existing Task",
				Description: "Imported",
				Completed:   false,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "2",
				Title:       "New Task",
				Description: "New Description",
				Completed:   false,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	})

	cmd := NewImportCmd(db)
	cmd.SetArgs([]string{"--file", filePath, "--skip-existing"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("import command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Conflicts: 1") {
		t.Errorf("expected conflict count in output, got %q", output)
	}
	if !strings.Contains(output, "Conflicting task IDs: 1") {
		t.Errorf("expected conflict IDs in output, got %q", output)
	}
	if !strings.Contains(output, "Skipped existing task IDs: 1") {
		t.Errorf("expected skipped IDs in output, got %q", output)
	}
}

func TestImportCmdRejectsAmbiguousConflictFlags(t *testing.T) {
	db := NewMockModel()
	cmd := NewImportCmd(db)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "import.json")
	if err := os.WriteFile(filePath, []byte(`{"version":1,"tasks":[]}`), 0o644); err != nil {
		t.Fatalf("failed to write import file: %v", err)
	}

	cmd.SetArgs([]string{"--file", filePath, "--skip-existing", "--on-conflict", "rename"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when combining --skip-existing with --on-conflict")
	}
	if !strings.Contains(err.Error(), "--skip-existing cannot be combined with --on-conflict") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExportCmdDryRun(t *testing.T) {
	db := NewMockModel()
	if err := db.CreateTask(&ItemModel{Title: "Todo", Description: "Task", Completed: false}); err != nil {
		t.Fatalf("failed to seed todo task: %v", err)
	}
	if err := db.CreateTask(&ItemModel{Title: "Done", Description: "Task", Completed: true}); err != nil {
		t.Fatalf("failed to seed done task: %v", err)
	}

	cmd := NewExportCmd(db)
	cmd.SetArgs([]string{"--dry-run"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export dry-run failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Would export 1 tasks") {
		t.Fatalf("expected dry-run summary, got %q", buf.String())
	}
}

func TestExportCmdStdout(t *testing.T) {
	db := NewMockModel()
	if err := db.CreateTask(&ItemModel{Title: "Task", Description: "Desc"}); err != nil {
		t.Fatalf("failed to seed task: %v", err)
	}

	cmd := NewExportCmd(db)
	cmd.SetArgs([]string{"--stdout"})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export stdout failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, `"version": 1`) {
		t.Fatalf("expected version in JSON output, got %q", output)
	}
	if !strings.Contains(output, `"title": "Task"`) {
		t.Fatalf("expected task in JSON output, got %q", output)
	}
}

func TestExportCmdWritesFile(t *testing.T) {
	db := NewMockModel()
	if err := db.CreateTask(&ItemModel{Title: "Task", Description: "Desc"}); err != nil {
		t.Fatalf("failed to seed task: %v", err)
	}

	filePath := filepath.Join(t.TempDir(), "export.json")
	cmd := NewExportCmd(db)
	cmd.SetArgs([]string{"--file", filePath})
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export file failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Exported 1 tasks") {
		t.Fatalf("expected success output, got %q", buf.String())
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("expected export file to exist: %v", err)
	}
	if !strings.Contains(string(data), `"tasks"`) {
		t.Fatalf("expected tasks key in export file")
	}
}

func TestExportCmdReturnsErrorWhenDatabaseFails(t *testing.T) {
	db := NewMockModel()
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close db: %v", err)
	}

	cmd := NewExportCmd(db)
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected export to fail with closed database")
	}
}

func TestImportCmdRequiresFileFlag(t *testing.T) {
	db := NewMockModel()
	cmd := NewImportCmd(db)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing file flag error")
	}
	if !strings.Contains(err.Error(), "required flag: --file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportCmdReplaceModeAbortsWhenUserDeclines(t *testing.T) {
	db := NewMockModel()
	if err := db.CreateTask(&ItemModel{Title: "Existing", Description: "Local"}); err != nil {
		t.Fatalf("failed to seed existing task: %v", err)
	}

	now := time.Now()
	filePath := writeTestImportBundle(t, ExportBundle{
		Version:    1,
		ExportedAt: now,
		Tasks: []TaskDTO{
			{
				ID:          "2",
				Title:       "Imported Task",
				Description: "Imported Desc",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	})

	cmd := NewImportCmd(db)
	cmd.SetArgs([]string{"--file", filePath, "--mode", "replace"})
	cmd.SetIn(strings.NewReader("n\n"))

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected aborted-by-user error")
	}
	if !strings.Contains(err.Error(), "aborted by user") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportCmdReplaceModeAppliesWhenUserConfirms(t *testing.T) {
	db := NewMockModel()
	if err := db.CreateTask(&ItemModel{Title: "Existing", Description: "Local"}); err != nil {
		t.Fatalf("failed to seed existing task: %v", err)
	}

	now := time.Now()
	filePath := writeTestImportBundle(t, ExportBundle{
		Version:    1,
		ExportedAt: now,
		Tasks: []TaskDTO{
			{
				ID:          "3",
				Title:       "Imported Replacement",
				Description: "Imported Desc",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	})

	cmd := NewImportCmd(db)
	cmd.SetArgs([]string{"--file", filePath, "--mode", "replace"})
	cmd.SetIn(strings.NewReader("y\n"))
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("replace import failed: %v", err)
	}

	tasks, err := db.ListTasks()
	if err != nil {
		t.Fatalf("failed to list tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Imported Replacement" {
		t.Fatalf("expected imported replacement task, got %+v", tasks)
	}
	if !strings.Contains(buf.String(), "Import complete: created=1 updated=0 unchanged=0 skipped=0 conflicted=0") {
		t.Fatalf("expected completion output, got %q", buf.String())
	}
}

// ============================================ Integration Tests ============================================

func TestIntegration_AddAndList(t *testing.T) {
	db := NewMockModel()

	// Add task
	addCmd := NewAddCmd(db)
	addCmd.SetArgs([]string{"-t", "Integration Task", "-d", "Integration Desc"})
	err := addCmd.Execute()
	if err != nil {
		t.Errorf("add command failed: %v", err)
	}

	// List tasks
	listCmd := NewListCmd(db)
	buf := &bytes.Buffer{}
	listCmd.SetOut(buf)
	err = listCmd.Execute()
	if err != nil {
		t.Errorf("list command failed: %v", err)
	}

	if !strings.Contains(buf.String(), "Integration Task") {
		t.Errorf("expected task in list output")
	}
}

func TestIntegration_AddCompleteDelete(t *testing.T) {
	db := NewMockModel()

	// Add task
	addCmd := NewAddCmd(db)
	addCmd.SetArgs([]string{"-t", "Integration Task", "-d", "Integration Desc"})
	err := addCmd.Execute()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Complete task
	completeCmd := CompleteTaskCmd(db)
	completeCmd.SetArgs([]string{"1"})
	err = completeCmd.Execute()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	task, _ := db.GetTaskByID(1)
	if !task.Completed {
		t.Errorf("expected task to be completed")
	}

	// Delete task
	deleteCmd := DeleteTaskCmd(db)
	deleteCmd.SetArgs([]string{"1"})
	stdin := strings.NewReader("y\n")
	deleteCmd.SetIn(stdin)
	err = deleteCmd.Execute()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tasks, _ := db.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("expected no tasks after deletion")
	}
}

// ============================================ P-005 / P-010 regressions ============================================

func TestDeleteTaskCmd_MissingTaskFailsWithoutPrompt(t *testing.T) {
	db := NewMockModel()
	cmd := DeleteTaskCmd(db)
	cmd.SetArgs([]string{"999"})
	cmd.SetIn(strings.NewReader("y\n"))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil || !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
	if strings.Contains(out.String(), "deleted") || strings.Contains(out.String(), "[y/N]") {
		t.Fatalf("expected no prompt or success message, got %q", out.String())
	}
}

func TestDeleteTaskCmd_AnswerHandling(t *testing.T) {
	cases := []struct {
		input       string
		wantDeleted bool
	}{
		{"y\n", true},
		{"YES\n", true},
		{"Yes\n", true},
		{"y", true}, // no trailing newline
		{"y nope\n", false},
		{"n\n", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.input), func(t *testing.T) {
			db := NewMockModel()
			if err := db.CreateTask(&ItemModel{Title: "Task", Description: "Desc"}); err != nil {
				t.Fatalf("setup failed: %v", err)
			}
			cmd := DeleteTaskCmd(db)
			cmd.SetArgs([]string{"1"})
			cmd.SetIn(strings.NewReader(tc.input))
			cmd.SetOut(&bytes.Buffer{})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tasks, _ := db.ListTasks()
			if deleted := len(tasks) == 0; deleted != tc.wantDeleted {
				t.Fatalf("input %q: deleted=%v, want %v", tc.input, deleted, tc.wantDeleted)
			}
		})
	}
}

func TestConfirm_EndOfInputMeansNo(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})
	ok, err := Confirm(cmd, "? ")
	if ok || err != nil {
		t.Fatalf("expected (false, nil) at end of input, got (%v, %v)", ok, err)
	}
}

func TestRootCmd_HelpVersionAndCompletionDoNotCreateDatabase(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--version"}, {"help"}, {"completion", "bash"}, {"add", "--help"}, {"list", "--bogus"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "munus.db")
			db := NewDeferredDatabase(path)
			root := NewRootCmd(db)
			root.Version = "1.2.3"
			root.SetArgs(args)
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			_ = root.Execute()

			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("expected no database file for %v, stat err=%v", args, err)
			}
		})
	}
}

func TestRootCmd_VersionFlag(t *testing.T) {
	root := NewRootCmd(NewDeferredDatabase(filepath.Join(t.TempDir(), "munus.db")))
	root.Version = "v9.9.9"
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(out.String(), "v9.9.9") {
		t.Fatalf("expected version in output, got %q", out.String())
	}
}

func TestRootCmd_SubcommandOpensDeferredDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "munus.db")
	db := NewDeferredDatabase(path)
	defer func() { _ = db.Close() }()
	root := NewRootCmd(db)
	root.SetArgs([]string{"add", "-t", "Title", "-d", "Desc"})
	root.SetOut(&bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	tasks, err := db.ListTasks()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("expected 1 task in opened database, got %v (err %v)", tasks, err)
	}
}

func TestRootCmd_ReportsDatabaseOpenFailure(t *testing.T) {
	db := NewDeferredDatabase(filepath.Join(t.TempDir(), "missing", "munus.db"))
	root := NewRootCmd(db)
	root.SetArgs([]string{"list"})
	root.SetOut(&bytes.Buffer{})
	errOut := &bytes.Buffer{}
	root.SetErr(errOut)
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "failed to initialize database") {
		t.Fatalf("expected database initialization error, got %v", err)
	}
	if strings.Contains(errOut.String(), "Usage:") {
		t.Fatalf("expected no usage text for database errors, got %q", errOut.String())
	}
}

func TestImportCmdRejectsInvalidOptionValues(t *testing.T) {
	file := writeTestImportBundle(t, ExportBundle{Version: 1})
	for _, args := range [][]string{
		{"--mode", "bogus"},
		{"--on-conflict", "bogus"},
		{"--id-strategy", "ict"},
	} {
		cmd := NewImportCmd(NewMockModel())
		cmd.SetArgs(append([]string{"--file", file, "--dry-run"}, args...))
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid") {
			t.Errorf("expected invalid-option error for %v, got %v", args, err)
		}
	}
}
