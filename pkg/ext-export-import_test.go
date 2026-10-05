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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// MockStorage is a mock implementation of Storage for testing
type MockStorage struct {
	tasks []*ItemModel
	err   error
}

func (m *MockStorage) CreateTask(_ context.Context, task *ItemModel) error {
	if m.err != nil {
		return m.err
	}
	m.tasks = append(m.tasks, task)
	return nil
}

func (m *MockStorage) GetTaskByID(_ context.Context, id int) (*ItemModel, error) {
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

func (m *MockStorage) ListTasks(_ context.Context) ([]*ItemModel, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tasks, nil
}

func (m *MockStorage) UpdateTask(_ context.Context, task *ItemModel) error {
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

func (m *MockStorage) DeleteTask(_ context.Context, id int) error {
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

func (m *MockStorage) ReplaceAllTasks(_ context.Context, tasks []*ItemModel) error {
	if m.err != nil {
		return m.err
	}
	m.tasks = tasks
	return nil
}

func (m *MockStorage) ReplaceAllTasksFunc(ctx context.Context, fn func([]*ItemModel) ([]*ItemModel, error)) error {
	if m.err != nil {
		return m.err
	}
	next, err := fn(m.tasks)
	if err != nil {
		return err
	}
	return m.ReplaceAllTasks(ctx, next)
}

func (m *MockStorage) UpdateTaskStatus(_ context.Context, id int, next func(TaskStatus) TaskStatus, now time.Time) (TaskStatus, error) {
	if m.err != nil {
		return "", m.err
	}
	for _, t := range m.tasks {
		if t.ID == id {
			status, err := parseTaskStatus(string(next(itemStatus(t))))
			if err != nil {
				return "", err
			}
			t.setStatus(status, now)
			return status, nil
		}
	}
	return "", fmt.Errorf("%w: %d", ErrTaskNotFound, id)
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

	tasks, err := adapter.ListTasks(t.Context())
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

	tasks, err := adapter.ListTasks(t.Context())
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

	if err := adapter.ReplaceAll(t.Context(), tasks); err != nil {
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

	if err := adapter.ReplaceAll(t.Context(), []Task{{ID: "tsk_new_1", Title: "Renamed"}, {ID: "", Title: "Blank"}, {ID: "0", Title: "Zero"}}); err != nil {
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
	err := adapter.ReplaceAll(t.Context(), tasks)
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
	plan, err := PlanExport(t.Context(), adapter, filter)
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
	plan, err := PlanExport(t.Context(), adapter, filter)
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
	data, err := ExportToBytes(t.Context(), adapter, filter, false)
	if err != nil {
		t.Fatalf("ExportToBytes failed: %v", err)
	}

	var bundle ExportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("failed to unmarshal exported data: %v", err)
	}

	if bundle.Version != exportSchemaVersion {
		t.Errorf("expected version %d, got %d", exportSchemaVersion, bundle.Version)
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
	data, err := ExportToBytes(t.Context(), adapter, filter, true)
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
	err := ExportToFile(t.Context(), adapter, filter, filePath, false)
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

func TestMergePlaceholderIDsAreUniqueAndAvoidExistingIDs(t *testing.T) {
	current := []Task{{ID: "tsk_new_1", Title: "Existing placeholder-like ID"}}
	incoming := []Task{
		{ID: "tsk_new_2", Title: "Incoming placeholder-like ID"},
		{ID: "", Title: "No ID A"},
		{ID: "", Title: "No ID B"},
	}

	merged, res := merge(current, incoming, ImportConfig{Mode: "merge"})
	if res.Created != 3 || len(merged) != 4 {
		t.Fatalf("expected 3 created / 4 merged, got %+v / %d", res, len(merged))
	}
	seen := map[string]bool{}
	for _, task := range merged {
		if seen[task.ID] {
			t.Fatalf("duplicate ID %q in merge result %+v", task.ID, merged)
		}
		seen[task.ID] = true
	}
	for _, task := range merged[2:] {
		if !strings.HasPrefix(task.ID, "tsk_new_") || task.ID == "tsk_new_1" || task.ID == "tsk_new_2" {
			t.Errorf("expected a fresh placeholder for %q, got %q", task.Title, task.ID)
		}
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

	tasks, version, err := readAndParseImportFile(filePath, false)
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

	_, _, err = readAndParseImportFile(filePath, false)
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

	// Default import keeps the task under a placeholder title (plan 3, D-3);
	// strict import rejects it.
	tasks, _, err := readAndParseImportFile(filePath, false)
	if err != nil || len(tasks) != 1 || tasks[0].Title != untitledTaskTitle {
		t.Fatalf("expected default import to keep the task as %q, got %+v, %v", untitledTaskTitle, tasks, err)
	}
	if _, _, err := readAndParseImportFile(filePath, true); err == nil || !strings.Contains(err.Error(), "tasks[0].title is required") {
		t.Fatalf("expected strict import to reject a missing title, got %v", err)
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

	_, _, err = readAndParseImportFile(filePath, false)
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
	_, _, err = readAndParseImportFile(filePath, false)
	if err != nil {
		t.Fatalf("non-strict mode failed: %v", err)
	}

	// Strict mode should fail
	_, _, err = readAndParseImportFile(filePath, true)
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
	plan, err := PlanImport(t.Context(), adapter, filePath, cfg)
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

	plan, err := PlanImport(t.Context(), NewTestAdapter(storage), filePath, ImportConfig{Mode: "merge", SkipExisting: true})
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

	if err := db.CreateTask(t.Context(), &ItemModel{Title: "Existing Task", Description: "Local"}); err != nil {
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

	res, err := ApplyImport(t.Context(), &TaskServiceAdapter{storage: db}, filePath, ImportConfig{Mode: "merge"})
	if err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}

	if res.Created != 1 || res.Updated != 1 || res.Skipped != 0 || res.Conflicted != 1 {
		t.Errorf("unexpected import result: %+v", res)
	}

	tasks, err := db.ListTasks(t.Context())
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

	if err := db.CreateTask(t.Context(), &ItemModel{Title: "Existing Task", Description: "Local"}); err != nil {
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

	res, err := ApplyImport(t.Context(), &TaskServiceAdapter{storage: db}, filePath, ImportConfig{Mode: "merge", SkipExisting: true})
	if err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}

	if res.Created != 1 || res.Updated != 0 || res.Skipped != 1 || res.Conflicted != 1 {
		t.Errorf("unexpected import result: %+v", res)
	}
	if len(res.SkippedIDs) != 1 || res.SkippedIDs[0] != "1" {
		t.Errorf("expected skipped IDs [1], got %v", res.SkippedIDs)
	}

	tasks, err := db.ListTasks(t.Context())
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

	path, err := writeBackup(t.Context(), tasks)
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
	tasks, err := db.ListTasks(t.Context())
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
		if err := db.CreateTask(t.Context(), task); err != nil {
			t.Fatalf("CreateTask failed: %v", err)
		}
	}
	if err := db.DeleteTask(t.Context(), 1); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}
	before := tasksByID(t, db)

	file := filepath.Join(t.TempDir(), "export.json")
	if err := ExportToFile(t.Context(), svc, ExportFilter{IncludeCompleted: true}, file, true); err != nil {
		t.Fatalf("ExportToFile failed: %v", err)
	}

	for i := 1; i <= 2; i++ {
		res, err := ApplyImport(t.Context(), svc, file, ImportConfig{Mode: "merge"})
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

	if _, err := ApplyImport(t.Context(), &TaskServiceAdapter{storage: db}, file, ImportConfig{Mode: "merge"}); err != nil {
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
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "Old", Description: "x"}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	completedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "12", Title: "Twelve", Completed: true, CompletedAt: &completedAt},
		{ID: "", Title: "No ID"},
	}})

	res, err := ApplyImport(t.Context(), &TaskServiceAdapter{storage: db}, file, ImportConfig{Mode: "replace"})
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
	tasks, _, err := readAndParseImportFile(file, false)
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
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "A", Description: "x"}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	deadline := time.Date(2031, 5, 6, 0, 0, 0, 0, time.UTC)
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "1", Title: "A", Description: "x", Deadline: &deadline},
	}})
	svc := &TaskServiceAdapter{storage: db}

	plan, err := PlanImport(t.Context(), svc, file, ImportConfig{})
	if err != nil {
		t.Fatalf("PlanImport failed: %v", err)
	}
	if plan.ToUpdate != 1 || plan.Unchanged != 0 {
		t.Fatalf("expected deadline change to be an update, got %+v", plan)
	}
	if _, err := ApplyImport(t.Context(), svc, file, ImportConfig{}); err != nil {
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
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "A", Description: "x"}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{{ID: "1", Title: "A", Description: "x"}}})
	svc := &TaskServiceAdapter{storage: db}
	cfg := ImportConfig{Mode: "merge", IDStrategy: "regenerate"}

	plan, err := PlanImport(t.Context(), svc, file, cfg)
	if err != nil {
		t.Fatalf("PlanImport failed: %v", err)
	}
	res, err := ApplyImport(t.Context(), svc, file, cfg)
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
		if _, err := PlanImport(t.Context(), svc, file, cfg); err == nil {
			t.Errorf("PlanImport(t.Context(), %+v): expected error", cfg)
		}
		if _, err := ApplyImport(t.Context(), svc, file, cfg); err == nil {
			t.Errorf("ApplyImport(t.Context(), %+v): expected error", cfg)
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
	if _, _, err := readAndParseImportFile(file, false); err == nil || !strings.Contains(err.Error(), "duplicate id") {
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
			if _, _, err := readAndParseImportFile(file, true); err == nil {
				t.Fatalf("expected strict import to reject %s", name)
			}
		})
		t.Run("default strips "+name, func(t *testing.T) {
			file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{dto}})
			tasks, _, err := readAndParseImportFile(file, false)
			if err != nil {
				t.Fatalf("expected %s to be sanitised, got %v", name, err)
			}
			if containsControl(tasks[0].Title, false) || containsControl(tasks[0].Description, true) {
				t.Fatalf("expected control characters removed, got %q / %q", tasks[0].Title, tasks[0].Description)
			}
		})
	}

	crlf := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{{ID: "1", Title: "ok", Description: "line1\r\nline2"}}})
	tasks, _, err := readAndParseImportFile(crlf, false)
	if err != nil || tasks[0].Description != "line1\nline2" {
		t.Fatalf("expected CRLF normalised to LF, got %q (err %v)", tasks[0].Description, err)
	}

	// Over-length text: strict import rejects it with the task index; default
	// import shortens it to the limit (plan 4, D-13; see
	// TestImportShortensOverLengthText).
	for name, dto := range map[string]TaskDTO{
		"title too long":       {ID: "1", Title: strings.Repeat("a", MaxTitleLength+1)},
		"description too long": {ID: "1", Title: "ok", Description: strings.Repeat("a", MaxDescriptionLength+1)},
	} {
		file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{dto}})
		if _, _, err := readAndParseImportFile(file, true); err == nil || !strings.Contains(err.Error(), "tasks[0]") {
			t.Errorf("expected strict import to reject %s with the task index, got %v", name, err)
		}
		tasks, _, err := readAndParseImportFile(file, false)
		if err != nil || len(tasks[0].Title) > MaxTitleLength || len(tasks[0].Description) > MaxDescriptionLength {
			t.Errorf("expected default import to shorten %s, got %+v (err %v)", name, tasks, err)
		}
	}

	ok := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{{ID: "1", Title: "ok", Description: "line1\n\tline2"}}})
	if _, _, err := readAndParseImportFile(ok, true); err != nil {
		t.Fatalf("expected newline/tab in description to be accepted, got %v", err)
	}
}

// P-051 / D-13: an export of a database holding text longer than today's
// limits (written before they existed, or by another tool) restores with the
// default import, which shortens the text and reports how many tasks it
// shortened; --strict rejects the file with the task index.
func TestImportShortensOverLengthText(t *testing.T) {
	old := newFileTestDB(t)
	long := strings.Repeat("t", 5000)
	seedTask(t, old, &ItemModel{Title: long, Description: strings.Repeat("d", 5000)})
	seedTask(t, old, &ItemModel{Title: strings.Repeat(" ", 150) + "x", Description: "blank once shortened"})
	seedTask(t, old, &ItemModel{Title: strings.Repeat("€", 40), Description: "fits"})
	seedTask(t, old, &ItemModel{Title: "short", Description: "unchanged"})
	export, err := runCmd(t, NewExportCmd(old), "", "--stdout")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	file := filepath.Join(t.TempDir(), "old.json")
	if err := os.WriteFile(file, []byte(export), 0o600); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	strictDB := newFileTestDB(t)
	if _, err := runCmd(t, NewImportCmd(strictDB), "", "--file", file, "--strict"); err == nil || !strings.Contains(err.Error(), "tasks[") || !strings.Contains(err.Error(), "exceeds maximum length") {
		t.Fatalf("expected --strict to reject the file with the task index, got %v", err)
	}
	if tasks, _ := strictDB.ListTasks(t.Context()); len(tasks) != 0 {
		t.Fatalf("strict import wrote %d tasks", len(tasks))
	}

	db := newFileTestDB(t)
	out, err := runCmd(t, NewImportCmd(db), "", "--file", file)
	if err != nil {
		t.Fatalf("default import failed: %v", err)
	}
	if !strings.Contains(out, "Shortened: 3 tasks") || !strings.Contains(out, "shortened=3") {
		t.Fatalf("expected the shortened count in the plan and the result, got:\n%s", out)
	}
	byTitle := map[string]*ItemModel{}
	tasks, _ := db.ListTasks(t.Context())
	for _, task := range tasks {
		byTitle[task.Title] = task
		if len(task.Title) > MaxTitleLength || len(task.Description) > MaxDescriptionLength || !utf8.ValidString(task.Title) {
			t.Errorf("task %d still over the limits: %d/%d bytes", task.ID, len(task.Title), len(task.Description))
		}
	}
	if task := byTitle[long[:MaxTitleLength]]; task == nil || task.Description != strings.Repeat("d", MaxDescriptionLength) {
		t.Errorf("expected the long title and description cut to the limits, got %v", task)
	}
	if byTitle[untitledTaskTitle] == nil {
		t.Errorf("expected a title that is blank once shortened to become %q", untitledTaskTitle)
	}
	if byTitle[strings.Repeat("€", 33)] == nil {
		t.Errorf("expected the multi-byte title cut at a character boundary")
	}
	if byTitle["short"] == nil || len(tasks) != 4 {
		t.Errorf("expected all four tasks imported, got %d", len(tasks))
	}

	// Re-importing the shortened data with --strict now works.
	again, err := runCmd(t, NewExportCmd(db), "", "--stdout")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if _, _, err := parseImportData([]byte(again), true); err != nil {
		t.Fatalf("export of the imported tasks does not re-import with --strict: %v", err)
	}
}

// P-051: dry runs and the TUI preview report the count too.
func TestImportPlanReportsShortenedTasks(t *testing.T) {
	data, err := marshalBundle([]Task{
		{ID: "1", Title: strings.Repeat("a", MaxTitleLength+1)},
		{ID: "2", Title: "ok", Description: strings.Repeat("b", MaxDescriptionLength+1)},
		{ID: "3", Title: "ok"},
	}, false)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	svc := &TaskServiceAdapter{storage: newFileTestDB(t)}
	plan, err := planImportData(t.Context(), svc, data, ImportConfig{Mode: "merge"})
	if err != nil || plan.Shortened != 2 {
		t.Fatalf("plan.Shortened = %d (err %v), want 2", plan.Shortened, err)
	}
	res, err := applyImportData(t.Context(), svc, data, ImportConfig{Mode: "replace", DryRun: true})
	if err != nil || res.Shortened != 2 {
		t.Fatalf("dry-run Shortened = %d (err %v), want 2", res.Shortened, err)
	}
	if _, err := planImportData(t.Context(), svc, data, ImportConfig{Mode: "merge", Strict: true}); err == nil || !strings.Contains(err.Error(), "tasks[0]") {
		t.Fatalf("expected strict planning to fail with the task index, got %v", err)
	}
}

func TestBackupOfLegacyControlCharactersCanBeRestored(t *testing.T) {
	setTestHome(t)
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	// Rows written before validation existed may contain control characters.
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "legacy", Description: "line1\r\nline2\x07"}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	empty := writeTestImportBundle(t, ExportBundle{Version: 1})
	res, err := ApplyImport(t.Context(), svc, empty, ImportConfig{Mode: "replace", Backup: true})
	if err != nil {
		t.Fatalf("replace import failed: %v", err)
	}

	if _, err := ApplyImport(t.Context(), svc, res.BackupPath, ImportConfig{Mode: "replace"}); err != nil {
		t.Fatalf("restoring the backup failed: %v", err)
	}
	tasks, _ := db.ListTasks(t.Context())
	if len(tasks) != 1 || tasks[0].Description != "line1\nline2" {
		t.Fatalf("expected sanitised legacy task restored, got %+v", tasks)
	}
}

func TestImportDuplicateIDErrorIsQuoted(t *testing.T) {
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "x\x1b[2J", Title: "A"}, {ID: "x\x1b[2J", Title: "B"},
	}})
	_, _, err := readAndParseImportFile(file, false)
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
	if _, err := ApplyImport(t.Context(), svc, file, ImportConfig{Mode: "replace"}); err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}
	for id := range tasksByID(t, db) {
		if id > maxImportedTaskID {
			t.Fatalf("expected oversized IDs to be replaced, found ID %d", id)
		}
	}
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "after", Description: "x"}); err != nil {
		t.Fatalf("expected later inserts to keep working, got %v", err)
	}
}

func TestApplyImportReplaceWithRegenerateAssignsNewIDs(t *testing.T) {
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{{ID: "500", Title: "A"}, {ID: "501", Title: "B"}}})
	cfg := ImportConfig{Mode: "replace", IDStrategy: "regenerate"}

	plan, err := PlanImport(t.Context(), svc, file, cfg)
	if err != nil {
		t.Fatalf("PlanImport failed: %v", err)
	}
	res, err := ApplyImport(t.Context(), svc, file, cfg)
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
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "A", Description: "x"}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 || entries[0].Name() != "u.db" {
		t.Fatalf("expected only u.db in %s, got %v", dir, entries)
	}
	if err := ExportToFile(t.Context(), &TaskServiceAdapter{storage: db}, ExportFilter{}, real, true); err == nil {
		t.Fatal("expected export over the parameterised database to be refused")
	}
}

func TestExportToFileWithLongName(t *testing.T) {
	db := newFileTestDB(t)
	target := filepath.Join(t.TempDir(), strings.Repeat("n", 240)+".json")
	if err := ExportToFile(t.Context(), &TaskServiceAdapter{storage: db}, ExportFilter{}, target, true); err != nil {
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
	if _, err := writeBackup(t.Context(), nil); err != nil {
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
	if err := f.Truncate(maxImportFileSize + 1); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	_ = f.Close()

	if _, _, err := readAndParseImportFile(file, false); err == nil || !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("expected maximum size error, got %v", err)
	}
}

// ============================== export / backup hardening ==============================

func TestExportToFileRefusesActiveDatabase(t *testing.T) {
	db := newFileTestDB(t)
	if err := db.CreateTask(t.Context(), &ItemModel{Title: "A", Description: "x"}); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	err := ExportToFile(t.Context(), &TaskServiceAdapter{storage: db}, ExportFilter{}, db.path, true)
	if err == nil || !strings.Contains(err.Error(), "active database") {
		t.Fatalf("expected refusal to overwrite database, got %v", err)
	}
	if tasks, err := db.ListTasks(t.Context()); err != nil || len(tasks) != 1 {
		t.Fatalf("database damaged after refused export: tasks=%v err=%v", tasks, err)
	}
}

func TestExportToFileOverwritesOtherFilesWithOwnerOnlyPermissions(t *testing.T) {
	db := newFileTestDB(t)
	target := filepath.Join(t.TempDir(), "out.json")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := ExportToFile(t.Context(), &TaskServiceAdapter{storage: db}, ExportFilter{}, target, true); err != nil {
		t.Fatalf("ExportToFile failed: %v", err)
	}
	data, _ := os.ReadFile(target)
	if !bytes.Contains(data, []byte(`"version": 2`)) {
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

	if err := ExportToFile(t.Context(), &TaskServiceAdapter{storage: db}, ExportFilter{}, target, true); err != nil {
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

	first, err := writeBackup(t.Context(), tasks)
	if err != nil {
		t.Fatalf("writeBackup failed: %v", err)
	}
	second, err := writeBackup(t.Context(), tasks)
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
	b, err := ExportToBytes(t.Context(), NewTestAdapter(storage), ExportFilter{IncludeCompleted: true}, false)
	if err != nil {
		t.Fatalf("ExportToBytes failed: %v", err)
	}
	if !bytes.Contains(b, []byte(`"completed_at":"2026-03-04T05:06:07Z"`)) {
		t.Fatalf("expected completed_at in export, got %s", b)
	}
}

// ============================== plan 2: schema v2, status, tags ==============================

func TestExportV2RoundTripKeepsStatusTagsAndCompletedTasks(t *testing.T) {
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	for _, task := range []*ItemModel{
		{Title: "todo", Description: "x", Tags: []string{"home"}},
		{Title: "doing", Description: "x", Status: StatusDoing, Tags: []string{"work", "urgent"}},
		{Title: "done", Description: "x", Status: StatusDone, Completed: true},
	} {
		if err := db.CreateTask(t.Context(), task); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}
	before, _ := db.ListTasks(t.Context())

	file := filepath.Join(t.TempDir(), "export.json")
	if err := ExportToFile(t.Context(), svc, ExportFilter{IncludeCompleted: true}, file, true); err != nil {
		t.Fatalf("ExportToFile failed: %v", err)
	}
	raw, _ := os.ReadFile(file)
	for _, want := range []string{`"version": 2`, `"status": "doing"`, `"urgent"`, `"status": "done"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("export missing %s:\n%s", want, raw)
		}
	}

	if _, err := ApplyImport(t.Context(), svc, file, ImportConfig{Mode: "replace"}); err != nil {
		t.Fatalf("replace import failed: %v", err)
	}
	after, _ := db.ListTasks(t.Context())
	if len(after) != len(before) {
		t.Fatalf("expected %d tasks after round trip, got %d", len(before), len(after))
	}
	for i := range before {
		b, a := before[i], after[i]
		if a.ID != b.ID || a.Status != b.Status || !slices.Equal(a.Tags, b.Tags) || a.Completed != b.Completed {
			t.Fatalf("round trip changed task:\n before %+v\n after  %+v", b, a)
		}
	}
}

func TestImportStatusRules(t *testing.T) {
	cases := []struct {
		name       string
		version    int
		dto        TaskDTO
		strict     bool
		wantStatus TaskStatus
		wantErr    string
	}{
		{name: "v1 derives todo", version: 1, dto: TaskDTO{ID: "1", Title: "a"}, wantStatus: StatusTodo},
		{name: "v1 derives done", version: 1, dto: TaskDTO{ID: "1", Title: "a", Completed: true}, wantStatus: StatusDone},
		{name: "v1 ignores status field", version: 1, dto: TaskDTO{ID: "1", Title: "a", Status: StatusDoing}, wantStatus: StatusTodo},
		{name: "v2 doing", version: 2, dto: TaskDTO{ID: "1", Title: "a", Status: StatusDoing}, wantStatus: StatusDoing},
		{name: "v2 missing status derives", version: 2, dto: TaskDTO{ID: "1", Title: "a", Completed: true}, wantStatus: StatusDone},
		{name: "v2 status wins", version: 2, dto: TaskDTO{ID: "1", Title: "a", Status: StatusDone}, wantStatus: StatusDone},
		{name: "v2 strict contradiction", version: 2, dto: TaskDTO{ID: "1", Title: "a", Status: StatusDone}, strict: true, wantErr: "contradicts"},
		{name: "v2 unknown status falls back when not strict", version: 2, dto: TaskDTO{ID: "1", Title: "a", Status: "blocked", Completed: true}, wantStatus: StatusDone},
		{name: "v2 unknown status rejected when strict", version: 2, dto: TaskDTO{ID: "1", Title: "a", Status: "blocked"}, strict: true, wantErr: "invalid status"},
		{name: "v2 invalid tag", version: 2, dto: TaskDTO{ID: "1", Title: "a", Tags: []string{"bad tag"}}, wantErr: "letters, digits"},
		{name: "unsupported version", version: 3, dto: TaskDTO{ID: "1", Title: "a"}, wantErr: "unsupported import version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := writeTestImportBundle(t, ExportBundle{Version: tc.version, Tasks: []TaskDTO{tc.dto}})
			tasks, _, err := readAndParseImportFile(file, tc.strict)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tasks[0].Status != tc.wantStatus || tasks[0].Completed != (tc.wantStatus == StatusDone) {
				t.Fatalf("got status %q completed %v, want %q", tasks[0].Status, tasks[0].Completed, tc.wantStatus)
			}
		})
	}
}

func TestMergeDetectsStatusAndTagChanges(t *testing.T) {
	current := []Task{{ID: "1", Title: "a", Status: StatusTodo, Tags: []string{"x"}}}
	for name, in := range map[string]Task{
		"status": {ID: "1", Title: "a", Status: StatusDoing, Tags: []string{"x"}},
		"tags":   {ID: "1", Title: "a", Status: StatusTodo, Tags: []string{"y"}},
	} {
		if _, res := merge(current, []Task{in}, ImportConfig{Mode: "merge"}); res.Updated != 1 {
			t.Errorf("%s change: expected 1 update, got %+v", name, res)
		}
	}
	same := Task{ID: "1", Title: "a", Tags: []string{"x"}} // empty status derives todo
	if _, res := merge(current, []Task{same}, ImportConfig{Mode: "merge"}); res.Unchanged != 1 {
		t.Errorf("expected unchanged, got %+v", res)
	}
}

func TestPlanExportCountsStatuses(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "a", Status: StatusTodo},
		{ID: 2, Title: "b", Status: StatusDoing},
		{ID: 3, Title: "c", Status: StatusDone, Completed: true},
		{ID: 4, Title: "legacy", Completed: true},
	}}
	plan, err := PlanExport(t.Context(), NewTestAdapter(storage), ExportFilter{IncludeCompleted: true})
	if err != nil {
		t.Fatalf("PlanExport failed: %v", err)
	}
	if plan.Total != 4 || plan.Todo != 1 || plan.Doing != 1 || plan.Done != 2 {
		t.Fatalf("unexpected plan %+v", plan)
	}
	pending, _ := PlanExport(t.Context(), NewTestAdapter(storage), ExportFilter{})
	if pending.Total != 2 || pending.Done != 0 {
		t.Fatalf("unexpected pending-only plan %+v", pending)
	}
}

// staleSnapshotStorage returns an outdated list from ListTasks, while
// ReplaceAllTasksFunc sees the up-to-date tasks, as a transaction would.
type staleSnapshotStorage struct {
	MockStorage
	stale []*ItemModel
}

func (s *staleSnapshotStorage) ListTasks(context.Context) ([]*ItemModel, error) {
	return s.stale, nil
}

func TestApplyImportUsesTransactionalSnapshot(t *testing.T) {
	storage := &staleSnapshotStorage{
		stale: []*ItemModel{{ID: 1, Title: "old", Description: "x"}},
		MockStorage: MockStorage{tasks: []*ItemModel{
			{ID: 1, Title: "old", Description: "x"},
			{ID: 2, Title: "added concurrently", Description: "x"},
		}},
	}
	file := writeTestImportBundle(t, ExportBundle{Version: 2, Tasks: []TaskDTO{{ID: "3", Title: "imported"}}})

	if _, err := ApplyImport(t.Context(), NewTestAdapter(storage), file, ImportConfig{Mode: "merge"}); err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}
	if findTaskByTitle(storage.tasks, "added concurrently") == nil {
		t.Fatalf("task written after the plan was lost: %+v", storage.tasks)
	}
	if findTaskByTitle(storage.tasks, "imported") == nil || len(storage.tasks) != 3 {
		t.Fatalf("expected 3 tasks after import, got %+v", storage.tasks)
	}
}

func TestImportAndExportHonourCancelledContext(t *testing.T) {
	setTestHome(t)
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	file := writeTestImportBundle(t, ExportBundle{Version: 2, Tasks: []TaskDTO{{ID: "1", Title: "a"}}})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := ApplyImport(ctx, svc, file, ImportConfig{Backup: true}); !errors.Is(err, context.Canceled) {
		t.Errorf("ApplyImport: got %v, want context.Canceled", err)
	}
	if _, err := PlanImport(ctx, svc, file, ImportConfig{}); !errors.Is(err, context.Canceled) {
		t.Errorf("PlanImport: got %v, want context.Canceled", err)
	}
	if _, err := ExportToBytes(ctx, svc, ExportFilter{}, false); !errors.Is(err, context.Canceled) {
		t.Errorf("ExportToBytes: got %v, want context.Canceled", err)
	}
	if _, err := writeBackup(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("writeBackup: got %v, want context.Canceled", err)
	}
	if tasks, _ := db.ListTasks(t.Context()); len(tasks) != 0 {
		t.Fatalf("expected nothing imported, got %d tasks", len(tasks))
	}
}

func TestReadImportSource(t *testing.T) {
	data, err := readImportSource(stdinImportPath, strings.NewReader(`{"version":2}`))
	if err != nil || string(data) != `{"version":2}` {
		t.Fatalf("stdin read = %q, %v", data, err)
	}
	if _, err := readImportSource(stdinImportPath, nil); err == nil {
		t.Fatal("expected error when stdin is unavailable")
	}
	big := io.LimitReader(zeroReader{}, maxImportFileSize+1)
	if _, err := readImportSource(stdinImportPath, big); err == nil || !strings.Contains(err.Error(), "maximum size") {
		t.Fatalf("expected size limit error for oversized stdin, got %v", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func TestApplyImportV1MergeKeepsTagsAndDoingStatus(t *testing.T) {
	db := newFileTestDB(t)
	seed := []*ItemModel{
		{ID: 1, Title: "in progress", Description: "x", Status: StatusDoing, Tags: []string{"work"}},
		{ID: 2, Title: "finish me", Description: "x", Status: StatusDoing, Tags: []string{"home"}},
		{ID: 3, Title: "retitle me", Description: "x", Tags: []string{"misc"}},
	}
	if err := db.ReplaceAllTasks(t.Context(), seed); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	// A version 1 file, as written by Munus v2.1.1, knows nothing about tags
	// or the doing status.
	file := writeTestImportBundle(t, ExportBundle{Version: 1, Tasks: []TaskDTO{
		{ID: "1", Title: "in progress", Description: "x"},
		{ID: "2", Title: "finish me", Description: "x", Completed: true},
		{ID: "3", Title: "retitled", Description: "x"},
	}})
	svc := &TaskServiceAdapter{storage: db}

	plan, err := PlanImport(t.Context(), svc, file, ImportConfig{})
	if err != nil {
		t.Fatalf("PlanImport failed: %v", err)
	}
	if plan.Unchanged != 1 || plan.ToUpdate != 2 {
		t.Fatalf("expected 1 unchanged and 2 updates, got %+v", plan)
	}
	res, err := ApplyImport(t.Context(), svc, file, ImportConfig{})
	if err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}
	if res.Unchanged != 1 || res.Updated != 2 {
		t.Fatalf("expected apply to match the plan, got %+v", res)
	}

	got := tasksByID(t, db)
	checks := []struct {
		id     int
		title  string
		status TaskStatus
		tags   []string
	}{
		{1, "in progress", StatusDoing, []string{"work"}},
		{2, "finish me", StatusDone, []string{"home"}},
		{3, "retitled", StatusTodo, []string{"misc"}},
	}
	for _, c := range checks {
		task := got[c.id]
		if task == nil || task.Title != c.title || task.Status != c.status || !slices.Equal(task.Tags, c.tags) {
			t.Errorf("task %d = %+v, want title %q status %q tags %v", c.id, task, c.title, c.status, c.tags)
		}
	}
}

func TestApplyImportV2MergeReplacesTagsAndStatus(t *testing.T) {
	db := newFileTestDB(t)
	if err := db.ReplaceAllTasks(t.Context(), []*ItemModel{
		{ID: 1, Title: "A", Description: "x", Status: StatusDoing, Tags: []string{"work"}},
	}); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	file := writeTestImportBundle(t, ExportBundle{Version: 2, Tasks: []TaskDTO{
		{ID: "1", Title: "A", Description: "x", Status: StatusTodo},
	}})
	if _, err := ApplyImport(t.Context(), &TaskServiceAdapter{storage: db}, file, ImportConfig{}); err != nil {
		t.Fatalf("ApplyImport failed: %v", err)
	}
	got := tasksByID(t, db)[1]
	if got == nil || got.Status != StatusTodo || len(got.Tags) != 0 {
		t.Fatalf("expected a version 2 file to set status and tags exactly, got %+v", got)
	}
}

// ============================== plan 3: import integrity ==============================

// sortedIDs returns the IDs of the tasks in db in ascending order.
func sortedIDs(t *testing.T, db *Database) []int {
	t.Helper()
	ids := make([]int, 0)
	for id := range tasksByID(t, db) {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// P-031 / SEC-015: IDs above the cap that already exist are never renumbered.
func TestImportKeepsExistingIDsAboveCap(t *testing.T) {
	setTestHome(t)
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	ctx := t.Context()

	first := writeTestImportBundle(t, ExportBundle{Version: 2, Tasks: []TaskDTO{{ID: strconv.Itoa(maxImportedTaskID), Title: "imported"}}})
	if _, err := ApplyImport(ctx, svc, first, ImportConfig{}); err != nil {
		t.Fatalf("first import failed: %v", err)
	}
	seedTask(t, db, &ItemModel{Title: "added", Description: "x"})
	want := []int{maxImportedTaskID, maxImportedTaskID + 1}
	if got := sortedIDs(t, db); !slices.Equal(got, want) {
		t.Fatalf("setup: IDs %v, want %v", got, want)
	}

	// An import that changes nothing must not renumber anything.
	empty := writeTestImportBundle(t, ExportBundle{Version: 2})
	if _, err := ApplyImport(ctx, svc, empty, ImportConfig{}); err != nil {
		t.Fatalf("empty import failed: %v", err)
	}
	if got := sortedIDs(t, db); !slices.Equal(got, want) {
		t.Fatalf("empty merge import renumbered tasks: %v, want %v", got, want)
	}

	// Re-importing an export (merge and replace) keeps IDs and adds nothing,
	// and the plan matches the result.
	data, err := ExportToBytes(ctx, svc, ExportFilter{IncludeCompleted: true}, false)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	export := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(export, data, 0o600); err != nil {
		t.Fatalf("write export: %v", err)
	}
	for _, mode := range []string{"merge", "replace"} {
		cfg := ImportConfig{Mode: mode}
		plan, err := PlanImport(ctx, svc, export, cfg)
		if err != nil {
			t.Fatalf("%s: plan failed: %v", mode, err)
		}
		res, err := ApplyImport(ctx, svc, export, cfg)
		if err != nil {
			t.Fatalf("%s: import failed: %v", mode, err)
		}
		if got := sortedIDs(t, db); !slices.Equal(got, want) {
			t.Fatalf("%s: re-import changed IDs to %v, want %v", mode, got, want)
		}
		if plan.ToCreate != res.Created || plan.Unchanged != res.Unchanged {
			t.Fatalf("%s: plan %+v does not match result %+v", mode, plan, res)
		}
		if mode == "merge" && (res.Created != 0 || res.Unchanged != 2) {
			t.Fatalf("merge re-import should be a no-op, got %+v", res)
		}
	}

	// A non-canonical spelling of an existing large ID updates that task.
	update := writeTestImportBundle(t, ExportBundle{Version: 2, Tasks: []TaskDTO{{ID: "0" + strconv.Itoa(maxImportedTaskID+1), Title: "renamed"}}})
	if _, err := ApplyImport(ctx, svc, update, ImportConfig{}); err != nil {
		t.Fatalf("update import failed: %v", err)
	}
	if got := tasksByID(t, db)[maxImportedTaskID+1]; got == nil || got.Title != "renamed" || len(tasksByID(t, db)) != 2 {
		t.Fatalf("expected task %d to be updated in place, got %+v", maxImportedTaskID+1, tasksByID(t, db))
	}
}

// P-031: the cap for IDs that are new to the database is maxImportedTaskID.
func TestImportIDCapBoundary(t *testing.T) {
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	file := writeTestImportBundle(t, ExportBundle{Version: 2, Tasks: []TaskDTO{
		{ID: strconv.Itoa(maxImportedTaskID), Title: "at cap"},
		{ID: strconv.Itoa(maxImportedTaskID + 5), Title: "above cap"},
	}})
	if _, err := ApplyImport(t.Context(), svc, file, ImportConfig{Mode: "replace"}); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	byID := tasksByID(t, db)
	if byID[maxImportedTaskID] == nil || byID[maxImportedTaskID].Title != "at cap" {
		t.Fatalf("expected ID %d to be kept, got %v", maxImportedTaskID, sortedIDs(t, db))
	}
	// The task above the cap gets the next database ID instead of its own.
	if byID[maxImportedTaskID+5] != nil || byID[maxImportedTaskID+1] == nil || byID[maxImportedTaskID+1].Title != "above cap" {
		t.Fatalf("expected the task above the cap to get a new ID, got %v", sortedIDs(t, db))
	}
	if maxImportedTaskID >= 1<<31-1 {
		t.Fatalf("maxImportedTaskID must stay well below 2^31-1, is %d", maxImportedTaskID)
	}
}

// P-032 / SEC-014: blank titles are rejected by --strict and imported under a
// placeholder by default, so exports stay restorable.
func TestParseImportDataBlankTitles(t *testing.T) {
	for _, title := range []string{"", "   ", "\b", "\r\n", "\x1b\a", "\u202e", " \t "} {
		data, err := json.Marshal(ExportBundle{Version: 2, Tasks: []TaskDTO{{Title: title}}})
		if err != nil {
			t.Fatal(err)
		}
		tasks, _, err := parseImportData(data, false)
		if err != nil || len(tasks) != 1 || tasks[0].Title != untitledTaskTitle {
			t.Errorf("default import of title %q: got %+v, %v; want %q", title, tasks, err, untitledTaskTitle)
		}
		if _, _, err := parseImportData(data, true); err == nil {
			t.Errorf("strict import of title %q should fail", title)
		}
	}
}

func TestBlankTitleImportKeepsExportsRestorable(t *testing.T) {
	setTestHome(t)
	db := newFileTestDB(t)
	svc := &TaskServiceAdapter{storage: db}
	ctx := t.Context()
	seedTask(t, db, &ItemModel{Title: "real", Description: "x"})
	if _, err := applyImportData(ctx, svc, []byte(`{"version":2,"tasks":[{"title":"\u0008"}]}`), ImportConfig{}); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	data, err := ExportToBytes(ctx, svc, ExportFilter{IncludeCompleted: true}, true)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	fresh := newFileTestDB(t)
	if _, err := applyImportData(ctx, &TaskServiceAdapter{storage: fresh}, data, ImportConfig{Mode: "replace", Strict: true}); err != nil {
		t.Fatalf("strict restore of the export failed: %v", err)
	}
	if n := len(tasksByID(t, fresh)); n != 2 {
		t.Fatalf("expected 2 restored tasks, got %d", n)
	}
}

// P-035 / SEC-016: the task cap is enforced while decoding, before storage.
func TestParseImportDataTaskCap(t *testing.T) {
	build := func(n int) []byte {
		var b strings.Builder
		b.WriteString(`{"version":2,"tasks":[`)
		for i := range n {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`{"title":"t"}`)
		}
		b.WriteString(`]}`)
		return []byte(b.String())
	}
	for _, strict := range []bool{false, true} {
		if tasks, _, err := parseImportData(build(maxImportTasks), strict); err != nil || len(tasks) != maxImportTasks {
			t.Fatalf("strict=%v: a file with exactly %d tasks should import, got %d, %v", strict, maxImportTasks, len(tasks), err)
		}
		over := build(maxImportTasks + 1)
		if _, _, err := parseImportData(over, strict); !errors.Is(err, errTooManyImportTasks) {
			t.Fatalf("strict=%v: expected the task cap error, got %v", strict, err)
		}
		// No storage call happens before the cap is checked.
		storage := &MockStorage{err: errors.New("storage must not be called")}
		svc := NewTestAdapter(storage)
		if _, err := planImportData(t.Context(), svc, over, ImportConfig{Strict: strict}); !errors.Is(err, errTooManyImportTasks) {
			t.Fatalf("strict=%v: plan should fail on the cap before storage, got %v", strict, err)
		}
		if _, err := applyImportData(t.Context(), svc, over, ImportConfig{Strict: strict}); !errors.Is(err, errTooManyImportTasks) {
			t.Fatalf("strict=%v: apply should fail on the cap before storage, got %v", strict, err)
		}
	}
}

// P-035 / N-029: data after the bundle is rejected in both modes.
func TestParseImportDataRejectsTrailingData(t *testing.T) {
	bundle := `{"version":2,"tasks":[{"title":"first"}]}`
	for _, input := range []string{
		bundle + ` {"version":2,"tasks":[{"title":"second"}]}`,
		bundle + `garbage`,
		bundle + `]`,
		bundle + ` null`,
	} {
		for _, strict := range []bool{false, true} {
			if _, _, err := parseImportData([]byte(input), strict); !errors.Is(err, errTrailingImportData) {
				t.Errorf("strict=%v %q: expected errTrailingImportData, got %v", strict, input, err)
			}
		}
	}
	for _, strict := range []bool{false, true} {
		if _, _, err := parseImportData([]byte(bundle+"\n \t\r\n"), strict); err != nil {
			t.Errorf("strict=%v: trailing whitespace must be accepted, got %v", strict, err)
		}
	}
}

// P-035: the streaming decoder behaves like json.Unmarshal for well-formed
// bundles (case-insensitive keys, ignored unknown fields, null values) and
// keeps strict mode's unknown-field checks.
func TestDecodeExportBundleMatchesUnmarshal(t *testing.T) {
	inputs := []string{
		`{"version":2,"exported_at":"2026-09-27T01:02:03Z","tasks":[{"id":"1","title":"a","tags":["x"],"status":"doing"}]}`,
		`{"VERSION":1,"Tasks":[{"Title":"a","COMPLETED":true}]}`,
		`{"version":2,"extra":{"nested":[1,2,{"x":null}]},"tasks":[]}`,
		`{"version":2,"tasks":null}`,
		`{"version":null,"tasks":[{"title":"a"}]}`,
		`{"tasks":[{"title":"a"}],"tasks":[{"title":"b"},{"title":"c"}],"version":2}`,
		`{}`,
	}
	for _, input := range inputs {
		var want ExportBundle
		if err := json.Unmarshal([]byte(input), &want); err != nil {
			t.Fatalf("setup %q: %v", input, err)
		}
		got, err := decodeExportBundle([]byte(input), false)
		if err != nil {
			t.Errorf("%q: unexpected error %v", input, err)
			continue
		}
		wantJSON, _ := json.Marshal(want)
		gotJSON, _ := json.Marshal(got)
		if !bytes.Equal(wantJSON, gotJSON) {
			t.Errorf("%q:\n got  %s\n want %s", input, gotJSON, wantJSON)
		}
	}

	for _, input := range []string{
		`{"version":2,"extra":1,"tasks":[]}`,
		`{"version":2,"tasks":[{"title":"a","bogus":true}]}`,
	} {
		if _, err := decodeExportBundle([]byte(input), true); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Errorf("strict %q: expected an unknown field error, got %v", input, err)
		}
		if _, err := decodeExportBundle([]byte(input), false); err != nil {
			t.Errorf("default %q: unknown fields must be ignored, got %v", input, err)
		}
	}

	for _, input := range []string{`[]`, `"x"`, `{"version":2,"tasks":{}}`, `{"version":2,"tasks":[null]}`, `{"version":2,"tasks":[1]}`, `{"version":"2"}`, `{`, ``} {
		if _, err := decodeExportBundle([]byte(input), false); err == nil {
			t.Errorf("%q: expected an error", input)
		}
	}
	if _, _, err := parseImportData([]byte(`null`), false); err == nil || !strings.Contains(err.Error(), "unsupported import version: 0") {
		t.Errorf("null bundle: expected unsupported version, got %v", err)
	}

	// Empty and truncated files report what json.Unmarshal reports, not "EOF".
	for _, input := range []string{``, `  `, `{"version":2`, `{"version":2,"tasks":[{"title":"a"}`, `{"tasks":[`} {
		for _, strict := range []bool{false, true} {
			_, err := decodeExportBundle([]byte(input), strict)
			if !errors.Is(err, errIncompleteImportData) || errors.Is(err, io.EOF) {
				t.Errorf("strict=%v %q: expected %v, got %v", strict, input, errIncompleteImportData, err)
			}
		}
	}
}

// P-031: only the canonical spelling of an existing ID keeps it; the cap
// applies to IDs that are new to the database.
func TestStoredTaskID(t *testing.T) {
	existing := map[int]bool{-5: true, maxImportedTaskID + 1: true, 7: true}
	cases := []struct {
		id   string
		want int
	}{
		{"7", 7}, {"8", 8}, {strconv.Itoa(maxImportedTaskID), maxImportedTaskID},
		{strconv.Itoa(maxImportedTaskID + 1), maxImportedTaskID + 1},
		{strconv.Itoa(maxImportedTaskID + 2), 0},
		{"0" + strconv.Itoa(maxImportedTaskID+1), 0},
		{"-5", -5}, {"-05", 0}, {"0", 0}, {"", 0}, {"tsk_new_1", 0},
	}
	for _, tc := range cases {
		if got := storedTaskID(tc.id, existing); got != tc.want {
			t.Errorf("storedTaskID(%q) = %d, want %d", tc.id, got, tc.want)
		}
	}
	if got := storedTaskID(strconv.Itoa(maxImportedTaskID+1), nil); got != 0 {
		t.Errorf("without known IDs an oversized ID must not be kept, got %d", got)
	}
}

// P-034: exports never carry an unknown stored status.
func TestMarshalBundleExportsKnownStatuses(t *testing.T) {
	data, err := marshalBundle([]Task{
		{ID: "1", Title: "a", Status: "\x1b]0;x\x07"},
		{ID: "2", Title: "b", Status: "bogus", Completed: true},
		{ID: "3", Title: "c", Status: "DOING"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	var bundle ExportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatal(err)
	}
	want := []TaskStatus{StatusTodo, StatusDone, StatusDoing}
	for i, task := range bundle.Tasks {
		if task.Status != want[i] {
			t.Errorf("task %s exported status %q, want %q", task.ID, task.Status, want[i])
		}
	}
}

// P-043: whatever import accepts satisfies the stored-task invariants and
// exports to a file that strict import accepts again.
func FuzzParseImportData(f *testing.F) {
	f.Add([]byte(`{"version":2,"tasks":[{"id":"1","title":"a","status":"doing","tags":["X","x"]}]}`), false)
	f.Add([]byte(`{"version":1,"tasks":[{"id":"01","title":"a\u001b","completed":true}]}`), false)
	f.Add([]byte(`{"version":2,"tasks":[{"title":"t","status":"done","completed":false}]}`), true)
	f.Add([]byte(`{"version":1,"tAsks":[{"title":"\b"}]}`), false)
	f.Add([]byte(`{"version":2,"tasks":[{"id":"1000000001","title":"x"},{"id":"01000000001","title":"y"}]}`), false)
	// P-051: over-length text (multi-byte, and a title that is blank once cut).
	f.Add([]byte(`{"version":2,"tasks":[{"title":"`+strings.Repeat("\u20ac", 40)+`","description":"`+strings.Repeat("\u00e9", 300)+`"},{"title":"`+strings.Repeat(" ", 120)+`x"}]}`), false)
	f.Fuzz(func(t *testing.T, data []byte, strict bool) {
		parsed, err := decodeImportData(data, strict)
		if err != nil {
			return
		}
		tasks := parsed.tasks
		if len(tasks) > maxImportTasks {
			t.Fatalf("accepted %d tasks, more than the cap", len(tasks))
		}
		if parsed.shortened < 0 || parsed.shortened > len(tasks) || (strict && parsed.shortened != 0) {
			t.Fatalf("shortened count %d for %d tasks (strict %v)", parsed.shortened, len(tasks), strict)
		}
		if strict {
			// Whatever strict import accepts, default import accepts unchanged.
			lenient, err := decodeImportData(data, false)
			if err != nil || lenient.shortened != 0 || len(lenient.tasks) != len(tasks) {
				t.Fatalf("strict import accepted data that default import changes: %v, shortened %d", err, lenient.shortened)
			}
		}
		seen := map[string]bool{}
		for i, task := range tasks {
			if isBlank(task.Title) {
				t.Fatalf("task %d has a blank title", i)
			}
			if err := validateTaskText(task.Title, task.Description); err != nil {
				t.Fatalf("task %d passed import but fails validation: %v", i, err)
			}
			if task.ID != "" {
				if seen[task.ID] {
					t.Fatalf("duplicate id %q accepted", task.ID)
				}
				seen[task.ID] = true
			}
			if _, err := parseTaskStatus(string(task.Status)); err != nil {
				t.Fatalf("invalid status %q accepted", task.Status)
			}
			if task.Completed != (task.Status == StatusDone) || task.Completed != (task.CompletedAt != nil) {
				t.Fatalf("status %q, completed %v and completed_at %v disagree", task.Status, task.Completed, task.CompletedAt)
			}
			norm, err := normalizeTags(task.Tags)
			if err != nil || !slices.Equal(norm, task.Tags) {
				t.Fatalf("tags not normalised: %v", task.Tags)
			}
		}
		exported, err := marshalBundle(tasks, false)
		if err != nil {
			t.Fatal(err)
		}
		again, _, err := parseImportData(exported, true)
		if err != nil {
			t.Fatalf("export of accepted tasks does not re-import with --strict: %v", err)
		}
		if len(again) != len(tasks) {
			t.Fatalf("round trip changed the task count from %d to %d", len(tasks), len(again))
		}
	})
}

// P-042 / N-036: ApplyImport with DryRun reports exactly what the real import
// reports and writes nothing, not even a backup.
func TestApplyImportDryRunWritesNothing(t *testing.T) {
	home := setTestHome(t)
	file := writeTestImportBundle(t, ExportBundle{Version: 2, Tasks: []TaskDTO{
		{ID: "1", Title: "changed"}, {ID: "2", Title: "same", Description: "x"}, {ID: "5", Title: "new"}, {Title: "no id"},
	}})
	configs := []ImportConfig{
		{Mode: "merge"},
		{Mode: "merge", OnConflict: "skip"},
		{Mode: "merge", OnConflict: "rename"},
		{Mode: "merge", SkipExisting: true},
		{Mode: "merge", IDStrategy: "regenerate"},
		{Mode: "replace"},
		{Mode: "replace", IDStrategy: "regenerate"},
	}
	for _, cfg := range configs {
		dry := newFileTestDB(t)
		real := newFileTestDB(t)
		for _, db := range []*Database{dry, real} {
			seedTask(t, db, &ItemModel{Title: "original", Description: "x"})
			seedTask(t, db, &ItemModel{Title: "same", Description: "x"})
		}
		cfg.Backup = true
		want, err := ApplyImport(t.Context(), &TaskServiceAdapter{storage: real}, file, cfg)
		if err != nil {
			t.Fatalf("%+v: import failed: %v", cfg, err)
		}
		if err := os.RemoveAll(filepath.Join(home, ".munus")); err != nil {
			t.Fatal(err)
		}
		cfg.DryRun = true
		got, err := ApplyImport(t.Context(), &TaskServiceAdapter{storage: dry}, file, cfg)
		if err != nil {
			t.Fatalf("%+v: dry run failed: %v", cfg, err)
		}
		want.BackupPath = ""
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%+v: dry run reported %+v, the real import %+v", cfg, got, want)
		}
		if tasks := tasksByID(t, dry); len(tasks) != 2 || tasks[1].Title != "original" {
			t.Errorf("%+v: dry run changed the tasks: %+v", cfg, tasks)
		}
		if _, err := os.Stat(filepath.Join(home, ".munus")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%+v: dry run wrote a backup (stat: %v)", cfg, err)
		}
	}
}

// P-054: a cancelled export writes nothing, even when the storage does not
// check the context itself.
func TestExportToFileCancelledWritesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out := filepath.Join(t.TempDir(), "tasks.json")
	svc := &TaskServiceAdapter{storage: &MockStorage{tasks: []*ItemModel{{ID: 1, Title: "a"}}}}
	if err := ExportToFile(ctx, svc, ExportFilter{IncludeCompleted: true}, out, true); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a cancelled export wrote the file (stat: %v)", err)
	}
}
