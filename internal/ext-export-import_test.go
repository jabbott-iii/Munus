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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// MockStorage is a mock implementation of Storage for testing
type MockStorage struct {
	tasks []*ItemModel
	err   error
}

func (m *MockStorage) CreateTask(task *ItemModel) error {
	if m.err != nil {
		return m.err
	}
	m.tasks = append(m.tasks, task)
	return nil
}

func (m *MockStorage) GetTaskByID(id int) (*ItemModel, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, t := range m.tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, nil
}

func (m *MockStorage) ListTasks() ([]*ItemModel, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tasks, nil
}

func (m *MockStorage) UpdateTask(task *ItemModel) error {
	if m.err != nil {
		return m.err
	}
	for i, t := range m.tasks {
		if t.ID == task.ID {
			m.tasks[i] = task
			return nil
		}
	}
	return nil
}

func (m *MockStorage) DeleteTask(id int) error {
	if m.err != nil {
		return m.err
	}
	for i, t := range m.tasks {
		if t.ID == id {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *MockStorage) ReplaceAllTasks(tasks []*ItemModel) error {
	if m.err != nil {
		return m.err
	}
	m.tasks = tasks
	return nil

}

// Helper function to create a test adapter
func NewTestAdapter(storage Storage) *TaskServiceAdapter {
	return &TaskServiceAdapter{storage: storage}
}

func writeTestImportBundle(t *testing.T, bundle ExportBundle) string {
	t.Helper()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "import.json")

	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("failed to marshal import bundle: %v", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("failed to write import bundle: %v", err)
	}

	return filePath
}

func findTaskByTitle(tasks []*ItemModel, title string) *ItemModel {
	for _, task := range tasks {
		if task.Title == title {
			return task
		}
	}
	return nil
}

// Helper function to create sample tasks
func createSampleItemModels() []*ItemModel {
	now := time.Now()
	deadline := now.Add(24 * time.Hour)
	return []*ItemModel{
		{
			ID:          1,
			Title:       "Task 1",
			Description: "First task",
			Completed:   false,
			Deadline:    &deadline,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          2,
			Title:       "Task 2",
			Description: "Second task",
			Completed:   true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          3,
			Title:       "Task 3",
			Description: "Third task without deadline",
			Completed:   false,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

// Tests for ListTasks
func TestListTasks(t *testing.T) {
	storage := &MockStorage{
		tasks: createSampleItemModels(),
	}
	adapter := NewTestAdapter(storage)

	tasks, err := adapter.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}

	if tasks[0].Title != "Task 1" {
		t.Errorf("expected first task title 'Task 1', got %q", tasks[0].Title)
	}

	if tasks[0].ID != "1" {
		t.Errorf("expected task ID '1', got %q", tasks[0].ID)
	}
}

func TestListTasksError(t *testing.T) {
	storage := &MockStorage{err: new(testError)}
	adapter := NewTestAdapter(storage)

	tasks, err := adapter.ListTasks()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if tasks != nil {
		t.Errorf("expected nil tasks, got %v", tasks)
	}
}

// Tests for ReplaceAll
func TestReplaceAll(t *testing.T) {
	storage := &MockStorage{}
	adapter := NewTestAdapter(storage)

	tasks := []Task{
		{
			ID:          "1",
			Title:       "New Task 1",
			Description: "New description",
			Completed:   false,
		},
	}

	if err := adapter.ReplaceAll(tasks); err != nil {
		t.Fatalf("ReplaceAll failed: %v", err)
	}
	if len(storage.tasks) != 1 {
		t.Fatalf("expected 1 stored task, got %d", len(storage.tasks))
	}
	if storage.tasks[0].ID != 1 || storage.tasks[0].Title != "New Task 1" {
		t.Errorf("expected task with preserved ID 1, got %+v", storage.tasks[0])
	}
}

func TestReplaceAllAssignsNewIDsToNonNumericIDs(t *testing.T) {
	storage := &MockStorage{}
	adapter := NewTestAdapter(storage)

	if err := adapter.ReplaceAll([]Task{{ID: newID(), Title: "Renamed"}, {ID: "", Title: "Blank"}, {ID: "0", Title: "Zero"}}); err != nil {
		t.Fatalf("ReplaceAll failed: %v", err)
	}
	for _, task := range storage.tasks {
		if task.ID != 0 {
			t.Errorf("expected ID 0 (database-assigned) for %q, got %d", task.Title, task.ID)
		}
	}
}

func TestReplaceAllNilStorage(t *testing.T) {
	adapter := &TaskServiceAdapter{storage: nil}

	var tasks []Task
	err := adapter.ReplaceAll(tasks)
	if err == nil {
		t.Fatal("expected error for nil storage, got nil")
	}
}

// Tests for export functions
func TestFilterTasks(t *testing.T) {
	tasks := []Task{
		{ID: "1", Title: "Todo Task", Completed: false},
		{ID: "2", Title: "Done Task", Completed: true},
		{ID: "3", Title: "Another Todo", Completed: false},
	}

	// Test excluding completed tasks
	filter := ExportFilter{IncludeCompleted: false}
	filtered := filterTasks(tasks, filter)
	if len(filtered) != 2 {
		t.Errorf("expected 2 tasks when excluding completed, got %d", len(filtered))
	}

	// Test including completed tasks
	filter = ExportFilter{IncludeCompleted: true}
	filtered = filterTasks(tasks, filter)
	if len(filtered) != 3 {
		t.Errorf("expected 3 tasks when including completed, got %d", len(filtered))
	}
}

func TestToDTO(t *testing.T) {
	deadline := time.Now()
	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now()

	task := Task{
		ID:          "123",
		Title:       "Test Task",
		Description: "Test Description",
		Completed:   true,
		Deadline:    &deadline,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	dto := toDTO(TaskDTO(task))

	if dto.ID != "123" {
		t.Errorf("expected ID '123', got %q", dto.ID)
	}
	if dto.Title != "Test Task" {
		t.Errorf("expected title 'Test Task', got %q", dto.Title)
	}
	if !dto.Completed {
		t.Error("expected task to be completed")
	}
	if dto.Deadline != &deadline {
		t.Error("deadline mismatch")
	}
}

func TestPlanExport(t *testing.T) {
	storage := &MockStorage{
		tasks: createSampleItemModels(),
	}
	adapter := NewTestAdapter(storage)

	filter := ExportFilter{IncludeCompleted: false}
	plan, err := PlanExport(adapter, filter)
	if err != nil {
		t.Fatalf("PlanExport failed: %v", err)
	}

	if plan.Total != 2 {
		t.Errorf("expected total 2 (excluding completed), got %d", plan.Total)
	}
	if plan.Todo != 2 {
		t.Errorf("expected todo 2, got %d", plan.Todo)
	}
	if plan.Done != 0 {
		t.Errorf("expected done 0, got %d", plan.Done)
	}
}

func TestPlanExportIncludeCompleted(t *testing.T) {
	storage := &MockStorage{
		tasks: createSampleItemModels(),
	}
	adapter := NewTestAdapter(storage)

	filter := ExportFilter{IncludeCompleted: true}
	plan, err := PlanExport(adapter, filter)
	if err != nil {
		t.Fatalf("PlanExport failed: %v", err)
	}

	if plan.Total != 3 {
		t.Errorf("expected total 3, got %d", plan.Total)
	}
	if plan.Todo != 2 {
		t.Errorf("expected todo 2, got %d", plan.Todo)
	}
	if plan.Done != 1 {
		t.Errorf("expected done 1, got %d", plan.Done)
	}
}

func TestExportToBytes(t *testing.T) {
	storage := &MockStorage{
		tasks: createSampleItemModels(),
	}
	adapter := NewTestAdapter(storage)

	filter := ExportFilter{IncludeCompleted: true}

	// Test non-pretty format
	data, err := ExportToBytes(adapter, filter, false)
	if err != nil {
		t.Fatalf("ExportToBytes failed: %v", err)
	}

	var bundle ExportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("failed to unmarshal exported data: %v", err)
	}

	if bundle.Version != 1 {
		t.Errorf("expected version 1, got %d", bundle.Version)
	}
	if len(bundle.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(bundle.Tasks))
	}
}

func TestExportToBytesPretty(t *testing.T) {
	storage := &MockStorage{
		tasks: createSampleItemModels(),
	}
	adapter := NewTestAdapter(storage)

	filter := ExportFilter{IncludeCompleted: true}

	// Test pretty format
	data, err := ExportToBytes(adapter, filter, true)
	if err != nil {
		t.Fatalf("ExportToBytes failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("exported data is empty")
	}

	// Pretty format should contain whitespace
	if !bytes.Contains(data, []byte("\n")) && !bytes.Contains(data, []byte("  ")) {
		t.Fatal("expected pretty format with whitespace")
	}
}

func TestExportToFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "export.json")

	storage := &MockStorage{
		tasks: createSampleItemModels(),
	}
	adapter := NewTestAdapter(storage)

	filter := ExportFilter{IncludeCompleted: true}
	err := ExportToFile(adapter, filter, filePath, false)
	if err != nil {
		t.Fatalf("ExportToFile failed: %v", err)
	}

	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("exported file not found: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}

	var bundle ExportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("failed to unmarshal exported file: %v", err)
	}

	if len(bundle.Tasks) != 3 {
		t.Errorf("expected 3 tasks in export, got %d", len(bundle.Tasks))
	}
}

// Tests for import functions
func TestFromDTO(t *testing.T) {
	deadline := time.Now()
	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now()

	dto := TaskDTO{
		ID:          "456",
		Title:       "DTO Task",
		Description: "DTO Description",
		Completed:   false,
		Deadline:    &deadline,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	task := fromDTO(Task(dto))

	if task.ID != "456" {
		t.Errorf("expected ID '456', got %q", task.ID)
	}
	if task.Title != "DTO Task" {
		t.Errorf("expected title 'DTO Task', got %q", task.Title)
	}
	if task.Completed {
		t.Error("expected task to be incomplete")
	}
}

func TestEqualTask(t *testing.T) {
	now := time.Now()

	task1 := Task{
		ID:          "1",
		Title:       "Same Title",
		Description: "Same Description",
		Completed:   true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	task2 := Task{
		ID:          "2", // Different ID shouldn't matter for equality
		Title:       "Same Title",
		Description: "Same Description",
		Completed:   true,
		CreatedAt:   now.Add(-1 * time.Hour), // Different timestamps shouldn't matter
		UpdatedAt:   now.Add(-1 * time.Hour),
	}

	if !equalTask(task1, task2) {
		t.Error("expected tasks to be equal")
	}

	task3 := Task{
		ID:          "1",
		Title:       "Different Title",
		Description: "Same Description",
		Completed:   true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if equalTask(task1, task3) {
		t.Error("expected tasks to be different")
	}
}

func TestNewID(t *testing.T) {
	id1 := newID()
	time.Sleep(1 * time.Millisecond) // Ensure different timestamp
	id2 := newID()

	if id1 == "" || id2 == "" {
		t.Error("expected non-empty IDs")
	}
	if id1 == id2 {
		t.Error("expected unique IDs")
	}
	if !bytes.HasPrefix([]byte(id1), []byte("tsk_")) {
		t.Errorf("expected ID to start with 'tsk_', got %q", id1)
	}
}

func TestReadImportFileValid(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "import.json")

	bundle := ExportBundle{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Tasks: []TaskDTO{
			{
				ID:          "1",
				Title:       "Task 1",
				Description: "Description 1",
				Completed:   false,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			{
				ID:        "2",
				Title:     "Task 2",
				Completed: true,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}

	data, _ := json.Marshal(bundle)
	err := os.WriteFile(filePath, data, 0o644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tasks, version, err := readImportFile(filePath, false)
	if err != nil {
		t.Fatalf("readImportFile failed: %v", err)
	}

	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Title != "Task 1" {
		t.Errorf("expected first task title 'Task 1', got %q", tasks[0].Title)
	}
}

func TestReadImportFileInvalidVersion(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "import.json")

	bundle := ExportBundle{
		Version:    99, // Invalid version
		ExportedAt: time.Now().UTC(),
		Tasks:      []TaskDTO{},
	}

	data, _ := json.Marshal(bundle)
	err := os.WriteFile(filePath, data, 0o644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, _, err = readImportFile(filePath, false)
	if err == nil {
		t.Fatal("expected error for invalid version, got nil")
	}
}

func TestReadImportFileMissingTitle(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "import.json")

	bundle := ExportBundle{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Tasks: []TaskDTO{
			{
				ID:        "1",
				Title:     "", // Missing title
				Completed: false,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}

	data, _ := json.Marshal(bundle)
	err := os.WriteFile(filePath, data, 0o644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, _, err = readImportFile(filePath, false)
	if err == nil {
		t.Fatal("expected error for missing title, got nil")
	}
}

func TestReadImportFileDuplicateID(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "import.json")

	bundle := ExportBundle{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Tasks: []TaskDTO{
			{
				ID:        "1",
				Title:     "Task 1",
				Completed: false,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			{
				ID:        "1", // Duplicate ID
				Title:     "Task 2",
				Completed: false,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
	}

	data, _ := json.Marshal(bundle)
	err := os.WriteFile(filePath, data, 0o644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, _, err = readImportFile(filePath, false)
	if err == nil {
		t.Fatal("expected error for duplicate ID, got nil")
	}
}

func TestReadImportFileStrict(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "import.json")

	// Write a bundle with extra fields
	jsonData := `{
		"version": 1,
		"exported_at": "2026-01-01T00:00:00Z",
		"tasks": [
			{
				"id": "1",
				"title": "Task 1",
				"completed": false,
				"created_at": "2026-01-01T00:00:00Z",
				"updated_at": "2026-01-01T00:00:00Z"
			}
		],
		"extra_field": "should fail in strict mode"
	}`

	err := os.WriteFile(filePath, []byte(jsonData), 0o644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Non-strict should succeed
	_, _, err = readImportFile(filePath, false)
	if err != nil {
		t.Fatalf("non-strict mode failed: %v", err)
	}

	// Strict mode should fail
	_, _, err = readImportFile(filePath, true)
	if err == nil {
		t.Fatal("expected error in strict mode for unknown fields, got nil")
	}
}

func TestPlanImport(t *testing.T) {
	now := time.Now()
	filePath := writeTestImportBundle(t, ExportBundle{
		Version:    1,
		ExportedAt: now,
		Tasks: []TaskDTO{
			{
				ID:        "1",
				Title:     "Existing Task",
				Completed: false,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "2",
				Title:     "New Task",
				Completed: false,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	})

	// Set up existing tasks
	storage := &MockStorage{
		tasks: []*ItemModel{
			{
				ID:        1,
				Title:     "Existing Task",
				Completed: false,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	adapter := NewTestAdapter(storage)

	cfg := ImportConfig{Strict: false}
	plan, err := PlanImport(adapter, filePath, cfg)
	if err != nil {
		t.Fatalf("PlanImport failed: %v", err)
	}

	if plan.SchemaVersion != 1 {
		t.Errorf("expected version 1, got %d", plan.SchemaVersion)
	}
	if plan.ToCreate != 1 {
		t.Errorf("expected 1 task to create, got %d", plan.ToCreate)
	}
	if plan.Unchanged != 1 {
		t.Errorf("expected 1 unchanged task, got %d", plan.Unchanged)
	}
}

func TestPlanImportSkipExistingShowsConflicts(t *testing.T) {
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
				ID:        "2",
				Title:     "New Task",
				Completed: false,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	})

	storage := &MockStorage{
		tasks: []*ItemModel{
			{
				ID:          1,
				Title:       "Existing Task",
				Description: "Local",
				Completed:   false,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}

	plan, err := PlanImport(NewTestAdapter(storage), filePath, ImportConfig{Mode: "merge", SkipExisting: true})
	if err != nil {
		t.Fatalf("PlanImport failed: %v", err)
	}

	if plan.ToCreate != 1 {
		t.Errorf("expected 1 task to create, got %d", plan.ToCreate)
	}
	if plan.ToUpdate != 0 {
		t.Errorf("expected 0 tasks to update, got %d", plan.ToUpdate)
	}
	if plan.Conflicts != 1 {
		t.Errorf("expected 1 conflict, got %d", plan.Conflicts)
	}
	if len(plan.ConflictIDs) != 1 || plan.ConflictIDs[0] != "1" {
		t.Errorf("expected conflict ID [1], got %v", plan.ConflictIDs)
	}
}

func TestMergeReplace(t *testing.T) {
	current := []Task{
		{ID: "1", Title: "Old Task 1", Completed: false},
		{ID: "2", Title: "Old Task 2", Completed: true},
	}

	incoming := []Task{
		{ID: "3", Title: "New Task 1", Completed: false},
	}

	cfg := ImportConfig{Mode: "merge", OnConflict: "overwrite"}
	merged, result := merge(current, incoming, cfg)

	if len(merged) != 3 {
		t.Errorf("expected 3 merged tasks, got %d", len(merged))
	}
	if result.Created != 1 {
		t.Errorf("expected 1 created, got %d", result.Created)
	}
}

func TestMergeConflictSkip(t *testing.T) {
	now := time.Now()
	current := []Task{
		{ID: "1", Title: "Task 1", Description: "Current", Completed: false, CreatedAt: now, UpdatedAt: now},
	}

	incoming := []Task{
		{ID: "1", Title: "Task 1", Description: "Updated", Completed: false, CreatedAt: now, UpdatedAt: now},
	}

	cfg := ImportConfig{Mode: "merge", OnConflict: "skip"}
	merged, result := merge(current, incoming, cfg)

	if len(merged) != 1 {
		t.Errorf("expected 1 merged task, got %d", len(merged))
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
	if result.Conflicted != 1 {
		t.Errorf("expected 1 conflicted, got %d", result.Conflicted)
	}
	if len(result.SkippedIDs) != 1 || result.SkippedIDs[0] != "1" {
		t.Errorf("expected skipped IDs [1], got %v", result.SkippedIDs)
	}
}

func TestMergeConflictRename(t *testing.T) {
	now := time.Now()
	current := []Task{
		{ID: "1", Title: "Task 1", Completed: false, CreatedAt: now, UpdatedAt: now},
	}

	incoming := []Task{
		{ID: "1", Title: "Task 1", Description: "Updated", Completed: false, CreatedAt: now, UpdatedAt: now},
	}

	cfg := ImportConfig{Mode: "merge", OnConflict: "rename"}
	merged, result := merge(current, incoming, cfg)

	if len(merged) != 2 {
		t.Errorf("expected 2 merged tasks (original + renamed), got %d", len(merged))
	}
	if result.Created != 1 {
		t.Errorf("expected 1 created (renamed), got %d", result.Created)
	}
	if result.Conflicted != 1 {
		t.Errorf("expected 1 conflicted, got %d", result.Conflicted)
	}
}

func TestApplyImportDefaultOverwrite(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

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
				Completed:   true,
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

	res, err := ApplyImport(&TaskServiceAdapter{storage: db}, filePath, ImportConfig{Mode: "merge"})
	if err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}

	if res.Created != 1 || res.Updated != 1 || res.Skipped != 0 || res.Conflicted != 1 {
		t.Errorf("unexpected import result: %+v", res)
	}

	tasks, err := db.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks after import, got %d", len(tasks))
	}

	existing := findTaskByTitle(tasks, "Existing Task")
	if existing == nil {
		t.Fatal("expected existing task to remain present")
	}
	if existing.Description != "Imported" || !existing.Completed {
		t.Errorf("expected existing task to be overwritten, got %+v", existing)
	}
	if findTaskByTitle(tasks, "New Task") == nil {
		t.Fatal("expected new task to be imported")
	}
}

func TestApplyImportSkipExisting(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

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
				Completed:   true,
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

	res, err := ApplyImport(&TaskServiceAdapter{storage: db}, filePath, ImportConfig{Mode: "merge", SkipExisting: true})
	if err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}

	if res.Created != 1 || res.Updated != 0 || res.Skipped != 1 || res.Conflicted != 1 {
		t.Errorf("unexpected import result: %+v", res)
	}
	if len(res.SkippedIDs) != 1 || res.SkippedIDs[0] != "1" {
		t.Errorf("expected skipped IDs [1], got %v", res.SkippedIDs)
	}

	tasks, err := db.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks after import, got %d", len(tasks))
	}

	existing := findTaskByTitle(tasks, "Existing Task")
	if existing == nil {
		t.Fatal("expected existing task to remain present")
	}
	if existing.Description != "Local" || existing.Completed {
		t.Errorf("expected existing task to be preserved, got %+v", existing)
	}
	if findTaskByTitle(tasks, "New Task") == nil {
		t.Fatal("expected new task to be imported")
	}
}

func TestMergeIDRegenerate(t *testing.T) {
	now := time.Now()
	incoming := []Task{
		{ID: "1", Title: "Task 1", Completed: false, CreatedAt: now, UpdatedAt: now},
	}

	cfg := ImportConfig{IDStrategy: "regenerate"}
	merged, result := merge([]Task{}, incoming, cfg)

	if len(merged) != 1 {
		t.Errorf("expected 1 merged task, got %d", len(merged))
	}
	if !bytes.HasPrefix([]byte(merged[0].ID), []byte("tsk_")) {
		t.Errorf("expected regenerated ID to start with 'tsk_', got %q", merged[0].ID)
	}
	if result.Created != 1 {
		t.Errorf("expected 1 created, got %d", result.Created)
	}
}

func TestWriteBackup(t *testing.T) {
	now := time.Now()
	tasks := []Task{
		{ID: "1", Title: "Task 1", Completed: false, CreatedAt: now, UpdatedAt: now},
		{ID: "2", Title: "Task 2", Completed: true, CreatedAt: now, UpdatedAt: now},
	}

	setTestHome(t)

	path, err := writeBackup(tasks)
	if err != nil {
		t.Fatalf("writeBackup failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("backup file not created: %v", err)
	}

	data, _ := os.ReadFile(path)
	var bundle ExportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("failed to unmarshal backup: %v", err)
	}

	if len(bundle.Tasks) != 2 {
		t.Errorf("expected 2 tasks in backup, got %d", len(bundle.Tasks))
	}
}

// Test helper
type testError struct{}

func (e *testError) Error() string {
	return "test error"
}

// This allows us to use bytes.HasPrefix and bytes.Contains
// If bytes are not available in the test, we can use alternative checks

// ============================== import integrity regressions ==============================

func newFileTestDB(t *testing.T) *Database {
	t.Helper()
	db, err := NewDatabase(filepath.Join(t.TempDir(), "munus.db"))
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func setTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func tasksByID(t *testing.T, db *Database) map[int]*ItemModel {
	t.Helper()
	tasks, err := db.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	out := make(map[int]*ItemModel, len(tasks))
	for _, task := range tasks {
		out[task.ID] = task
	}
	return out
}

func TestApplyImportMergeRoundTripKeepsIDsAndCompletedAt(t *testing.T) {
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	completedAt := time.Now().Add(-48 * time.Hour).Truncate(time.Second)

	for _, task := range []*ItemModel{
		{Title: "A", Description: "a"},
		{Title: "B", Description: "b", Completed: true, CompletedAt: &completedAt},
		{Title: "C", Description: "c"},
	} {
		if err := db.CreateTask(task); err != nil {
			t.Fatalf("CreateTask failed: %v", err)
		}
	}
	if err := db.DeleteTask(1); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}
	before := tasksByID(t, db)

	file := filepath.Join(t.TempDir(), "export.json")
	if err := ExportToFile(svc, ExportFilter{IncludeCompleted: true}, file, true); err != nil {
		t.Fatalf("ExportToFile failed: %v", err)
	}

	for i := 1; i <= 2; i++ {
		res, err := ApplyImport(svc, file, ImportConfig{Mode: "merge"})
		if err != nil {
			t.Fatalf("import #%d failed: %v", i, err)
		}
		if res.Created != 0 || res.Unchanged != 2 {
			t.Fatalf("import #%d: expected 0 created / 2 unchanged, got %+v", i, res)
		}
		after := tasksByID(t, db)
		if len(after) != len(before) {
			t.Fatalf("import #%d: expected %d tasks, got %d", i, len(before), len(after))
		}
		for id, task := range before {
			got, ok := after[id]
			if !ok || got.Title != task.Title {
				t.Fatalf("import #%d: task %d (%s) not preserved, got %+v", i, id, task.Title, after)
			}
		}
		b := after[2]
		if b.CompletedAt == nil || !b.CompletedAt.Equal(completedAt) {
			t.Fatalf("import #%d: expected CompletedAt %v, got %v", i, completedAt, b.CompletedAt)
		}
	}
}

func TestApplyImportPreservesIncomingIDs(t *testing.T) {
	db := newFileTestDB(t)
	now := time.Now()
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "7", Title: "Seven", CreatedAt: now, UpdatedAt: now},
		{ID: "legacy-id", Title: "Legacy", CreatedAt: now, UpdatedAt: now},
	}})

	if _, err := ApplyImport(&TaskServiceAdapter{storage: db}, file, ImportConfig{Mode: "merge"}); err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}
	got := tasksByID(t, db)
	if got[7] == nil || got[7].Title != "Seven" {
		t.Fatalf("expected task 7 to keep its ID, got %+v", got)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(got))
	}
}

func TestApplyImportReplaceKeepsIDsAndCompletedAt(t *testing.T) {
	db := newFileTestDB(t)
	if err := db.CreateTask(&ItemModel{Title: "Old", Description: "x"}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	completedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "12", Title: "Twelve", Completed: true, CompletedAt: &completedAt},
		{ID: "", Title: "No ID"},
	}})

	res, err := ApplyImport(&TaskServiceAdapter{storage: db}, file, ImportConfig{Mode: "replace"})
	if err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}
	if res.Created != 2 {
		t.Fatalf("expected 2 created, got %+v", res)
	}
	got := tasksByID(t, db)
	if got[12] == nil || got[12].CompletedAt == nil || !got[12].CompletedAt.Equal(completedAt) {
		t.Fatalf("expected task 12 with CompletedAt %v, got %+v", completedAt, got[12])
	}
	if findTaskByTitle(mapValues(got), "Old") != nil {
		t.Fatal("expected replace to remove local tasks")
	}
	if findTaskByTitle(mapValues(got), "No ID") == nil {
		t.Fatal("expected task without ID to be created")
	}
}

func mapValues(m map[int]*ItemModel) []*ItemModel {
	out := make([]*ItemModel, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func TestReadImportFileFillsCompletedAtForLegacyFiles(t *testing.T) {
	updated := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	stray := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "1", Title: "Done", Completed: true, UpdatedAt: updated},
		{ID: "2", Title: "Open", Completed: false, CompletedAt: &stray},
	}})
	tasks, _, err := readImportFile(file, false)
	if err != nil {
		t.Fatalf("readImportFile failed: %v", err)
	}
	if tasks[0].CompletedAt == nil || !tasks[0].CompletedAt.Equal(updated) {
		t.Errorf("expected CompletedAt to fall back to updated_at, got %v", tasks[0].CompletedAt)
	}
	if tasks[1].CompletedAt != nil {
		t.Errorf("expected incomplete task to have no CompletedAt, got %v", tasks[1].CompletedAt)
	}
}

func TestApplyImportMergeUpdatesChangedDeadline(t *testing.T) {
	db := newFileTestDB(t)
	if err := db.CreateTask(&ItemModel{Title: "A", Description: "x"}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	deadline := time.Date(2031, 5, 6, 0, 0, 0, 0, time.UTC)
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "1", Title: "A", Description: "x", Deadline: &deadline},
	}})
	svc := &TaskServiceAdapter{storage: db}

	plan, err := PlanImport(svc, file, ImportConfig{})
	if err != nil {
		t.Fatalf("PlanImport failed: %v", err)
	}
	if plan.ToUpdate != 1 || plan.Unchanged != 0 {
		t.Fatalf("expected deadline change to be an update, got %+v", plan)
	}
	if _, err := ApplyImport(svc, file, ImportConfig{}); err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}
	got := tasksByID(t, db)[1]
	if got == nil || got.Deadline == nil || !got.Deadline.Equal(deadline) {
		t.Fatalf("expected deadline %v, got %+v", deadline, got)
	}
}

func TestEqualTaskComparesDeadline(t *testing.T) {
	d1 := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	d2 := d1.In(time.FixedZone("x", 3600)) // same instant, different zone
	d3 := d1.Add(time.Minute)
	base := Task{Title: "T"}
	cases := []struct {
		a, b *time.Time
		want bool
	}{
		{nil, nil, true}, {&d1, nil, false}, {nil, &d1, false}, {&d1, &d2, true}, {&d1, &d3, false},
	}
	for _, c := range cases {
		a, b := base, base
		a.Deadline, b.Deadline = c.a, c.b
		if got := equalTask(a, b); got != c.want {
			t.Errorf("equalTask deadlines %v vs %v = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestPlanImportMatchesApplyForRegenerate(t *testing.T) {
	db := newFileTestDB(t)
	if err := db.CreateTask(&ItemModel{Title: "A", Description: "x"}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{{ID: "1", Title: "A", Description: "x"}}})
	svc := &TaskServiceAdapter{storage: db}
	cfg := ImportConfig{Mode: "merge", IDStrategy: "regenerate"}

	plan, err := PlanImport(svc, file, cfg)
	if err != nil {
		t.Fatalf("PlanImport failed: %v", err)
	}
	res, err := ApplyImport(svc, file, cfg)
	if err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}
	if plan.ToCreate != res.Created || plan.Unchanged != res.Unchanged || plan.ToUpdate != res.Updated {
		t.Fatalf("plan %+v does not match result %+v", plan, res)
	}
	if res.Created != 1 {
		t.Fatalf("expected regenerate to create 1 task, got %+v", res)
	}
}

func TestImportRejectsInvalidOptionsBeforeBackup(t *testing.T) {
	home := setTestHome(t)
	db := newFileTestDB(t)
	file := writeTestImportBundle(t, ExportBundle{Version: 1})
	svc := &TaskServiceAdapter{storage: db}

	for _, cfg := range []ImportConfig{
		{Mode: "bogus", Backup: true},
		{Mode: "merge", OnConflict: "bogus", Backup: true},
		{Mode: "merge", IDStrategy: "ict", Backup: true},
	} {
		if _, err := PlanImport(svc, file, cfg); err == nil {
			t.Errorf("PlanImport(%+v): expected error", cfg)
		}
		if _, err := ApplyImport(svc, file, cfg); err == nil {
			t.Errorf("ApplyImport(%+v): expected error", cfg)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".munus")); !os.IsNotExist(err) {
		t.Fatalf("expected no backup to be written for invalid options, stat err=%v", err)
	}
}

func TestReadImportFileCanonicalizesNumericIDs(t *testing.T) {
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "01", Title: "A"}, {ID: "1", Title: "B"},
	}})
	if _, _, err := readImportFile(file, false); err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("expected duplicate id error, got %v", err)
	}
}

func TestReadImportFileHandlesUnsafeText(t *testing.T) {
	controlCases := map[string]TaskDTO{
		"escape in title":     {ID: "1", Title: "\x1b]0;PWNED\x07ok"},
		"bell in description": {ID: "1", Title: "ok", Description: "ring\x07"},
		"carriage return":     {ID: "1", Title: "ok", Description: "a\rb"},
	}
	for name, dto := range controlCases {
		t.Run("strict rejects "+name, func(t *testing.T) {
			file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{dto}})
			if _, _, err := readImportFile(file, true); err == nil {
				t.Fatalf("expected strict import to reject %s", name)
			}
		})
		t.Run("default strips "+name, func(t *testing.T) {
			file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{dto}})
			tasks, _, err := readImportFile(file, false)
			if err != nil {
				t.Fatalf("expected %s to be sanitised, got %v", name, err)
			}
			if containsControl(tasks[0].Title, false) || containsControl(tasks[0].Description, true) {
				t.Fatalf("expected control characters removed, got %q / %q", tasks[0].Title, tasks[0].Description)
			}
		})
	}

	crlf := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{{ID: "1", Title: "ok", Description: "line1\r\nline2"}}})
	tasks, _, err := readImportFile(crlf, false)
	if err != nil || tasks[0].Description != "line1\nline2" {
		t.Fatalf("expected CRLF normalised to LF, got %q (err %v)", tasks[0].Description, err)
	}

	for name, dto := range map[string]TaskDTO{
		"title too long":       {ID: "1", Title: strings.Repeat("a", MaxTitleLength+1)},
		"description too long": {ID: "1", Title: "ok", Description: strings.Repeat("a", MaxDescriptionLength+1)},
	} {
		file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{dto}})
		if _, _, err := readImportFile(file, false); err == nil {
			t.Errorf("expected %s to be rejected", name)
		}
	}

	ok := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{{ID: "1", Title: "ok", Description: "line1\n\tline2"}}})
	if _, _, err := readImportFile(ok, true); err != nil {
		t.Fatalf("expected newline/tab in description to be accepted, got %v", err)
	}
}

func TestBackupOfLegacyControlCharactersCanBeRestored(t *testing.T) {
	setTestHome(t)
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	// Rows written before validation existed may contain control characters.
	if err := db.CreateTask(&ItemModel{Title: "legacy", Description: "line1\r\nline2\x07"}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	empty := writeTestImportBundle(t, ExportBundle{Version: 1})
	res, err := ApplyImport(svc, empty, ImportConfig{Mode: "replace", Backup: true})
	if err != nil {
		t.Fatalf("replace import failed: %v", err)
	}

	if _, err := ApplyImport(svc, res.BackupPath, ImportConfig{Mode: "replace"}); err != nil {
		t.Fatalf("restoring the backup failed: %v", err)
	}
	tasks, _ := db.ListTasks()
	if len(tasks) != 1 || tasks[0].Description != "line1\nline2" {
		t.Fatalf("expected sanitised legacy task restored, got %+v", tasks)
	}
}

func TestImportDuplicateIDErrorIsQuoted(t *testing.T) {
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "x\x1b[2J", Title: "A"}, {ID: "x\x1b[2J", Title: "B"},
	}})
	_, _, err := readImportFile(file, false)
	if err == nil || strings.Contains(err.Error(), "\x1b") {
		t.Fatalf("expected quoted duplicate id error without raw escapes, got %q", err)
	}
}

func TestImportHugeIDGetsNewDatabaseID(t *testing.T) {
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "9223372036854775807", Title: "Huge"}, {ID: "2147483648", Title: "Above cap"},
	}})
	if _, err := ApplyImport(svc, file, ImportConfig{Mode: "replace"}); err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}
	for id := range tasksByID(t, db) {
		if id > maxImportedTaskID {
			t.Fatalf("expected oversized IDs to be replaced, found ID %d", id)
		}
	}
	if err := db.CreateTask(&ItemModel{Title: "after", Description: "x"}); err != nil {
		t.Fatalf("expected later inserts to keep working, got %v", err)
	}
}

func TestApplyImportReplaceWithRegenerateAssignsNewIDs(t *testing.T) {
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{{ID: "500", Title: "A"}, {ID: "501", Title: "B"}}})
	cfg := ImportConfig{Mode: "replace", IDStrategy: "regenerate"}

	plan, err := PlanImport(svc, file, cfg)
	if err != nil {
		t.Fatalf("PlanImport failed: %v", err)
	}
	res, err := ApplyImport(svc, file, cfg)
	if err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}
	if plan.ToCreate != res.Created || res.Created != 2 {
		t.Fatalf("plan %+v does not match result %+v", plan, res)
	}
	got := tasksByID(t, db)
	if got[500] != nil || got[501] != nil || len(got) != 2 {
		t.Fatalf("expected regenerated IDs, got %v", got)
	}
}

func TestMergeOverwriteKeepsOriginalCompletionTime(t *testing.T) {
	original := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	incomingTime := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	current := []Task{{ID: "1", Title: "T", Description: "old", Completed: true, CompletedAt: &original}}
	incoming := []Task{{ID: "1", Title: "T", Description: "new", Completed: true, CompletedAt: &incomingTime}}

	merged, res := merge(current, incoming, ImportConfig{Mode: "merge", OnConflict: "overwrite"})
	if res.Updated != 1 || merged[0].Description != "new" {
		t.Fatalf("expected overwrite, got %+v / %+v", res, merged)
	}
	if merged[0].CompletedAt == nil || !merged[0].CompletedAt.Equal(original) {
		t.Fatalf("expected original completion time kept, got %v", merged[0].CompletedAt)
	}
}

func TestExportRefusesDatabaseOpenedWithParameters(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "u.db")
	db, err := NewDatabase(real + "?_busy_timeout=5000")
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.CreateTask(&ItemModel{Title: "A", Description: "x"}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 || entries[0].Name() != "u.db" {
		t.Fatalf("expected only u.db in %s, got %v", dir, entries)
	}
	if err := ExportToFile(&TaskServiceAdapter{storage: db}, ExportFilter{}, real, true); err == nil {
		t.Fatal("expected export over the parameterised database to be refused")
	}
}

func TestExportToFileWithLongName(t *testing.T) {
	db := newFileTestDB(t)
	target := filepath.Join(t.TempDir(), strings.Repeat("n", 240)+".json")
	if err := ExportToFile(&TaskServiceAdapter{storage: db}, ExportFilter{}, target, true); err != nil {
		t.Fatalf("expected long file name to export, got %v", err)
	}
}

func TestWriteBackupTightensExistingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions are not enforced on Windows")
	}
	home := setTestHome(t)
	dir := filepath.Join(home, ".munus", "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if _, err := writeBackup(nil); err != nil {
		t.Fatalf("writeBackup failed: %v", err)
	}
	if info, _ := os.Stat(dir); info.Mode().Perm() != 0o700 {
		t.Fatalf("expected backup dir tightened to 0700, got %v", info.Mode().Perm())
	}
}

func TestReadImportFileRejectsOversizedFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "huge.json")
	f, err := os.Create(file)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := f.Truncate(MaxImportFileSize + 1); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	_ = f.Close()

	if _, _, err := readImportFile(file, false); err == nil || !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("expected maximum size error, got %v", err)
	}
}

// ============================== export / backup hardening ==============================

func TestExportToFileRefusesActiveDatabase(t *testing.T) {
	db := newFileTestDB(t)
	if err := db.CreateTask(&ItemModel{Title: "A", Description: "x"}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	err := ExportToFile(&TaskServiceAdapter{storage: db}, ExportFilter{}, db.DatabasePath(), true)
	if err == nil || !strings.Contains(err.Error(), "active database") {
		t.Fatalf("expected refusal to overwrite database, got %v", err)
	}
	if tasks, err := db.ListTasks(); err != nil || len(tasks) != 1 {
		t.Fatalf("database damaged after refused export: tasks=%v err=%v", tasks, err)
	}
}

func TestExportToFileOverwritesOtherFilesWithOwnerOnlyPermissions(t *testing.T) {
	db := newFileTestDB(t)
	target := filepath.Join(t.TempDir(), "out.json")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := ExportToFile(&TaskServiceAdapter{storage: db}, ExportFilter{}, target, true); err != nil {
		t.Fatalf("ExportToFile failed: %v", err)
	}
	data, _ := os.ReadFile(target)
	if !bytes.Contains(data, []byte(`"version": 1`)) {
		t.Fatalf("expected export content, got %q", data)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(target)
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("expected 0600 export, got %v", perm)
		}
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".out.json.*.tmp"))
	if len(leftovers) != 0 {
		t.Fatalf("expected temp files to be cleaned up, got %v", leftovers)
	}
}

func TestExportToFileDoesNotFollowPlantedTmpSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	db := newFileTestDB(t)
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(victim, []byte("secret"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	target := filepath.Join(dir, "out.json")
	if err := os.Symlink(victim, target+".tmp"); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := ExportToFile(&TaskServiceAdapter{storage: db}, ExportFilter{}, target, true); err != nil {
		t.Fatalf("ExportToFile failed: %v", err)
	}
	if data, _ := os.ReadFile(victim); string(data) != "secret" {
		t.Fatalf("symlink target was overwritten: %q", data)
	}
	if info, err := os.Lstat(target); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected regular export file, got %v (err %v)", info, err)
	}
}

func TestWriteBackupIsUniqueAndOwnerOnly(t *testing.T) {
	home := setTestHome(t)
	tasks := []Task{{ID: "1", Title: "Task 1"}}

	first, err := writeBackup(tasks)
	if err != nil {
		t.Fatalf("writeBackup failed: %v", err)
	}
	second, err := writeBackup(tasks)
	if err != nil {
		t.Fatalf("writeBackup failed: %v", err)
	}
	if first == second {
		t.Fatalf("expected unique backup paths, both were %s", first)
	}
	if !strings.HasPrefix(first, filepath.Join(home, ".munus", "backups")) {
		t.Fatalf("expected backup under home, got %s", first)
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(first)
		di, _ := os.Stat(filepath.Dir(first))
		if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
			t.Fatalf("expected 0600 file / 0700 dir, got %v / %v", fi.Mode().Perm(), di.Mode().Perm())
		}
	}
}

func TestExportIncludesCompletedAt(t *testing.T) {
	completedAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	storage := &MockStorage{tasks: []*ItemModel{{ID: 1, Title: "Done", Completed: true, CompletedAt: &completedAt}}}
	b, err := ExportToBytes(NewTestAdapter(storage), ExportFilter{IncludeCompleted: true}, false)
	if err != nil {
		t.Fatalf("ExportToBytes failed: %v", err)
	}
	if !bytes.Contains(b, []byte(`"completed_at":"2026-03-04T05:06:07Z"`)) {
		t.Fatalf("expected completed_at in export, got %s", b)
	}
}
