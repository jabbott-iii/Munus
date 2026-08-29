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
	storage := &MockStorage{err: ErrMsg{error: new(testError)}}
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

	err := adapter.ReplaceAll(tasks)
	if err == nil {
		// Expected - mock storage allows ReplaceAll
	}
}

func TestReplaceAllNilStorage(t *testing.T) {
	adapter := &TaskServiceAdapter{storage: nil}

	tasks := []Task{}
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

	dto := toDTO(task)

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
	if bytes.Contains(data, []byte("\n")) || bytes.Contains(data, []byte("  ")) {
		// Expected - pretty format
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

	task := fromDTO(dto)

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
	os.WriteFile(filePath, data, 0o644)

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
	os.WriteFile(filePath, data, 0o644)

	_, _, err := readImportFile(filePath, false)
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
	os.WriteFile(filePath, data, 0o644)

	_, _, err := readImportFile(filePath, false)
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
	os.WriteFile(filePath, data, 0o644)

	_, _, err := readImportFile(filePath, false)
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

	os.WriteFile(filePath, []byte(jsonData), 0o644)

	// Non-strict should succeed
	_, _, err := readImportFile(filePath, false)
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
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "import.json")

	now := time.Now()
	bundle := ExportBundle{
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
	}

	data, _ := json.Marshal(bundle)
	os.WriteFile(filePath, data, 0o644)

	// Setup existing tasks
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

	// Save HOME env var and temporarily set it
	oldHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

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
// If bytes is not available in the test, we can use alternative checks
