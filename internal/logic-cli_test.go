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
	"strings"
	"testing"
	"time"
)

// ============================================ MockDatabase ============================================

type MockDatabase struct {
	tasks              map[int]*ItemModel
	nextID             int
	createTaskError    error
	listTasksError     error
	getTaskByIDError   error
	deleteTaskError    error
	updateTaskError    error
}

func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		tasks:  make(map[int]*ItemModel),
		nextID: 1,
	}
}

func (m *MockDatabase) CreateTask(task *ItemModel) error {
	if m.createTaskError != nil {
		return m.createTaskError
	}
	task.ID = m.nextID
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	m.tasks[m.nextID] = task
	m.nextID++
	return nil
}

func (m *MockDatabase) ListTasks() ([]*ItemModel, error) {
	if m.listTasksError != nil {
		return nil, m.listTasksError
	}
	tasks := make([]*ItemModel, 0, len(m.tasks))
	for i := 1; i < m.nextID; i++ {
		if task, ok := m.tasks[i]; ok {
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func (m *MockDatabase) GetTaskByID(id int) (*ItemModel, error) {
	if m.getTaskByIDError != nil {
		return nil, m.getTaskByIDError
	}
	return m.tasks[id], nil
}

func (m *MockDatabase) DeleteTask(id int) error {
	if m.deleteTaskError != nil {
		return m.deleteTaskError
	}
	delete(m.tasks, id)
	return nil
}

func (m *MockDatabase) UpdateTask(task *ItemModel) error {
	if m.updateTaskError != nil {
		return m.updateTaskError
	}
	task.UpdatedAt = time.Now()
	m.tasks[task.ID] = task
	return nil
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

func TestPrintList_SingleTask(t *testing.T) {
	buf := &bytes.Buffer{}
	deadline := time.Date(2025, 12, 25, 10, 0, 0, 0, time.UTC)
	task := &ItemModel{
		ID:          1,
		Title:       "Test Task",
		Description: "Test Description",
		Deadline:    &deadline,
		Completed:   false,
	}
	PrintList(buf, []*ItemModel{task})

	output := buf.String()
	if !strings.Contains(output, "Test Task") {
		t.Errorf("output missing task title: %s", output)
	}
	if !strings.Contains(output, "Test Description") {
		t.Errorf("output missing task description: %s", output)
	}
	if !strings.Contains(output, "TODO") {
		t.Errorf("output missing status: %s", output)
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
	db := NewMockDatabase()
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
	db := NewMockDatabase()
	cmd := NewAddCmd(db)

	cmd.SetArgs([]string{"-d", "Test Description"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for missing title")
	}
}

func TestAddCmd_MissingDescription(t *testing.T) {
	db := NewMockDatabase()
	cmd := NewAddCmd(db)

	cmd.SetArgs([]string{"-t", "Test Task"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for missing description")
	}
}

func TestAddCmd_TitleExceedsMaxLength(t *testing.T) {
	db := NewMockDatabase()
	cmd := NewAddCmd(db)

	longTitle := strings.Repeat("a", MaxTitleLength+1)
	cmd.SetArgs([]string{"-t", longTitle, "-d", "Description"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for title exceeding max length")
	}
}

func TestAddCmd_DescriptionExceedsMaxLength(t *testing.T) {
	db := NewMockDatabase()
	cmd := NewAddCmd(db)

	longDesc := strings.Repeat("a", MaxDescriptionLength+1)
	cmd.SetArgs([]string{"-t", "Title", "-d", longDesc})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for description exceeding max length")
	}
}

func TestAddCmd_WithDeadline(t *testing.T) {
	db := NewMockDatabase()
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
	db := NewMockDatabase()
	cmd := NewAddCmd(db)

	cmd.SetArgs([]string{"-t", "Task", "-d", "Desc", "-n", "invalid-deadline"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for invalid deadline")
	}
}

// ============================================ NewListCmd Tests ============================================

func TestListCmd_NoTasks(t *testing.T) {
	db := NewMockDatabase()
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
	db := NewMockDatabase()

	// Add some tasks
	db.CreateTask(&ItemModel{
		Title:       "Task 1",
		Description: "Desc 1",
	})
	db.CreateTask(&ItemModel{
		Title:       "Task 2",
		Description: "Desc 2",
	})

	cmd := NewListCmd(db)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Task 1") || !strings.Contains(output, "Task 2") {
		t.Errorf("expected tasks in output, got %s", output)
	}
}

func TestListCmd_Error(t *testing.T) {
	db := NewMockDatabase()
	db.listTasksError = errors.New("database error")

	cmd := NewListCmd(db)
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error from database")
	}
}

// ============================================ DeleteTaskCmd Tests ============================================

func TestDeleteTaskCmd_InvalidID(t *testing.T) {
	db := NewMockDatabase()
	cmd := DeleteTaskCmd(db)

	cmd.SetArgs([]string{"invalid"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for invalid task ID")
	}
}

func TestDeleteTaskCmd_NegativeID(t *testing.T) {
	db := NewMockDatabase()
	cmd := DeleteTaskCmd(db)

	cmd.SetArgs([]string{"-5"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for negative task ID")
	}
}

func TestDeleteTaskCmd_UserCancels(t *testing.T) {
	db := NewMockDatabase()
	db.CreateTask(&ItemModel{
		Title:       "Task",
		Description: "Desc",
	})

	cmd := DeleteTaskCmd(db)
	cmd.SetArgs([]string{"1"})

	// Simulate user input: "n" (cancel)
	stdin := strings.NewReader("n\n")
	cmd.SetIn(stdin)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := cmd.Execute()
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
	db := NewMockDatabase()
	db.CreateTask(&ItemModel{
		Title:       "Task",
		Description: "Desc",
	})

	cmd := DeleteTaskCmd(db)
	cmd.SetArgs([]string{"1"})

	// Simulate user input: "y" (confirm)
	stdin := strings.NewReader("y\n")
	cmd.SetIn(stdin)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := cmd.Execute()
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

func TestDeleteTaskCmd_DatabaseError(t *testing.T) {
	db := NewMockDatabase()
	db.deleteTaskError = errors.New("db error")

	cmd := DeleteTaskCmd(db)
	cmd.SetArgs([]string{"1"})

	stdin := strings.NewReader("y\n")
	cmd.SetIn(stdin)

	err := cmd.Execute()
	if err == nil {
		t.Errorf("expected error from database")
	}
}

// ============================================ CompleteTaskCmd Tests ============================================

func TestCompleteTaskCmd_InvalidID(t *testing.T) {
	db := NewMockDatabase()
	cmd := CompleteTaskCmd(db)

	cmd.SetArgs([]string{"invalid"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for invalid task ID")
	}
}

func TestCompleteTaskCmd_TaskNotFound(t *testing.T) {
	db := NewMockDatabase()
	cmd := CompleteTaskCmd(db)

	cmd.SetArgs([]string{"999"})
	err := cmd.Execute()

	if err == nil {
		t.Errorf("expected error for non-existent task")
	}
}

func TestCompleteTaskCmd_CompleteTask(t *testing.T) {
	db := NewMockDatabase()
	db.CreateTask(&ItemModel{
		Title:       "Task",
		Description: "Desc",
		Completed:   false,
	})

	cmd := CompleteTaskCmd(db)
	cmd.SetArgs([]string{"1"})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := cmd.Execute()
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
	db := NewMockDatabase()
	now := time.Now()
	db.CreateTask(&ItemModel{
		Title:       "Task",
		Description: "Desc",
		Completed:   true,
		CompletedAt: &now,
	})

	cmd := CompleteTaskCmd(db)
	cmd.SetArgs([]string{"1", "--undo"})

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	err := cmd.Execute()
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

func TestCompleteTaskCmd_GetTaskError(t *testing.T) {
	db := NewMockDatabase()
	db.getTaskByIDError = errors.New("db error")

	cmd := CompleteTaskCmd(db)
	cmd.SetArgs([]string{"1"})

	err := cmd.Execute()
	if err == nil {
		t.Errorf("expected error from database")
	}
}

func TestCompleteTaskCmd_UpdateError(t *testing.T) {
	db := NewMockDatabase()
	db.CreateTask(&ItemModel{
		Title:       "Task",
		Description: "Desc",
	})
	db.updateTaskError = errors.New("db error")

	cmd := CompleteTaskCmd(db)
	cmd.SetArgs([]string{"1"})

	err := cmd.Execute()
	if err == nil {
		t.Errorf("expected error from database")
	}
}

// ============================================ Confirm Tests ============================================

func TestConfirm_UserConfirmsWithY(t *testing.T) {
	cmd := NewRootCmd(NewMockDatabase())
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
	cmd := NewRootCmd(NewMockDatabase())
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
	cmd := NewRootCmd(NewMockDatabase())
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
	cmd := NewRootCmd(NewMockDatabase())
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
	cmd := NewRootCmd(NewMockDatabase())
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
	db := NewMockDatabase()
	cmd := NewRootCmd(db)

	subcommands := cmd.Commands()
	expectedCmds := map[string]bool{
		"add":    false,
		"list":   false,
		"delete": false,
		"complete": false,
		"export": false,
		"import": false,
	}

	for _, subcmd := range subcommands {
		expectedCmds[subcmd.Name()] = true
	}

	for cmdName, found := range expectedCmds {
		if !found {
			t.Errorf("expected subcommand %q not found", cmdName)
		}
	}
}

// ============================================ Integration Tests ============================================

func TestIntegration_AddAndList(t *testing.T) {
	db := NewMockDatabase()

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
	db := NewMockDatabase()

	// Add task
	addCmd := NewAddCmd(db)
	addCmd.SetArgs([]string{"-t", "Integration Task", "-d", "Integration Desc"})
	addCmd.Execute()

	// Complete task
	completeCmd := CompleteTaskCmd(db)
	completeCmd.SetArgs([]string{"1"})
	completeCmd.Execute()

	task, _ := db.GetTaskByID(1)
	if !task.Completed {
		t.Errorf("expected task to be completed")
	}

	// Delete task
	deleteCmd := DeleteTaskCmd(db)
	deleteCmd.SetArgs([]string{"1"})
	stdin := strings.NewReader("y\n")
	deleteCmd.SetIn(stdin)
	deleteCmd.Execute()

	tasks, _ := db.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("expected no tasks after deletion")
	}
}
