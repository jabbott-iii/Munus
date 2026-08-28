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
	"fmt"
	"testing"
)

// TestNewFormModelInitializesWithEmptyFields verifies that NewFormModel creates
// a properly initialized FormModel with all fields set to default values.
func TestNewFormModelInitializesWithEmptyFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	fm := NewFormModel(db)

	if fm == nil {
		t.Fatal("NewFormModel() returned nil")
	}

	if fm.storage == nil {
		t.Fatal("FormModel.storage is nil")
	}

	if len(fm.fields) != 3 {
		t.Fatalf("FormModel.fields length = %d, want 3", len(fm.fields))
	}

	if fm.currentField != titleField {
		t.Fatalf("FormModel.currentField = %v, want %v", fm.currentField, titleField)
	}

	if fm.done {
		t.Fatal("FormModel.done should be false initially")
	}

	if fm.submitted {
		t.Fatal("FormModel.submitted should be false initially")
	}
}

// TestNewListModelInitializesWithEmptyTasks verifies that NewListModel creates
// a properly initialized ListModel with correct state.
func TestNewListModelInitializesWithEmptyTasks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	lm := NewListModel(db)

	if lm == nil {
		t.Fatal("NewListModel() returned nil")
	}

	if lm.storage == nil {
		t.Fatal("ListModel.storage is nil")
	}

	if lm.expanded == nil {
		t.Fatal("ListModel.expanded is nil")
	}

	if len(lm.expanded) != 0 {
		t.Fatalf("ListModel.expanded should be empty, got %d items", len(lm.expanded))
	}

	if !lm.loading {
		t.Fatal("ListModel.loading should be true initially")
	}

	if lm.confirmingDelete {
		t.Fatal("ListModel.confirmingDelete should be false initially")
	}

	if lm.cursor != 0 {
		t.Fatalf("ListModel.cursor = %d, want 0", lm.cursor)
	}
}

// TestFormModelFieldCharacterLimits verifies that form field constraints are properly
// defined for title, description, and deadline fields.
func TestFormModelFieldCharacterLimits(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		wantLimit int
	}{
		{"Title", MaxTitleLength, 100},
		{"Description", MaxDescriptionLength, 500},
		{"Deadline", MaxDeadlineLength, 365},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.limit != tt.wantLimit {
				t.Fatalf("Max%sLength = %d, want %d", tt.name, tt.limit, tt.wantLimit)
			}
		})
	}
}

// TestListModelCursorInitialValue verifies that ListModel cursor starts at 0.
func TestListModelCursorInitialValue(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	lm := NewListModel(db)

	if lm.cursor != 0 {
		t.Fatalf("ListModel.cursor = %d, want 0", lm.cursor)
	}
}

// TestListModelCurrentPageInitialValue verifies that ListModel starts on page 0.
func TestListModelCurrentPageInitialValue(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	lm := NewListModel(db)

	if lm.currentPage != 0 {
		t.Fatalf("ListModel.currentPage = %d, want 0", lm.currentPage)
	}
}

// TestFormModelCursorInitialValue verifies that FormModel cursor starts at 0.
func TestFormModelCursorInitialValue(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	fm := NewFormModel(db)

	if fm.cursor != 0 {
		t.Fatalf("FormModel.cursor = %d, want 0", fm.cursor)
	}
}

// TestDataLoadedMsgContainsTaskList verifies that DataLoadedMsg properly carries
// task data.
func TestDataLoadedMsgContainsTaskList(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	task := &ItemModel{Title: "Test Task"}
	db.CreateTask(task)

	tasks, _ := db.ListTasks()

	msg := DataLoadedMsg{tasks: tasks}

	if len(msg.tasks) != 1 {
		t.Fatalf("DataLoadedMsg.tasks length = %d, want 1", len(msg.tasks))
	}

	if msg.tasks[0].Title != "Test Task" {
		t.Fatalf("DataLoadedMsg.tasks[0].Title = %q, want Test Task", msg.tasks[0].Title)
	}
}

// TestErrMsgCarriesError verifies that ErrMsg correctly wraps error values.
func TestErrMsgCarriesError(t *testing.T) {
	testErr := "test error message"
	errMsg := ErrMsg{error: fmt.Errorf(testErr)}

	if errMsg.Error() != testErr {
		t.Fatalf("ErrMsg.Error() = %q, want %q", errMsg.Error(), testErr)
	}
}

// TestFormFieldConstants verifies form field enum values are sequential.
func TestFormFieldConstants(t *testing.T) {
	if titleField != 0 {
		t.Fatalf("titleField = %d, want 0", titleField)
	}

	if descriptionField != 1 {
		t.Fatalf("descriptionField = %d, want 1", descriptionField)
	}

	if deadlineField != 2 {
		t.Fatalf("deadlineField = %d, want 2", deadlineField)
	}
}
