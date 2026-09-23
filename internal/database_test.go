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
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestNewDatabaseCreatesDatabase verifies that NewDatabase successfully creates and initializes
//
//	an SQLite database at the specified path. This test ensures:
//
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

	defer func(db *Database) {
		err := db.Close()
		if err != nil {
			t.Fatalf("Failed to close database: %v", err)
		}
	}(db)
}

// TestNewDatabaseWithEmptyPathUsesDefault verifies that an empty path parameter is handled
// gracefully by the database initialization logic.
func TestNewDatabaseWithEmptyPathCreatesValidDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	originalWD, _ := os.Getwd()
	defer func(dir string) {
		err := os.Chdir(dir)
		if err != nil {
			t.Fatalf("Failed to change directory back to original: %v", err)
		}
	}(originalWD)

	// Change to temp directory so default db path is created there
	err := os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	db, err := NewDatabase("")
	if err != nil {
		t.Fatalf("NewDatabase(\"\") error = %v, want nil", err)
	}

	if db == nil {
		t.Fatal("NewDatabase(\"\") returned nil")
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "munus.db")); err != nil {
		if os.IsNotExist(err) {
			t.Fatal("NewDatabase(\"\") did not create the default database file")
		}
		t.Fatalf("Failed to stat default database file: %v", err)
	}

	defer func(db *Database) {
		err := db.Close()
		if err != nil {
			t.Fatalf("Failed to close database: %v", err)
		}
	}(db)
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
// - Empty list on the new database
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
	for i := range taskCount {
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
		Title:     "No ID Task",
		ID:        0,
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

	err := db.CreateTask(task1)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	err = db.CreateTask(task2)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Delete first task
	if err := db.DeleteTask(task1.ID); err != nil {
		t.Fatalf("DeleteTask() error = %v", err)
	}

	// Verify deletion
	_, err = db.GetTaskByID(task1.ID)
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
	err := db.CreateTask(task1)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	err = db.CreateTask(task2)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

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
	err := db.CreateTask(task)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Replace it with empty list
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

	defer func(db *Database) {
		err := db.Close()
		if err != nil {
			t.Fatalf("Failed to close database: %v", err)
		}
	}(db)
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
			name: "overdue task",
			// Fixed durations (not calendar days) keep this stable across DST changes.
			deadline:      new(now.Add(-5*24*time.Hour - time.Hour)),
			wantDays:      -5,
			wantIsOverdue: true,
		},
		{
			name:          "due today",
			deadline:      new(now.Add(time.Hour)),
			wantDays:      0,
			wantIsOverdue: false,
		},
		{
			// An extra hour keeps the truncated day count stable while the test runs.
			name:          "upcoming task",
			deadline:      new(now.Add(3*24*time.Hour + time.Hour)),
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

			if tt.deadline != nil {
				if days := task.DaysUntilDeadline(); days != tt.wantDays {
					t.Errorf("DaysUntilDeadline() = %d, want %d", days, tt.wantDays)
				}
			} else if days := task.DaysUntilDeadline(); days != -1 {
				t.Errorf("DaysUntilDeadline() with no deadline = %d, want -1", days)
			}
			if got := task.IsOverdue(); got != tt.wantIsOverdue {
				t.Errorf("IsOverdue() = %v, want %v", got, tt.wantIsOverdue)
			}
		})
	}
}

// TestItemModelMarkComplete verifies that MarkComplete() correctly updates the task state.
// This tests the business logic for marking tasks as completed.
func TestItemModelMarkComplete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	task := &ItemModel{
		Title:     "Task to Complete",
		Completed: false,
	}
	err := db.CreateTask(task)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Mark complete
	task.MarkComplete()

	if !task.Completed {
		t.Fatal("MarkComplete() did not set Completed to true")
	}

	if task.CompletedAt == nil {
		t.Fatal("MarkComplete() did not set CompletedAt timestamp")
	}

	// Persist and verify
	err = db.UpdateTask(task)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
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
		Title:       "Task to Reopen",
		Completed:   true,
		CompletedAt: new(time.Now()),
	}
	err := db.CreateTask(task)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

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

// setupTestDB creates a temporary database for testing and returns a cleanup function.
// This helper abstracts common test setup logic.
func setupTestDB(t *testing.T) (*Database, func()) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	return db, func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Failed to close database: %v", err)
		}
	}
}

func TestNewDatabaseCreatesOwnerOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	path := filepath.Join(t.TempDir(), "private.db")
	db, err := NewDatabase(path)
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected new database to be 0600, got %v", perm)
	}
	if got := db.DatabasePath(); got != path {
		t.Fatalf("DatabasePath() = %q, want %q", got, path)
	}
}

func TestNewDatabaseLeavesExistingFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	path := filepath.Join(t.TempDir(), "shared.db")
	if err := os.WriteFile(path, nil, 0o640); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	db, err := NewDatabase(path)
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	defer func() { _ = db.Close() }()
	info, _ := os.Stat(path)
	if perm := info.Mode().Perm(); perm != 0o640 {
		t.Fatalf("expected existing permissions to be kept, got %v", perm)
	}
}

func TestDeferredDatabaseOpensOnDemand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deferred.db")
	db := NewDeferredDatabase(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no database file before Open, stat err=%v", err)
	}
	if _, err := db.ListTasks(); !errors.Is(err, errDatabaseNotOpen) {
		t.Fatalf("expected errDatabaseNotOpen before Open, got %v", err)
	}
	if err := db.CreateTask(&ItemModel{Title: "x"}); !errors.Is(err, errDatabaseNotOpen) {
		t.Fatalf("expected errDatabaseNotOpen before Open, got %v", err)
	}

	if err := db.Open(); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.Open(); err != nil {
		t.Fatalf("second Open should be a no-op, got %v", err)
	}
	if err := db.CreateTask(&ItemModel{Title: "x", Description: "y"}); err != nil {
		t.Fatalf("CreateTask after Open failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected database file after Open: %v", err)
	}
}

func TestDeferredDatabaseOpenReportsErrors(t *testing.T) {
	db := NewDeferredDatabase(filepath.Join(t.TempDir(), "missing-dir", "x.db"))
	if err := db.Open(); err == nil {
		t.Fatal("expected error opening database in a missing directory")
	}
	var nilDB *Database
	if err := nilDB.Open(); err == nil {
		t.Fatal("expected error for nil database")
	}
}

func TestDeleteTaskReportsMissingTask(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	if err := db.DeleteTask(999); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestReplaceAllTasksRestoresExplicitIDsBeforeAssigningNewOnes(t *testing.T) {
	db, err := NewDatabase(filepath.Join(t.TempDir(), "ids.db"))
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// The auto-assigned row comes first in the slice; it must still not take ID 1 or 2.
	tasks := []*ItemModel{{Title: "auto"}, {ID: 2, Title: "two"}, {ID: 1, Title: "one"}}
	if err := db.ReplaceAllTasks(tasks); err != nil {
		t.Fatalf("ReplaceAllTasks failed: %v", err)
	}
	one, err := db.GetTaskByID(1)
	if err != nil || one.Title != "one" {
		t.Fatalf("expected task 1 restored, got %+v (err %v)", one, err)
	}
	two, err := db.GetTaskByID(2)
	if err != nil || two.Title != "two" {
		t.Fatalf("expected task 2 restored, got %+v (err %v)", two, err)
	}
	if tasks[0].ID <= 2 {
		t.Fatalf("expected auto task to get a fresh ID, got %d", tasks[0].ID)
	}
}

func TestDatabaseDoesNotLog(t *testing.T) {
	var buf bytes.Buffer
	previous := gormLogWriter
	gormLogWriter = &buf
	t.Cleanup(func() { gormLogWriter = previous })

	db, err := NewDatabase(filepath.Join(t.TempDir(), "log.db"))
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.GetTaskByID(999); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no GORM log output, got %q", buf.String())
	}
}
