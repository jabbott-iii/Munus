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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
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

	err := db.CreateTask(t.Context(), task)
	if err != nil {
		t.Fatalf("CreateTask() error = %v, want nil", err)
	}

	// Verify task ID was assigned
	if task.ID == 0 {
		t.Fatal("CreateTask() did not assign task ID")
	}

	// Retrieve and verify
	retrieved, err := db.GetTaskByID(t.Context(), task.ID)
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

	err := db.CreateTask(t.Context(), nil)
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
	tasks, err := db.ListTasks(t.Context())
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
		if err := db.CreateTask(t.Context(), task); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		taskIDs[i] = task.ID
	}

	// Verify retrieval
	tasks, err = db.ListTasks(t.Context())
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
	if err := db.CreateTask(t.Context(), original); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	// Retrieve by ID
	retrieved, err := db.GetTaskByID(t.Context(), original.ID)
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

	_, err := db.GetTaskByID(t.Context(), 99999)
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
	if err := db.CreateTask(t.Context(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	// Modify task
	task.Title = "Updated Title"
	task.Description = "Updated Description"
	task.Completed = true

	// Update in database
	if err := db.UpdateTask(t.Context(), task); err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}

	// Retrieve and verify
	retrieved, err := db.GetTaskByID(t.Context(), task.ID)
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

	err := db.UpdateTask(t.Context(), nil)
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

	err := db.UpdateTask(t.Context(), task)
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

	err := db.CreateTask(t.Context(), task1)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	err = db.CreateTask(t.Context(), task2)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Delete first task
	if err := db.DeleteTask(t.Context(), task1.ID); err != nil {
		t.Fatalf("DeleteTask() error = %v", err)
	}

	// Verify deletion
	_, err = db.GetTaskByID(t.Context(), task1.ID)
	if err == nil {
		t.Fatal("GetTaskByID() should fail after DeleteTask()")
	}

	// Verify second task still exists
	if _, err := db.GetTaskByID(t.Context(), task2.ID); err != nil {
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
	err := db.CreateTask(t.Context(), task1)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	err = db.CreateTask(t.Context(), task2)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Replace all with new tasks
	newTasks := []*ItemModel{
		{Title: "New 1"},
		{Title: "New 2"},
		{Title: "New 3"},
	}

	if err := db.ReplaceAllTasks(t.Context(), newTasks); err != nil {
		t.Fatalf("ReplaceAllTasks() error = %v", err)
	}

	// Verify old tasks are gone
	if _, err := db.GetTaskByID(t.Context(), task1.ID); err == nil {
		t.Fatal("Old task 1 still exists after ReplaceAllTasks()")
	}

	// Verify new tasks count
	tasks, err := db.ListTasks(t.Context())
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
	err := db.CreateTask(t.Context(), task)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Replace it with empty list
	if err := db.ReplaceAllTasks(t.Context(), []*ItemModel{}); err != nil {
		t.Fatalf("ReplaceAllTasks([]) error = %v", err)
	}

	// Verify all tasks removed
	tasks, err := db.ListTasks(t.Context())
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
	err := db.ReplaceAllTasks(t.Context(), []*ItemModel{})
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
	err := db.CreateTask(t.Context(), task)
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
	err = db.UpdateTask(t.Context(), task)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	retrieved, _ := db.GetTaskByID(t.Context(), task.ID)
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
	err := db.CreateTask(t.Context(), task)
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
	if got := db.path; got != path {
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
	if _, err := db.ListTasks(t.Context()); !errors.Is(err, errDatabaseNotOpen) {
		t.Fatalf("expected errDatabaseNotOpen before Open, got %v", err)
	}
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "x"}); !errors.Is(err, errDatabaseNotOpen) {
		t.Fatalf("expected errDatabaseNotOpen before Open, got %v", err)
	}

	if err := db.open(); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.open(); err != nil {
		t.Fatalf("second Open should be a no-op, got %v", err)
	}
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "x", Description: "y"}); err != nil {
		t.Fatalf("CreateTask after Open failed: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected database file after Open: %v", err)
	}
}

func TestDeferredDatabaseOpenReportsErrors(t *testing.T) {
	db := NewDeferredDatabase(filepath.Join(t.TempDir(), "missing-dir", "x.db"))
	if err := db.open(); err == nil {
		t.Fatal("expected error opening database in a missing directory")
	}
	var nilDB *Database
	if err := nilDB.open(); err == nil {
		t.Fatal("expected error for nil database")
	}
}

func TestDeleteTaskReportsMissingTask(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	if err := db.DeleteTask(t.Context(), 999); !errors.Is(err, ErrTaskNotFound) {
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
	if err := db.ReplaceAllTasks(t.Context(), tasks); err != nil {
		t.Fatalf("ReplaceAllTasks failed: %v", err)
	}
	one, err := db.GetTaskByID(t.Context(), 1)
	if err != nil || one.Title != "one" {
		t.Fatalf("expected task 1 restored, got %+v (err %v)", one, err)
	}
	two, err := db.GetTaskByID(t.Context(), 2)
	if err != nil || two.Title != "two" {
		t.Fatalf("expected task 2 restored, got %+v (err %v)", two, err)
	}
	if tasks[0].ID <= 2 {
		t.Fatalf("expected auto task to get a fresh ID, got %d", tasks[0].ID)
	}
}

func TestDatabaseDoesNotLog(t *testing.T) {
	var buf bytes.Buffer
	db, err := openDatabase(filepath.Join(t.TempDir(), "log.db"), &buf)
	if err != nil {
		t.Fatalf("openDatabase failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.GetTaskByID(t.Context(), 999); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no GORM log output, got %q", buf.String())
	}
}

// ============================== plan 2: status, tags, context ==============================

// v211Schema is the item_models table created by Munus v2.1.1.
const v211Schema = "CREATE TABLE `item_models` (`id` integer PRIMARY KEY AUTOINCREMENT,`title` text NOT NULL,`description` text,`deadline` datetime,`completed` numeric NOT NULL DEFAULT false,`completed_at` datetime,`created_at` datetime,`updated_at` datetime)"

func TestNewDatabaseMigratesV211Database(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	old, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open old db: %v", err)
	}
	for _, stmt := range []string{
		v211Schema,
		"INSERT INTO item_models (title, description, completed, completed_at) VALUES ('open', 'x', 0, NULL)",
		"INSERT INTO item_models (title, description, completed, completed_at) VALUES ('done', 'x', 1, '2026-01-02 03:04:05')",
	} {
		if _, err := old.Exec(stmt); err != nil {
			t.Fatalf("seed old db: %v", err)
		}
	}
	if err := old.Close(); err != nil {
		t.Fatalf("close old db: %v", err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	db, err := NewDatabase(path)
	if err != nil {
		t.Fatalf("NewDatabase on v2.1.1 file failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tasks, err := db.ListTasks(t.Context())
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	got := map[string]TaskStatus{}
	for _, task := range tasks {
		got[task.Title] = task.Status
	}
	if got["open"] != StatusTodo || got["done"] != StatusDone {
		t.Fatalf("expected statuses backfilled from completed, got %v", got)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
			t.Fatalf("migration changed file permissions to %v", info.Mode().Perm())
		}
	}
}

func TestTaskTagsLifecycle(t *testing.T) {
	db := newFileTestDB(t)
	task := &ItemModel{Title: "A", Description: "x", Tags: []string{"Work", "home", "work"}}
	if err := db.CreateTask(t.Context(), task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	other := &ItemModel{Title: "B", Description: "x", Tags: []string{"home"}}
	if err := db.CreateTask(t.Context(), other); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	got, err := db.GetTaskByID(t.Context(), task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}
	if want := []string{"home", "work"}; !slices.Equal(got.Tags, want) {
		t.Fatalf("expected normalised tags %v, got %v", want, got.Tags)
	}

	got.Tags = []string{"errands"}
	if err := db.UpdateTask(t.Context(), got); err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}
	all, err := db.ListTasks(t.Context())
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	byTitle := map[string][]string{}
	for _, it := range all {
		byTitle[it.Title] = it.Tags
	}
	if !slices.Equal(byTitle["A"], []string{"errands"}) || !slices.Equal(byTitle["B"], []string{"home"}) {
		t.Fatalf("unexpected tags after update: %v", byTitle)
	}
	if n := countRows(t, db, "tags"); n != 2 {
		t.Fatalf("expected unused tag 'work' pruned (2 tags left), got %d", n)
	}

	if err := db.DeleteTask(t.Context(), other.ID); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}
	if n := countRows(t, db, "task_tags"); n != 1 {
		t.Fatalf("expected deleted task's tag links removed, got %d links", n)
	}
	if n := countRows(t, db, "tags"); n != 1 {
		t.Fatalf("expected orphaned tag pruned, got %d tags", n)
	}
}

func countRows(t *testing.T, db *Database, table string) int {
	t.Helper()
	var n int64
	if err := db.Conn().Table(table).Count(&n).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return int(n)
}

func TestInvalidTagsAreRejectedOnSave(t *testing.T) {
	db := newFileTestDB(t)
	err := db.CreateTask(t.Context(), &ItemModel{Title: "A", Description: "x", Tags: []string{"bad tag"}})
	if err == nil || !strings.Contains(err.Error(), "letters, digits") {
		t.Fatalf("expected tag validation error, got %v", err)
	}
	if tasks, _ := db.ListTasks(t.Context()); len(tasks) != 0 {
		t.Fatalf("expected no task stored after a rejected save, got %d", len(tasks))
	}
}

func TestBeforeSaveKeepsStatusAndCompletedConsistent(t *testing.T) {
	cases := []struct {
		name          string
		in            ItemModel
		wantStatus    TaskStatus
		wantCompleted bool
	}{
		{"empty status, not completed", ItemModel{}, StatusTodo, false},
		{"empty status, completed", ItemModel{Completed: true}, StatusDone, true},
		{"doing", ItemModel{Status: StatusDoing}, StatusDoing, false},
		{"completed flag wins over todo", ItemModel{Status: StatusTodo, Completed: true}, StatusDone, true},
		{"cleared completed flag wins over done", ItemModel{Status: StatusDone}, StatusTodo, false},
		{"status case is normalised", ItemModel{Status: "DOING"}, StatusDoing, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := tc.in
			if err := item.BeforeSave(nil); err != nil {
				t.Fatalf("BeforeSave failed: %v", err)
			}
			if item.Status != tc.wantStatus || item.Completed != tc.wantCompleted {
				t.Fatalf("got status=%q completed=%v, want %q/%v", item.Status, item.Completed, tc.wantStatus, tc.wantCompleted)
			}
			if item.Completed != (item.CompletedAt != nil) {
				t.Fatalf("CompletedAt inconsistent with Completed: %+v", item)
			}
		})
	}

	bad := ItemModel{Status: "blocked"}
	if err := bad.BeforeSave(nil); err == nil {
		t.Fatal("expected invalid status to be rejected")
	}
}

func TestDatabaseOperationsHonourCancelledContext(t *testing.T) {
	db := newFileTestDB(t)
	task := &ItemModel{Title: "A", Description: "x"}
	if err := db.CreateTask(t.Context(), task); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	checks := map[string]error{
		"CreateTask":          db.CreateTask(ctx, &ItemModel{Title: "B", Description: "x"}),
		"UpdateTask":          db.UpdateTask(ctx, task),
		"DeleteTask":          db.DeleteTask(ctx, task.ID),
		"ReplaceAllTasks":     db.ReplaceAllTasks(ctx, nil),
		"ReplaceAllTasksFunc": db.ReplaceAllTasksFunc(ctx, func(c []*ItemModel) ([]*ItemModel, error) { return nil, nil }),
	}
	_, checks["ListTasks"] = db.ListTasks(ctx)
	_, checks["GetTaskByID"] = db.GetTaskByID(ctx, task.ID)
	for name, err := range checks {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s with cancelled context: got %v, want context.Canceled", name, err)
		}
	}
	if tasks, err := db.ListTasks(t.Context()); err != nil || len(tasks) != 1 {
		t.Fatalf("expected data untouched after cancelled calls, got %v (err %v)", tasks, err)
	}
}

func TestReplaceAllTasksFuncRollsBackOnError(t *testing.T) {
	db := newFileTestDB(t)
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "keep", Description: "x", Tags: []string{"t"}}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	boom := errors.New("boom")
	err := db.ReplaceAllTasksFunc(t.Context(), func(current []*ItemModel) ([]*ItemModel, error) {
		if len(current) != 1 || !slices.Equal(current[0].Tags, []string{"t"}) {
			return nil, fmt.Errorf("unexpected snapshot %+v", current)
		}
		return nil, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected fn error, got %v", err)
	}
	tasks, _ := db.ListTasks(t.Context())
	if len(tasks) != 1 || tasks[0].Title != "keep" {
		t.Fatalf("expected no change after failed fn, got %+v", tasks)
	}
}

func TestMigratedDatabaseCanBeOpenedReadOnly(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDatabase(filepath.Join(dir, "munus.db"))
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "A", Description: "x", Tags: []string{"work"}}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// A relative URI keeps the DSN portable across operating systems.
	t.Chdir(dir)
	ro, err := NewDatabase("file:munus.db?mode=ro")
	if err != nil {
		t.Fatalf("opening an up-to-date database read-only failed: %v", err)
	}
	t.Cleanup(func() { _ = ro.Close() })
	tasks, err := ro.ListTasks(t.Context())
	if err != nil || len(tasks) != 1 || !slices.Equal(tasks[0].Tags, []string{"work"}) {
		t.Fatalf("expected the stored task through a read-only connection, got %+v (err %v)", tasks, err)
	}
	if err := ro.CreateTask(t.Context(), &ItemModel{Title: "B", Description: "x"}); err == nil {
		t.Fatal("expected writes through a read-only connection to fail")
	}
}

func TestConsistencyTriggersRepairOlderBinaryWrites(t *testing.T) {
	db := newFileTestDB(t)
	doing := &ItemModel{Title: "doing", Description: "x", Status: StatusDoing, Tags: []string{"work"}}
	done := &ItemModel{Title: "done", Description: "x", Completed: true}
	for _, task := range []*ItemModel{doing, done} {
		if err := db.CreateTask(t.Context(), task); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}

	// These statements are what Munus v2.1.1 issues: it knows nothing about
	// the status column or the tag tables.
	stamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO item_models (title, description, completed, completed_at, created_at, updated_at) VALUES ('old-done', 'x', 1, ?, ?, ?)", []any{stamp, stamp, stamp}},
		{"UPDATE item_models SET completed = 0, completed_at = NULL WHERE id = ?", []any{done.ID}},
		{"UPDATE item_models SET completed = 1, completed_at = ? WHERE id = ?", []any{stamp, doing.ID}},
	} {
		if err := db.Conn().Exec(stmt.sql, stmt.args...).Error; err != nil {
			t.Fatalf("%s: %v", stmt.sql, err)
		}
	}

	tasks, err := db.ListTasks(t.Context())
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	got := map[string]TaskStatus{}
	for _, task := range tasks {
		got[task.Title] = task.Status
	}
	want := map[string]TaskStatus{"old-done": StatusDone, "done": StatusTodo, "doing": StatusDone}
	for title, status := range want {
		if got[title] != status {
			t.Errorf("%s: status %q, want %q", title, got[title], status)
		}
	}

	if err := db.Conn().Exec("DELETE FROM item_models WHERE id = ?", doing.ID).Error; err != nil {
		t.Fatalf("raw delete failed: %v", err)
	}
	if n := countRows(t, db, "task_tags"); n != 0 {
		t.Fatalf("expected tag links of a task deleted by an older binary to be removed, got %d", n)
	}
}

func TestOpeningRepairsOrphansAndInstallsTriggersOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "munus.db")
	db, err := NewDatabase(path)
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	// Simulate state left behind before the delete trigger existed: a tag
	// link to a task that is gone, and a status that disagrees with completed.
	for _, stmt := range []string{
		"DROP TRIGGER munus_tags_after_delete",
		"DROP TRIGGER munus_status_after_insert",
		"INSERT INTO tags (id, name) VALUES (7, 'ghost')",
		"INSERT INTO task_tags (task_id, tag_id) VALUES (999, 7)",
		"INSERT INTO item_models (title, description, status, completed) VALUES ('mismatch', 'x', 'todo', 1)",
	} {
		if err := db.Conn().Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	for range 2 {
		// Each connection gets its own variable so every cleanup closes its
		// own handle; Windows cannot remove the temp dir while one is open.
		reopened, err := NewDatabase(path)
		if err != nil {
			t.Fatalf("reopen failed: %v", err)
		}
		t.Cleanup(func() { _ = reopened.Close() })
		db = reopened
	}
	if n := countRows(t, db, "task_tags"); n != 0 {
		t.Fatalf("expected orphaned tag link removed, got %d", n)
	}
	if n := countRows(t, db, "tags"); n != 0 {
		t.Fatalf("expected orphaned tag pruned, got %d", n)
	}
	tasks, err := db.ListTasks(t.Context())
	if err != nil || len(tasks) != 1 || tasks[0].Status != StatusDone {
		t.Fatalf("expected mismatched status repaired to done, got %+v (err %v)", tasks, err)
	}
	var triggers int64
	if err := db.Conn().Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name LIKE 'munus_%'").Scan(&triggers).Error; err != nil {
		t.Fatalf("count triggers: %v", err)
	}
	if int(triggers) != len(consistencyTriggers) {
		t.Fatalf("expected %d triggers after reopening, got %d", len(consistencyTriggers), triggers)
	}
}

func TestUpdateTaskDoesNotRecreateDeletedTask(t *testing.T) {
	db := newFileTestDB(t)
	task := &ItemModel{Title: "A", Description: "x", Tags: []string{"work"}}
	if err := db.CreateTask(t.Context(), task); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := db.DeleteTask(t.Context(), task.ID); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	task.Title = "edited"
	if err := db.UpdateTask(t.Context(), task); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
	if tasks, _ := db.ListTasks(t.Context()); len(tasks) != 0 {
		t.Fatalf("expected the deleted task to stay deleted, got %+v", tasks)
	}
	if n := countRows(t, db, "task_tags"); n != 0 {
		t.Fatalf("expected no tag links for a deleted task, got %d", n)
	}
}

func TestReplaceAllTasksRemovesOldTagLinksAndUnusedTags(t *testing.T) {
	db := newFileTestDB(t)
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "old", Description: "x", Tags: []string{"stale", "shared"}}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	replacement := []*ItemModel{
		{ID: 50, Title: "new", Description: "x", Tags: []string{"shared", "fresh"}},
		{Title: "untagged", Description: "x"},
	}
	if err := db.ReplaceAllTasks(t.Context(), replacement); err != nil {
		t.Fatalf("ReplaceAllTasks failed: %v", err)
	}

	if n := countRows(t, db, "task_tags"); n != 2 {
		t.Fatalf("expected only the new task's 2 tag links, got %d", n)
	}
	var names []string
	if err := db.Conn().Table("tags").Order("name").Pluck("name", &names).Error; err != nil {
		t.Fatalf("read tags: %v", err)
	}
	if want := []string{"fresh", "shared"}; !slices.Equal(names, want) {
		t.Fatalf("expected tags %v after replace, got %v", want, names)
	}
}

func TestLoadTagsSortsNames(t *testing.T) {
	db := newFileTestDB(t)
	task := &ItemModel{Title: "A", Description: "x"}
	if err := db.CreateTask(t.Context(), task); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	// Insert links in reverse order so the stored order is not sorted.
	for i, name := range []string{"zeta", "mid", "alpha"} {
		if err := db.Conn().Exec("INSERT INTO tags (id, name) VALUES (?, ?)", i+1, name).Error; err != nil {
			t.Fatalf("insert tag: %v", err)
		}
		if err := db.Conn().Exec("INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?)", task.ID, i+1).Error; err != nil {
			t.Fatalf("insert link: %v", err)
		}
	}

	want := []string{"alpha", "mid", "zeta"}
	got, err := db.GetTaskByID(t.Context(), task.ID)
	if err != nil || !slices.Equal(got.Tags, want) {
		t.Fatalf("GetTaskByID tags = %v (err %v), want %v", got.Tags, err, want)
	}
	all, err := db.ListTasks(t.Context())
	if err != nil || len(all) != 1 || !slices.Equal(all[0].Tags, want) {
		t.Fatalf("ListTasks tags = %+v (err %v), want %v", all, err, want)
	}
}

// ============================== plan 3: storage ==============================

// P-033 / N-026: SetTaskStatus writes only the status columns, so edits made
// through another handle after the task was read are kept.
func TestSetTaskStatusKeepsOtherFields(t *testing.T) {
	db := newFileTestDB(t)
	ctx := t.Context()
	task := &ItemModel{Title: "old", Description: "d", Tags: []string{"home"}}
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	edited, _ := db.GetTaskByID(ctx, task.ID)
	edited.Title, edited.Tags = "edited elsewhere", []string{"urgent"}
	if err := db.UpdateTask(ctx, edited); err != nil {
		t.Fatalf("setup edit failed: %v", err)
	}

	doneAt := time.Date(2026, 9, 27, 1, 2, 3, 0, time.UTC)
	if err := db.SetTaskStatus(ctx, task.ID, StatusDone, doneAt); err != nil {
		t.Fatalf("SetTaskStatus failed: %v", err)
	}
	got, _ := db.GetTaskByID(ctx, task.ID)
	if got.Title != "edited elsewhere" || !slices.Equal(got.Tags, []string{"urgent"}) {
		t.Fatalf("status change overwrote other fields: %+v", got)
	}
	if got.Status != StatusDone || !got.Completed || got.CompletedAt == nil || !got.CompletedAt.Equal(doneAt) {
		t.Fatalf("expected done with completed_at %v, got %+v", doneAt, got)
	}

	// Setting the same status again keeps the original completion time.
	if err := db.SetTaskStatus(ctx, task.ID, StatusDone, doneAt.Add(time.Hour)); err != nil {
		t.Fatalf("SetTaskStatus failed: %v", err)
	}
	if got, _ := db.GetTaskByID(ctx, task.ID); !got.CompletedAt.Equal(doneAt) {
		t.Fatalf("completing a done task changed completed_at to %v", got.CompletedAt)
	}

	if err := db.SetTaskStatus(ctx, task.ID, StatusDoing, doneAt); err != nil {
		t.Fatalf("SetTaskStatus failed: %v", err)
	}
	if got, _ := db.GetTaskByID(ctx, task.ID); got.Status != StatusDoing || got.Completed || got.CompletedAt != nil {
		t.Fatalf("expected doing without completion, got %+v", got)
	}

	if err := db.SetTaskStatus(ctx, 999, StatusDone, doneAt); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
	if err := db.SetTaskStatus(ctx, task.ID, "bogus", doneAt); err == nil {
		t.Fatal("expected an invalid status to be rejected")
	}
	if err := db.SetTaskStatus(ctx, 0, StatusDone, doneAt); err == nil {
		t.Fatal("expected a zero ID to be rejected")
	}
}

// P-034 / SEC-013: opening a database repairs statuses that are not todo,
// doing or done (for example written by another tool).
func TestNewDatabaseRepairsUnknownStatuses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "munus.db")
	db, err := NewDatabase(path)
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	for _, task := range []*ItemModel{
		{Title: "escape", Description: "x"},
		{Title: "upper", Description: "x", Completed: true},
		{Title: "doing", Description: "x", Status: StatusDoing},
		{Title: "loud doing", Description: "x"},
	} {
		if err := db.CreateTask(t.Context(), task); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}
	for title, status := range map[string]string{"escape": "\x1b]0;PWNED\x07", "upper": "DONE", "loud doing": " DOING"} {
		if err := db.Conn().Exec("UPDATE item_models SET status = ? WHERE title = ?", status, title).Error; err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	reopened, err := NewDatabase(path)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	want := map[string]TaskStatus{"escape": StatusTodo, "upper": StatusDone, "doing": StatusDoing, "loud doing": StatusDoing}
	tasks, err := reopened.ListTasks(t.Context())
	if err != nil || len(tasks) != len(want) {
		t.Fatalf("expected %d tasks, got %d (err %v)", len(want), len(tasks), err)
	}
	for _, task := range tasks {
		if task.Status != want[task.Title] {
			t.Errorf("%s: status %q, want %q", task.Title, task.Status, want[task.Title])
		}
	}
}

// P-036 / SEC-017: databaseFilePath maps a DSN to the file sqlite creates.
func TestDatabaseFilePath(t *testing.T) {
	cases := []struct {
		dsn  string
		goos string
		want string
		ok   bool
	}{
		{dsn: "munus.db", want: "munus.db", ok: true},
		{dsn: "dir/munus.db?_busy_timeout=5000", want: "dir/munus.db", ok: true},
		{dsn: ":memory:"},
		{dsn: ":memory:?cache=shared"},
		{dsn: ""},
		{dsn: "?x=1", want: "?x=1", ok: true}, // go-sqlite3 cuts only after the first character
		{dsn: "file:munus.db", want: "munus.db", ok: true},
		{dsn: "file:sp%20ace.db?cache=shared", want: "sp ace.db", ok: true},
		{dsn: "file:/abs/munus.db", want: "/abs/munus.db", ok: true},
		{dsn: "file:///abs/munus.db", want: "/abs/munus.db", ok: true},
		{dsn: "file://localhost/abs/munus.db", want: "/abs/munus.db", ok: true},
		{dsn: "file:munus.db#fragment", want: "munus.db", ok: true},
		{dsn: "file:munus.db#frag?mode=ro", want: "munus.db", ok: true},
		{dsn: "file:munus.db?mode=rwc", want: "munus.db", ok: true},
		{dsn: "file:munus.db?mode=ro&mode=rwc", want: "munus.db", ok: true}, // the last mode wins
		{dsn: "file:munus.db?mode=rwc&mode=ro"},
		{dsn: "file:munus.db?mode="},
		{dsn: "file:munus.db?mode=ro"},
		{dsn: "file:munus.db?mode=rw"},
		{dsn: "file:munus.db?m%6Fde=ro"},
		{dsn: "file:mem?mode=memory"},
		{dsn: "file::memory:"},
		{dsn: "file:munus.db?vfs=unix-none", want: "munus.db", ok: true},
		{dsn: "file:munus.db?vfs=memdb"},
		{dsn: "file://otherhost/abs/munus.db"},
		{dsn: "file:bad%zz.db", want: "bad%zz.db", ok: true}, // sqlite keeps invalid escapes
		{dsn: "file:a%00b.db", want: "a", ok: true},          // an encoded NUL ends the path
		{dsn: "file:///C:/munus.db", goos: "windows", want: "C:/munus.db", ok: true},
		{dsn: "file:/C:/munus.db", goos: "windows", want: "C:/munus.db", ok: true},
		{dsn: "file:///C:/munus.db", goos: "linux", want: "/C:/munus.db", ok: true},
		{dsn: `C:\data\munus.db?_busy_timeout=5000`, goos: "windows", want: `C:\data\munus.db`, ok: true},
	}
	for _, tc := range cases {
		goos := tc.goos
		if goos == "" {
			goos = "linux"
		}
		got, ok := databaseFilePathFor(tc.dsn, goos)
		if got != tc.want || ok != tc.ok {
			t.Errorf("databaseFilePathFor(%q, %s) = %q, %v; want %q, %v", tc.dsn, goos, got, ok, tc.want, tc.ok)
		}
	}
}

// P-036 / SEC-017: new database files opened through a DSN are owner-only,
// sqlite opens exactly the pre-created file, and existing files keep their mode.
func TestNewDatabaseDSNFilesAreOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	dir := t.TempDir()
	for name, dsn := range map[string]string{
		"param.db":     filepath.Join(dir, "param.db") + "?_busy_timeout=5000",
		"uri.db":       "file:" + filepath.Join(dir, "uri.db"),
		"sp ace.db":    "file:" + filepath.Join(dir, "sp%20ace.db") + "?cache=shared",
		"localhost.db": "file://localhost" + filepath.Join(dir, "localhost.db"),
	} {
		db, err := NewDatabase(dsn)
		if err != nil {
			t.Fatalf("%s: NewDatabase failed: %v", name, err)
		}
		t.Cleanup(func() { _ = db.Close() })
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s: stat: %v", name, err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s: new database is %v, want 0600", name, perm)
		}
		opened, err := os.Stat(db.databaseFile())
		if err != nil || !os.SameFile(info, opened) {
			t.Errorf("%s: sqlite opened %q, not the pre-created file (%v)", name, db.databaseFile(), err)
		}
	}

	existing := filepath.Join(dir, "existing.db")
	seed, err := NewDatabase(existing)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	_ = seed.Close()
	if err := os.Chmod(existing, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	db, err := NewDatabase("file:" + existing + "?cache=shared")
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	_ = db.Close()
	if info, _ := os.Stat(existing); info.Mode().Perm() != 0o644 {
		t.Errorf("existing database mode changed to %v", info.Mode().Perm())
	}

	// A DSN the driver rejects after the file was pre-created leaves no file.
	bad := filepath.Join(dir, "bad.db")
	if db, err := NewDatabase(bad + "?_busy_timeout=abc"); err == nil {
		_ = db.Close()
		t.Error("expected an invalid driver parameter to fail")
	}
	if _, err := os.Stat(bad); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a failed open must not leave a stray file, stat: %v", err)
	}

	missing := filepath.Join(dir, "missing.db")
	if db, err := NewDatabase("file:" + missing + "?mode=ro"); err == nil {
		_ = db.Close()
		t.Error("expected a read-only open of a missing file to fail")
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a read-only open must not create the file, stat: %v", err)
	}
}

// ============================== plan 3: phase B ==============================

// openConcurrently opens path from n goroutines at once (separate connection
// pools, as separate processes would have) and returns the errors.
func openConcurrently(t *testing.T, path string, n int) []error {
	t.Helper()
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db, err := NewDatabase(path)
			if err != nil {
				errs[i] = err
				return
			}
			errs[i] = db.Close()
		}()
	}
	wg.Wait()
	return errs
}

// P-037 / N-027: several first opens of a new database, or of an older one
// that still needs its triggers, all succeed.
func TestConcurrentFirstOpenSucceeds(t *testing.T) {
	for i := range 20 {
		path := filepath.Join(t.TempDir(), "new.db")
		for _, err := range openConcurrently(t, path, 4) {
			if err != nil {
				t.Fatalf("round %d: concurrent first open failed: %v", i, err)
			}
		}
	}
	for i := range 20 {
		path := filepath.Join(t.TempDir(), "old.db")
		db, err := NewDatabase(path)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		for name := range consistencyTriggers {
			if err := db.Conn().Exec("DROP TRIGGER " + name).Error; err != nil {
				t.Fatalf("setup failed: %v", err)
			}
		}
		_ = db.Close()
		for _, err := range openConcurrently(t, path, 4) {
			if err != nil {
				t.Fatalf("round %d: concurrent upgrade failed: %v", i, err)
			}
		}
		check, err := NewDatabase(path)
		if err != nil {
			t.Fatalf("reopen failed: %v", err)
		}
		var n int64
		if err := check.Conn().Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger'").Scan(&n).Error; err != nil || n != int64(len(consistencyTriggers)) {
			t.Fatalf("expected %d triggers after the upgrade, got %d (%v)", len(consistencyTriggers), n, err)
		}
		_ = check.Close()
	}
}

// importWorkerEnv makes the test binary act as one import process for
// TestConcurrentImportProcessesAllApply (the usual helper-process pattern).
const importWorkerEnv = "MUNUS_TEST_IMPORT_WORKER_DB"

func TestMain(m *testing.M) {
	if path := os.Getenv(importWorkerEnv); path != "" {
		os.Exit(runImportWorker(path, os.Getenv("MUNUS_TEST_IMPORT_WORKER_TITLE")))
	}
	os.Exit(m.Run())
}

// importsPerWorker is how many imports each helper process applies in a row,
// so the processes' transactions overlap.
const importsPerWorker = 10

func runImportWorker(path, title string) int {
	// A long busy timeout: SQLite does not queue waiting writers fairly, so on
	// slow storage one worker can wait through many others' imports.
	db, err := NewDatabase(path + "?_busy_timeout=60000")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = db.Close() }()
	svc := &TaskServiceAdapter{storage: db}
	for i := range importsPerWorker {
		data := fmt.Sprintf(`{"version":2,"tasks":[{"title":"%s-%d"}]}`, title, i)
		if _, err := applyImportData(context.Background(), svc, []byte(data), ImportConfig{}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}

// P-037 / N-037: imports run by separate processes at the same time all
// apply. Every transaction starts with BEGIN IMMEDIATE, so a second writer
// waits for the lock instead of failing with "database is locked" when its
// read-then-write transaction cannot upgrade its lock (v2.2.0 failed about
// two thirds of such imports).
func TestConcurrentImportProcessesAllApply(t *testing.T) {
	if testing.Short() {
		t.Skip("starts helper processes")
	}
	setTestHome(t)
	path := filepath.Join(t.TempDir(), "munus.db")
	seed, err := NewDatabase(path)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	_ = seed.Close()

	const workers, rounds = 6, 2
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	for round := range rounds {
		cmds := make([]*exec.Cmd, workers)
		outs := make([]*bytes.Buffer, workers)
		for i := range cmds {
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
			cmd.Env = append(os.Environ(), importWorkerEnv+"="+path, fmt.Sprintf("MUNUS_TEST_IMPORT_WORKER_TITLE=r%d-w%d", round, i))
			outs[i] = &bytes.Buffer{}
			cmd.Stdout, cmd.Stderr = outs[i], outs[i]
			if err := cmd.Start(); err != nil {
				t.Fatalf("start worker: %v", err)
			}
			cmds[i] = cmd
		}
		for i, cmd := range cmds {
			if err := cmd.Wait(); err != nil {
				t.Errorf("round %d: import process %d failed: %v: %s", round, i, err, outs[i])
			}
		}
	}
	check, err := NewDatabase(path)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer func() { _ = check.Close() }()
	tasks, err := check.ListTasks(t.Context())
	if err != nil || len(tasks) != workers*rounds*importsPerWorker {
		t.Fatalf("expected %d imported tasks, got %d (err %v)", workers*rounds*importsPerWorker, len(tasks), err)
	}
}

func TestWithImmediateTransactions(t *testing.T) {
	for dsn, want := range map[string]string{
		"munus.db":                    "munus.db?_txlock=immediate",
		"munus.db?_busy_timeout=5000": "munus.db?_busy_timeout=5000&_txlock=immediate",
		"file:munus.db":               "file:munus.db?_txlock=immediate",
		"file:munus.db?mode=ro":       "file:munus.db?mode=ro&_txlock=immediate",
		":memory:":                    ":memory:?_txlock=immediate",
		"munus.db?_txlock=exclusive":  "munus.db?_txlock=exclusive",
		"?odd":                        "?odd",
	} {
		if got := withImmediateTransactions(dsn); got != want {
			t.Errorf("withImmediateTransactions(%q) = %q, want %q", dsn, got, want)
		}
	}
}

// P-042 / N-036: a database that fails to migrate is closed, so repeated
// failed opens do not leak file descriptors.
func TestNewDatabaseClosesConnectionWhenSetupFails(t *testing.T) {
	fds := func() int {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			t.Skip("needs /proc/self/fd")
		}
		return len(entries)
	}
	// A valid database whose "tags" name is taken by a view opens fine but
	// cannot be migrated.
	path := filepath.Join(t.TempDir(), "blocked.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("CREATE VIEW tags AS SELECT 1 AS id, 'x' AS name"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	_ = raw.Close()
	before := fds()
	for range 20 {
		if db, err := NewDatabase(path); err == nil {
			_ = db.Close()
			t.Fatal("expected the migration to fail")
		}
	}
	if grown := fds() - before; grown >= 10 {
		t.Fatalf("20 failed opens left %d more file descriptors open", grown)
	}
}

// P-037: installing the consistency triggers again is a no-op, so a migration
// that races another one cannot fail on an existing trigger.
func TestConsistencyTriggerDDLIsIdempotent(t *testing.T) {
	db := newFileTestDB(t)
	for name, ddl := range consistencyTriggers {
		if err := db.Conn().Exec(ddl).Error; err != nil {
			t.Errorf("%s: re-running the trigger DDL failed: %v", name, err)
		}
	}
}

// A deferred database creates its data directory (owner-only) only when it is
// first opened, and the database file inside it is owner-only too.
func TestDeferredDatabaseCreatesPrivateDataDirOnOpen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data", "munus")
	path := filepath.Join(dir, "munus.db")
	db := NewDeferredDatabaseAt(DatabaseLocation{Path: path, Dir: dir})
	t.Cleanup(func() { _ = db.Close() })

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected no data directory before open, stat err=%v", err)
	}
	if err := db.open(); err != nil {
		t.Fatalf("open failed: %v", err)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil || !dirInfo.IsDir() {
		t.Fatalf("expected data directory after open, got %v (err %v)", dirInfo, err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected database file after open: %v", err)
	}
	if runtime.GOOS == "windows" {
		return // Unix permission bits do not apply
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("data directory mode = %o, want 700", got)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("database file mode = %o, want 600", got)
	}
}

func TestDeferredDatabaseTightensExistingDataDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits do not apply on Windows")
	}
	dir := filepath.Join(t.TempDir(), "munus")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil { // the umask may have removed bits
		t.Fatalf("setup failed: %v", err)
	}
	db := NewDeferredDatabaseAt(DatabaseLocation{Path: filepath.Join(dir, "munus.db"), Dir: dir})
	t.Cleanup(func() { _ = db.Close() })
	if err := db.open(); err != nil {
		t.Fatalf("open failed: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("data directory mode = %o, want 700", got)
	}
}

func TestDeferredDatabaseDataDirMustBeADirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "munus")
	if err := os.WriteFile(dir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	db := NewDeferredDatabaseAt(DatabaseLocation{Path: filepath.Join(dir, "munus.db"), Dir: dir})
	err := db.open()
	if err == nil || !strings.Contains(err.Error(), "create data directory") {
		t.Fatalf("expected a data directory error, got %v", err)
	}
}

// A location that could not be resolved reports its error when the database
// is opened and creates nothing.
func TestDeferredDatabaseReportsLocationError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "munus")
	locErr := errors.New("no home directory")
	db := NewDeferredDatabaseAt(DatabaseLocation{Path: filepath.Join(dir, "munus.db"), Dir: dir, Err: locErr})
	if err := db.open(); !errors.Is(err, locErr) {
		t.Fatalf("open error = %v, want %v", err, locErr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected nothing created, stat err=%v", err)
	}
}

func TestDeferredDatabaseNoticeIsTakenOnce(t *testing.T) {
	db := NewDeferredDatabaseAt(DatabaseLocation{Path: filepath.Join(t.TempDir(), "munus.db"), Notice: "hello"})
	t.Cleanup(func() { _ = db.Close() })
	if got := db.takeNotice(); got != "hello" {
		t.Fatalf("first takeNotice = %q, want %q", got, "hello")
	}
	if got := db.takeNotice(); got != "" {
		t.Fatalf("second takeNotice = %q, want empty", got)
	}
	var nilDB *Database
	if got := nilDB.takeNotice(); got != "" {
		t.Fatalf("nil takeNotice = %q, want empty", got)
	}
}
