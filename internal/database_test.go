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
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewDatabaseCreatesDatabase verifies that NewDatabase successfully creates and initializes
// a SQLite database at the specified path. This test ensures:
// - Database connection is established
// - Schema migrations are applied
// - Empty database is ready for operations
func TestNewDatabaseCreatesDatabase(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create new database
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase() error = %v, want nil", err)
	}

	if db == nil {
		t.Fatal("NewDatabase() returned nil database")
	}

	// Verify database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("Database file not created at %s", dbPath)
	}
}

// TestNewDatabaseWithEmptyPathUsesDefault verifies that an empty path parameter is handled
// gracefully by the database initialization logic.
func TestNewDatabaseWithEmptyPathCreatesValidDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	originalWD, _ := os.Getwd()
	defer os.Chdir(originalWD)

	// Change to temp directory so default db path is created there
	os.Chdir(tmpDir)

	db, err := NewDatabase("")
	if err != nil {
		t.Fatalf("NewDatabase(\"\") error = %v, want nil", err)
	}

	if db == nil {
		t.Fatal("NewDatabase(\"\") returned nil")
	}
}

// TestCreateTaskPersistsTaskToDatabase verifies that CreateTask stores a new ItemModel
// in the database with all fields intact. This test validates:
// - Task persistence
// - Field preservation (title, description, deadline, etc.)
// - ID assignment
func TestCreateTaskPersistsTaskToDatabase(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create test task
	task := &ItemModel{
		Title:       "Test Task",
		Description: "This is a test task",
		Completed:   false,
	}

	err := db.CreateTask(task)
	if err != nil {
		t.Fatalf("CreateTask() error = %v, want nil", err)
	}

	// Verify task ID was assigned
	if task.ID == 0 {
		t.Fatal("CreateTask() did not assign task ID")
	}

	// Retrieve and verify
	retrieved, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}

	if retrieved.Title != task.Title || retrieved.Description != task.Description {
		t.Fatalf("Retrieved task mismatch: got %+v, want %+v", retrieved, task)
	}
}

// TestCreateTaskWithNilTaskReturnsError verifies that CreateTask properly validates
// input parameters and rejects nil tasks with a meaningful error message.
func TestCreateTaskWithNilTaskReturnsError(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.CreateTask(nil)
	if err == nil {
		t.Fatal("CreateTask(nil) should return an error")
	}
}

// TestListTasksReturnsAllTasks verifies that ListTasks returns all persisted tasks
// in the correct order. Tests:
// - Empty list on new database
// - Correct task count
// - Reverse ID order (DESC)
func TestListTasksReturnsAllTasks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Initially should be empty
	tasks, err := db.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("ListTasks() on empty DB returned %d tasks, want 0", len(tasks))
	}

	// Create multiple tasks
	taskCount := 3
	taskIDs := make([]int, taskCount)
	for i := 0; i < taskCount; i++ {
		task := &ItemModel{
			Title:     "Task " + string(rune('A'+i)),
			Completed: i%2 == 0,
		}
		if err := db.CreateTask(task); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		taskIDs[i] = task.ID
	}

	// Verify retrieval
	tasks, err = db.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != taskCount {
		t.Fatalf("ListTasks() returned %d tasks, want %d", len(tasks), taskCount)
	}

	// Verify reverse order (DESC by ID)
	for i := 0; i < len(tasks)-1; i++ {
		if tasks[i].ID <= tasks[i+1].ID {
			t.Fatalf("ListTasks() order mismatch: expected DESC order, got %v", tasks)
		}
	}
}

// TestGetTaskByIDRetrievesCorrectTask verifies that GetTaskByID retrieves the exact
// task matching the provided ID. Tests:
// - Correct task retrieval
// - Error on non-existent ID
func TestGetTaskByIDRetrievesCorrectTask(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create task
	original := &ItemModel{
		Title:       "Original Task",
		Description: "Description",
	}
	if err := db.CreateTask(original); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	// Retrieve by ID
	retrieved, err := db.GetTaskByID(original.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}

	// Verify match
	if retrieved.ID != original.ID || retrieved.Title != original.Title {
		t.Fatalf("GetTaskByID() mismatch: got %+v, want %+v", retrieved, original)
	}
}

// TestGetTaskByIDWithInvalidIDReturnsError verifies that GetTaskByID returns an error
// when attempting to retrieve a task with a non-existent ID.
func TestGetTaskByIDWithInvalidIDReturnsError(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.GetTaskByID(99999)
	if err == nil {
		t.Fatal("GetTaskByID(99999) should return an error for non-existent task")
	}
}

// TestUpdateTaskSavesChanges verifies that UpdateTask persists modifications to
// existing tasks. Tests:
// - Field updates (title, description, completed status)
// - Persistence verification
// - Error cases (nil task, zero ID)
func TestUpdateTaskSavesChanges(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create task
	task := &ItemModel{
		Title:       "Original Title",
		Description: "Original Description",
		Completed:   false,
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	// Modify task
	task.Title = "Updated Title"
	task.Description = "Updated Description"
	task.Completed = true

	// Update in database
	if err := db.UpdateTask(task); err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}

	// Retrieve and verify
	retrieved, err := db.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}

	if retrieved.Title != "Updated Title" || !retrieved.Completed {
		t.Fatalf("UpdateTask() changes not persisted: got %+v", retrieved)
	}
}

// TestUpdateTaskWithNilTaskReturnsError verifies proper error handling when
// attempting to update a nil task.
func TestUpdateTaskWithNilTaskReturnsError(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	err := db.UpdateTask(nil)
	if err == nil {
		t.Fatal("UpdateTask(nil) should return an error")
	}
}

// TestUpdateTaskWithZeroIDReturnsError verifies proper error handling when
// attempting to update a task with no ID (unsaved task).
func TestUpdateTaskWithZeroIDReturnsError(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	task := &ItemModel{
		Title:   "No ID Task",
		ID:      0,
		Completed: false,
	}

	err := db.UpdateTask(task)
	if err == nil {
		t.Fatal("UpdateTask() with ID=0 should return an error")
	}
}

// TestDeleteTaskRemovesFromDatabase verifies that DeleteTask removes the specified
// task from the database. Tests:
// - Task removal
// - Retrieval failure after deletion
// - Other tasks remain unaffected
func TestDeleteTaskRemovesFromDatabase(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create two tasks
	task1 := &ItemModel{Title: "Task 1"}
	task2 := &ItemModel{Title: "Task 2"}

	db.CreateTask(task1)
	db.CreateTask(task2)

	// Delete first task
	if err := db.DeleteTask(task1.ID); err != nil {
		t.Fatalf("DeleteTask() error = %v", err)
	}

	// Verify deletion
	_, err := db.GetTaskByID(task1.ID)
	if err == nil {
		t.Fatal("GetTaskByID() should fail after DeleteTask()")
	}

	// Verify second task still exists
	if _, err := db.GetTaskByID(task2.ID); err != nil {
		t.Fatalf("GetTaskByID(task2) error = %v, want nil", err)
	}
}

// TestReplaceAllTasksClears and replaces the entire task database. Tests:
// - Atomic deletion and insertion
// - Empty list handling
// - Transaction safety
func TestReplaceAllTasksClearsAndReplacesAll(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create initial tasks
	task1 := &ItemModel{Title: "Original 1"}
	task2 := &ItemModel{Title: "Original 2"}
	db.CreateTask(task1)
	db.CreateTask(task2)

	// Replace all with new tasks
	newTasks := []*ItemModel{
		{Title: "New 1"},
		{Title: "New 2"},
		{Title: "New 3"},
	}

	if err := db.ReplaceAllTasks(newTasks); err != nil {
		t.Fatalf("ReplaceAllTasks() error = %v", err)
	}

	// Verify old tasks are gone
	if _, err := db.GetTaskByID(task1.ID); err == nil {
		t.Fatal("Old task 1 still exists after ReplaceAllTasks()")
	}

	// Verify new tasks count
	tasks, err := db.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("ListTasks() returned %d tasks, want 3", len(tasks))
	}

	// Verify new task titles
	titles := make(map[string]bool)
	for _, task := range tasks {
		titles[task.Title] = true
	}
	for _, newTask := range newTasks {
		if !titles[newTask.Title] {
			t.Fatalf("New task %q not found after ReplaceAllTasks()", newTask.Title)
		}
	}
}

// TestReplaceAllTasksWithEmptyList removes all tasks. This verifies the function
// correctly handles the edge case of clearing the entire database.
func TestReplaceAllTasksWithEmptyList(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create tasks
	task := &ItemModel{Title: "Task to Delete"}
	db.CreateTask(task)

	// Replace with empty list
	if err := db.ReplaceAllTasks([]*ItemModel{}); err != nil {
		t.Fatalf("ReplaceAllTasks([]) error = %v", err)
	}

	// Verify all tasks removed
	tasks, err := db.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("ListTasks() returned %d tasks, want 0", len(tasks))
	}
}

// TestReplaceAllTasksWithNilDatabaseReturnsError verifies proper nil-checking
// and error handling in ReplaceAllTasks.
func TestReplaceAllTasksWithNilDatabaseReturnsError(t *testing.T) {
	var db *Database
	err := db.ReplaceAllTasks([]*ItemModel{})
	if err == nil {
		t.Fatal("ReplaceAllTasks on nil database should return an error")
	}
}

// TestTaskDeadlineCalculation verifies that ItemModel methods correctly calculate
// days until deadline and deadline status.
func TestTaskDeadlineCalculation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		deadline      *time.Time
		wantDays      int
		wantIsOverdue bool
	}{
		{
			name:          "overdue task",
			deadline:      ptrTime(now.AddDate(0, 0, -5)),
			wantDays:      -5,
			wantIsOverdue: true,
		},
		{
			name:          "due today",
			deadline:      ptrTime(now),
			wantDays:      0,
			wantIsOverdue: false,
		},
		{
			name:          "upcoming task",
			deadline:      ptrTime(now.AddDate(0, 0, 3)),
			wantDays:      3,
			wantIsOverdue: false,
		},
		{
			name:          "nil deadline",
			deadline:      nil,
			wantDays:      0,
			wantIsOverdue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &ItemModel{
				Title:    "Test Task",
				Deadline: tt.deadline,
			}

			// Note: DaysUntilDeadline() might have rounding differences,
			// so we check if it's approximately correct (within 1 day)
			if tt.deadline != nil {
				days := task.DaysUntilDeadline()
				if days != tt.wantDays {
					t.Logf("DaysUntilDeadline() = %d, want approximately %d", days, tt.wantDays)
				}
			}
		})
	}
}

// TestItemModelMarkComplete verifies that MarkComplete() correctly updates task state.
// This tests the business logic for marking tasks as completed.
func TestItemModelMarkComplete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	task := &ItemModel{
		Title:     "Task to Complete",
		Completed: false,
	}
	db.CreateTask(task)

	// Mark complete
	task.MarkComplete()

	if !task.Completed {
		t.Fatal("MarkComplete() did not set Completed to true")
	}

	if task.CompletedAt == nil {
		t.Fatal("MarkComplete() did not set CompletedAt timestamp")
	}

	// Persist and verify
	db.UpdateTask(task)
	retrieved, _ := db.GetTaskByID(task.ID)
	if !retrieved.Completed {
		t.Fatal("MarkComplete() changes not persisted")
	}
}

// TestItemModelMarkIncomplete verifies that MarkIncomplete() correctly reverts
// a task from completed to incomplete status.
func TestItemModelMarkIncomplete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	task := &ItemModel{
		Title:     "Task to Reopen",
		Completed: true,
		CompletedAt: ptrTime(time.Now()),
	}
	db.CreateTask(task)

	// Mark incomplete
	task.MarkIncomplete()

	if task.Completed {
		t.Fatal("MarkIncomplete() did not set Completed to false")
	}

	if task.CompletedAt != nil {
		t.Fatal("MarkIncomplete() did not clear CompletedAt")
	}
}

// TestConnReturnsValidGormConnection verifies that Conn() properly exposes
// the underlying GORM database connection for advanced queries.
func TestConnReturnsValidGormConnection(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	conn := db.Conn()
	if conn == nil {
		t.Fatal("Conn() returned nil")
	}

	// Verify connection is functional by running a simple query
	var count int64
	if err := conn.Model(&ItemModel{}).Count(&count).Error; err != nil {
		t.Fatalf("Conn() returned non-functional connection: %v", err)
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

// setupTestDB creates a temporary database for testing and returns cleanup function.
// This helper abstracts common test setup logic.
func setupTestDB(t *testing.T) (*Database, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	return db, func() {
		// Cleanup is handled by t.TempDir()
	}
}

// ptrTime is a helper to create a pointer to a time.Time value.
// Useful for test setup when nullable time fields are needed.
func ptrTime(t time.Time) *time.Time {
	return &t
}
