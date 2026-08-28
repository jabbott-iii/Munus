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
	"encoding/json"
	"testing"
	"time"
)

// TestExportToBytesPrettyFormattingProducesValidJSON verifies that ExportToBytes
// with pretty=true produces properly indented, valid JSON output.
func TestExportToBytesPrettyFormattingProducesValidJSON(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create sample tasks
	task := &ItemModel{
		Title:       "Export Test Task",
		Description: "Test description",
		Completed:   false,
	}
	db.CreateTask(task)

	svc := &TaskServiceAdapter{storage: db}
	filter := ExportFilter{IncludeCompleted: false}

	// Export with pretty formatting
	payload, err := ExportToBytes(svc, filter, true)
	if err != nil {
		t.Fatalf("ExportToBytes() error = %v", err)
	}

	// Verify it's valid JSON
	var result ExportBundle
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("ExportToBytes() produced invalid JSON: %v", err)
	}

	// Verify structure
	if result.Version != 1 {
		t.Fatalf("ExportBundle.Version = %d, want 1", result.Version)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("ExportBundle.Tasks length = %d, want 1", len(result.Tasks))
	}
}

// TestExportToBytesSetsVersionAndTimestamp verifies that the export bundle
// includes correct version number and export timestamp.
func TestExportToBytesSetsVersionAndTimestamp(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	task := &ItemModel{Title: "Test"}
	db.CreateTask(task)

	svc := &TaskServiceAdapter{storage: db}
	filter := ExportFilter{IncludeCompleted: false}

	beforeExport := time.Now()
	payload, err := ExportToBytes(svc, filter, false)
	afterExport := time.Now()

	if err != nil {
		t.Fatalf("ExportToBytes() error = %v", err)
	}

	var result ExportBundle
	json.Unmarshal(payload, &result)

	if result.Version != 1 {
		t.Fatalf("Version = %d, want 1", result.Version)
	}

	if result.ExportedAt.Before(beforeExport) || result.ExportedAt.After(afterExport) {
		t.Fatalf("ExportedAt %v not within [%v, %v]", result.ExportedAt, beforeExport, afterExport)
	}
}

// TestExportFilterIncludeCompletedFalseExcludesCompleted verifies that the
// IncludeCompleted filter correctly excludes completed tasks.
func TestExportFilterIncludeCompletedFalseExcludesCompleted(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create mixed tasks
	incomplete := &ItemModel{Title: "Incomplete", Completed: false}
	complete := &ItemModel{Title: "Complete", Completed: true}

	db.CreateTask(incomplete)
	db.CreateTask(complete)

	svc := &TaskServiceAdapter{storage: db}
	filter := ExportFilter{IncludeCompleted: false}

	payload, _ := ExportToBytes(svc, filter, false)

	var result ExportBundle
	json.Unmarshal(payload, &result)

	if len(result.Tasks) != 1 {
		t.Fatalf("Filtered export returned %d tasks, want 1", len(result.Tasks))
	}

	if result.Tasks[0].Title != "Incomplete" {
		t.Fatalf("Filtered export has wrong task: %q, want Incomplete", result.Tasks[0].Title)
	}
}

// TestExportFilterIncludeCompletedTrueIncludesAll verifies that IncludeCompleted=true
// exports both completed and incomplete tasks.
func TestExportFilterIncludeCompletedTrueIncludesAll(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	incomplete := &ItemModel{Title: "Incomplete", Completed: false}
	complete := &ItemModel{Title: "Complete", Completed: true}

	db.CreateTask(incomplete)
	db.CreateTask(complete)

	svc := &TaskServiceAdapter{storage: db}
	filter := ExportFilter{IncludeCompleted: true}

	payload, _ := ExportToBytes(svc, filter, false)

	var result ExportBundle
	json.Unmarshal(payload, &result)

	if len(result.Tasks) != 2 {
		t.Fatalf("Export with IncludeCompleted=true returned %d tasks, want 2", len(result.Tasks))
	}
}

// TestPlanExportCalculatesCorrectCounts verifies that PlanExport accurately counts
// tasks by status (total, todo, done).
func TestPlanExportCalculatesCorrectCounts(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create test tasks
	for i := 0; i < 5; i++ {
		task := &ItemModel{
			Title:     "Task " + string(rune('A'+i)),
			Completed: i < 2, // First 2 are complete
		}
		db.CreateTask(task)
	}

	svc := &TaskServiceAdapter{storage: db}
	filter := ExportFilter{IncludeCompleted: true}

	plan, err := PlanExport(svc, filter)
	if err != nil {
		t.Fatalf("PlanExport() error = %v", err)
	}

	if plan.Total != 5 {
		t.Fatalf("PlanExport().Total = %d, want 5", plan.Total)
	}

	if plan.Done != 2 {
		t.Fatalf("PlanExport().Done = %d, want 2", plan.Done)
	}

	if plan.Todo != 3 {
		t.Fatalf("PlanExport().Todo = %d, want 3", plan.Todo)
	}
}

// TestTaskServiceAdapterListTasksReturnsAllTasks verifies that TaskServiceAdapter
// correctly converts ItemModel instances to Task DTOs.
func TestTaskServiceAdapterListTasksReturnsAllTasks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create test data with various fields
	now := time.Now()
	task := &ItemModel{
		Title:       "Service Test Task",
		Description: "With description",
		Deadline:    &now,
		Completed:   false,
	}
	db.CreateTask(task)

	svc := &TaskServiceAdapter{storage: db}
	tasks, err := svc.ListTasks()

	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("ListTasks() returned %d tasks, want 1", len(tasks))
	}

	retrieved := tasks[0]
	if retrieved.Title != "Service Test Task" || retrieved.Description != "With description" {
		t.Fatalf("Task conversion failed: got %+v", retrieved)
	}
}

// TestTaskServiceAdapterReplaceAllHandlesEmptyList verifies that ReplaceAll
// correctly handles clearing the database with an empty list.
func TestTaskServiceAdapterReplaceAllHandlesEmptyList(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create initial tasks
	task := &ItemModel{Title: "To Delete"}
	db.CreateTask(task)

	// Replace with empty
	svc := &TaskServiceAdapter{storage: db}
	err := svc.ReplaceAll([]Task{})

	if err != nil {
		t.Fatalf("ReplaceAll([]) error = %v", err)
	}

	// Verify empty
	tasks, _ := db.ListTasks()
	if len(tasks) != 0 {
		t.Fatalf("After ReplaceAll([]), database has %d tasks, want 0", len(tasks))
	}
}

// TestTaskServiceAdapterReplaceAllPreserveTaskData verifies that ReplaceAll
// correctly migrates task data (IDs are regenerated but content is preserved).
func TestTaskServiceAdapterReplaceAllPreserveTaskData(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a task
	original := &ItemModel{
		Title:       "Original Task",
		Description: "Original Desc",
		Completed:   true,
	}
	db.CreateTask(original)

	// Extract and replace
	svc := &TaskServiceAdapter{storage: db}
	tasks, _ := svc.ListTasks()

	err := svc.ReplaceAll(tasks)
	if err != nil {
		t.Fatalf("ReplaceAll() error = %v", err)
	}

	// Verify content
	newTasks, _ := db.ListTasks()
	if len(newTasks) != 1 {
		t.Fatalf("After ReplaceAll, got %d tasks, want 1", len(newTasks))
	}

	replaced := newTasks[0]
	if replaced.Title != "Original Task" || replaced.Description != "Original Desc" || !replaced.Completed {
		t.Fatalf("Data not preserved: got %+v", replaced)
	}
}

// TestExportBundleSerialization verifies that ExportBundle correctly serializes
// to JSON with all expected fields.
func TestExportBundleSerialization(t *testing.T) {
	deadline := time.Now()
	bundle := ExportBundle{
		Version:    1,
		ExportedAt: deadline,
		Tasks: []TaskDTO{
			{
				ID:          "1",
				Title:       "Test Task",
				Description: "Test Desc",
				Completed:   false,
				Deadline:    &deadline,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
		},
	}

	payload, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Verify round-trip
	var result ExportBundle
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if result.Version != 1 || len(result.Tasks) != 1 {
		t.Fatalf("Round-trip serialization failed: %+v", result)
	}
}

// TestTaskDTOOmitsEmptyDescription verifies that TaskDTO marshals with
// description omitted when empty (JSON omitempty tag behavior).
func TestTaskDTOOmitsEmptyDescription(t *testing.T) {
	taskWithDesc := TaskDTO{
		ID:          "1",
		Title:       "With Desc",
		Description: "Not empty",
		Completed:   false,
	}

	taskNoDesc := TaskDTO{
		ID:        "2",
		Title:     "No Desc",
		Completed: false,
	}

	// Marshal both
	withDescJSON, _ := json.Marshal(taskWithDesc)
	noDescJSON, _ := json.Marshal(taskNoDesc)

	// Verify omitempty behavior
	if !bytes.Contains(withDescJSON, []byte("\"description\"")) {
		t.Fatal("TaskDTO with description not serialized")
	}

	if bytes.Contains(noDescJSON, []byte("\"description\"")) {
		t.Fatal("TaskDTO without description should omit description field")
	}
}

// TestExportBundleVersionField ensures the version field is correctly set
// for schema compatibility tracking.
func TestExportBundleVersionField(t *testing.T) {
	bundle := ExportBundle{
		Version:    1,
		ExportedAt: time.Now(),
		Tasks:      []TaskDTO{},
	}

	payload, _ := json.Marshal(bundle)

	var result map[string]interface{}
	json.Unmarshal(payload, &result)

	version, ok := result["version"]
	if !ok {
		t.Fatal("version field missing from ExportBundle JSON")
	}

	if v, ok := version.(float64); !ok || int(v) != 1 {
		t.Fatalf("version field = %v, want 1", version)
	}
}

// TestTaskServiceAdapterWithNilStorageReturnsError returns error on nil storage
func TestTaskServiceAdapterWithNilStorageReturnsError(t *testing.T) {
	svc := &TaskServiceAdapter{storage: nil}

	err := svc.ReplaceAll([]Task{})
	if err == nil {
		t.Fatal("ReplaceAll with nil storage should return error")
	}
}
