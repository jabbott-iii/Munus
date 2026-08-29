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
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type mockCreateErrorStorage struct {
	MockStorage
	createErr error
}

func (m *mockCreateErrorStorage) CreateTask(task *ItemModel) error {
	return m.createErr
}

func (m *MockStorage) GetTask(id int) (*ItemModel, error) {
	for _, t := range m.tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, errors.New("task not found")
}

// TestFormModelInit tests the Init method
func TestFormModelInit(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)

	cmd := form.Init()
	if cmd != nil {
		t.Errorf("Init() expected nil command, got %v", cmd)
	}
}

// TestFormModelInit tests the initialization of FormModel
func TestNewFormModel(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)

	if form.storage != storage {
		t.Errorf("NewFormModel() storage not set correctly")
	}
	if len(form.fields) != 3 {
		t.Errorf("NewFormModel() expected 3 fields, got %d", len(form.fields))
	}
	if form.currentField != titleField {
		t.Errorf("NewFormModel() expected currentField to be titleField, got %v", form.currentField)
	}
	if form.cursor != 0 {
		t.Errorf("NewFormModel() expected cursor to be 0, got %d", form.cursor)
	}
	if form.done {
		t.Errorf("NewFormModel() expected done to be false")
	}
	if form.submitted {
		t.Errorf("NewFormModel() expected submitted to be false")
	}
}

// TestAddCursor tests the addCursor method
func TestAddCursor(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		cursor   int
		expected string
	}{
		{
			name:     "cursor at beginning",
			text:     "hello",
			cursor:   0,
			expected: "█hello",
		},
		{
			name:     "cursor in middle",
			text:     "hello",
			cursor:   2,
			expected: "he█llo",
		},
		{
			name:     "cursor at end",
			text:     "hello",
			cursor:   5,
			expected: "hello█",
		},
		{
			name:     "cursor beyond length",
			text:     "hello",
			cursor:   10,
			expected: "hello█",
		},
		{
			name:     "empty text",
			text:     "",
			cursor:   0,
			expected: "█",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &MockStorage{}
			form := NewFormModel(storage)
			form.cursor = tt.cursor
			result := form.addCursor(tt.text)
			if result != tt.expected {
				t.Errorf("addCursor() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestUpdateKeyNavigation tests keyboard navigation
func TestUpdateKeyNavigation(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		startField    formField
		expectedField formField
	}{
		{
			name:          "tab moves to next field",
			key:           "tab",
			startField:    titleField,
			expectedField: descriptionField,
		},
		{
			name:          "down moves to next field",
			key:           "down",
			startField:    descriptionField,
			expectedField: deadlineField,
		},
		{
			name:          "shift+tab moves to previous field",
			key:           "shift+tab",
			startField:    descriptionField,
			expectedField: titleField,
		},
		{
			name:          "up moves to previous field",
			key:           "up",
			startField:    deadlineField,
			expectedField: descriptionField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &MockStorage{}
			form := NewFormModel(storage)
			form.currentField = tt.startField

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{}}
			switch tt.key {
			case "tab":
				msg.Type = tea.KeyTab
			case "down":
				msg.Type = tea.KeyDown
			case "shift+tab":
				msg.Type = tea.KeyShiftTab
			case "up":
				msg.Type = tea.KeyUp
			}

			form.Update(msg)

			if form.currentField != tt.expectedField {
				t.Errorf("Expected field %v, got %v", tt.expectedField, form.currentField)
			}
		})
	}
}

// TestUpdateQuitKeys tests quit keys
func TestUpdateQuitKeys(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		testFn func(tea.KeyMsg) tea.KeyMsg
	}{
		{
			name: "ctrl+c",
			key:  "ctrl+c",
			testFn: func(msg tea.KeyMsg) tea.KeyMsg {
				msg.Type = tea.KeyCtrlC
				return msg
			},
		},
		{
			name: "esc",
			key:  "esc",
			testFn: func(msg tea.KeyMsg) tea.KeyMsg {
				msg.Type = tea.KeyEsc
				return msg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &MockStorage{}
			form := NewFormModel(storage)
			form.done = false

			msg := tea.KeyMsg{}
			msg = tt.testFn(msg)

			form.Update(msg)

			if !form.done {
				t.Errorf("Expected done to be true for %s", tt.name)
			}
		})
	}
}

// TestUpdateCharacterInput tests typing characters
func TestUpdateCharacterInput(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.currentField = titleField
	form.cursor = 0

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	form.Update(msg)

	if form.fields[titleField] != "h" {
		t.Errorf("Expected field to be 'h', got %q", form.fields[titleField])
	}
	if form.cursor != 1 {
		t.Errorf("Expected cursor to be 1, got %d", form.cursor)
	}
}

// TestUpdateCharacterInputMaxLength tests that characters can't exceed max length
func TestUpdateCharacterInputMaxLength(t *testing.T) {
	tests := []struct {
		name     string
		field    formField
		maxLen   int
		input    string
		expected int
	}{
		{
			name:     "title field max length",
			field:    titleField,
			maxLen:   MaxTitleLength,
			input:    strings.Repeat("a", MaxTitleLength+10),
			expected: MaxTitleLength,
		},
		{
			name:     "description field max length",
			field:    descriptionField,
			maxLen:   MaxDescriptionLength,
			input:    strings.Repeat("b", MaxDescriptionLength+10),
			expected: MaxDescriptionLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &MockStorage{}
			form := NewFormModel(storage)
			form.currentField = tt.field

			for _, char := range tt.input {
				msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}}
				form.Update(msg)
			}

			if len(form.fields[tt.field]) > tt.expected {
				t.Errorf("Field length %d exceeded max %d", len(form.fields[tt.field]), tt.expected)
			}
		})
	}
}

// TestUpdateBackspace tests backspace handling
func TestUpdateBackspace(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.currentField = titleField
	form.fields[titleField] = "hello"
	form.cursor = 5

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	form.Update(msg)

	if form.fields[titleField] != "hell" {
		t.Errorf("Expected 'hell', got %q", form.fields[titleField])
	}
	if form.cursor != 4 {
		t.Errorf("Expected cursor to be 4, got %d", form.cursor)
	}
}

// TestUpdateCursorMovement tests cursor movement
func TestUpdateCursorMovement(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		startCursor   int
		fieldContent  string
		expectedCur   int
		testFn        func(tea.KeyMsg) tea.KeyMsg
	}{
		{
			name:         "left moves cursor left",
			key:          "left",
			startCursor:  3,
			fieldContent: "hello",
			expectedCur:  2,
			testFn: func(msg tea.KeyMsg) tea.KeyMsg {
				msg.Type = tea.KeyLeft
				return msg
			},
		},
		{
			name:         "right moves cursor right",
			key:          "right",
			startCursor:  2,
			fieldContent: "hello",
			expectedCur:  3,
			testFn: func(msg tea.KeyMsg) tea.KeyMsg {
				msg.Type = tea.KeyRight
				return msg
			},
		},
		{
			name:         "home moves cursor to start",
			key:          "home",
			startCursor:  3,
			fieldContent: "hello",
			expectedCur:  0,
			testFn: func(msg tea.KeyMsg) tea.KeyMsg {
				msg.Type = tea.KeyHome
				return msg
			},
		},
		{
			name:         "end moves cursor to end",
			key:          "end",
			startCursor:  0,
			fieldContent: "hello",
			expectedCur:  5,
			testFn: func(msg tea.KeyMsg) tea.KeyMsg {
				msg.Type = tea.KeyEnd
				return msg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &MockStorage{}
			form := NewFormModel(storage)
			form.currentField = titleField
			form.fields[titleField] = tt.fieldContent
			form.cursor = tt.startCursor

			msg := tt.testFn(tea.KeyMsg{})
			form.Update(msg)

			if form.cursor != tt.expectedCur {
				t.Errorf("Expected cursor %d, got %d", tt.expectedCur, form.cursor)
			}
		})
	}
}

// TestSubmitFormSuccess tests successful form submission
func TestSubmitFormSuccess(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.fields[titleField] = "Test Task"
	form.fields[descriptionField] = "Test Description"
	form.fields[deadlineField] = ""

	err := form.submitForm()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(storage.tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(storage.tasks))
	}

	task := storage.tasks[0]
	if task.Title != "Test Task" {
		t.Errorf("Expected title 'Test Task', got %q", task.Title)
	}
	if task.Description != "Test Description" {
		t.Errorf("Expected description 'Test Description', got %q", task.Description)
	}
	if task.Completed {
		t.Errorf("Expected Completed to be false")
	}
	if task.Deadline != nil {
		t.Errorf("Expected nil deadline, got %v", task.Deadline)
	}
}

// TestSubmitFormWithDeadline tests form submission with deadline
func TestSubmitFormWithDeadline(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.fields[titleField] = "Test Task"
	form.fields[descriptionField] = "Test Description"
	form.fields[deadlineField] = "2d"

	err := form.submitForm()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(storage.tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(storage.tasks))
	}

	task := storage.tasks[0]
	if task.Deadline == nil {
		t.Errorf("Expected deadline to be set")
	}
}

// TestSubmitFormEmptyTitle tests form submission with empty title
func TestSubmitFormEmptyTitle(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.fields[titleField] = "  "
	form.fields[descriptionField] = "Test Description"
	form.fields[deadlineField] = ""

	err := form.submitForm()

	if err == nil {
		t.Errorf("Expected error for empty title")
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("Expected 'title is required' error, got %v", err)
	}
}

// TestSubmitFormEmptyDescription tests form submission with empty description
func TestSubmitFormEmptyDescription(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.fields[titleField] = "Test Task"
	form.fields[descriptionField] = "   "
	form.fields[deadlineField] = ""

	err := form.submitForm()

	if err == nil {
		t.Errorf("Expected error for empty description")
	}
	if !strings.Contains(err.Error(), "description is required") {
		t.Errorf("Expected 'description is required' error, got %v", err)
	}
}

// TestSubmitFormTitleExceedsMaxLength tests form submission with title exceeding max length
func TestSubmitFormTitleExceedsMaxLength(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.fields[titleField] = strings.Repeat("a", MaxTitleLength+1)
	form.fields[descriptionField] = "Test Description"
	form.fields[deadlineField] = ""

	err := form.submitForm()

	if err == nil {
		t.Errorf("Expected error for title exceeding max length")
	}
	if !strings.Contains(err.Error(), "title exceeds maximum length") {
		t.Errorf("Expected 'title exceeds maximum length' error, got %v", err)
	}
}

// TestSubmitFormDescriptionExceedsMaxLength tests form submission with description exceeding max length
func TestSubmitFormDescriptionExceedsMaxLength(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.fields[titleField] = "Test Task"
	form.fields[descriptionField] = strings.Repeat("b", MaxDescriptionLength+1)
	form.fields[deadlineField] = ""

	err := form.submitForm()

	if err == nil {
		t.Errorf("Expected error for description exceeding max length")
	}
	if !strings.Contains(err.Error(), "description exceeds maximum length") {
		t.Errorf("Expected 'description exceeds maximum length' error, got %v", err)
	}
}

// TestSubmitFormInvalidDeadline tests form submission with invalid deadline
func TestSubmitFormInvalidDeadline(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.fields[titleField] = "Test Task"
	form.fields[descriptionField] = "Test Description"
	form.fields[deadlineField] = "invalid deadline"

	err := form.submitForm()

	if err == nil {
		t.Errorf("Expected error for invalid deadline")
	}
}

// TestSubmitFormStorageError tests form submission when storage returns error
func TestSubmitFormStorageError(t *testing.T) {
	storage := &mockCreateErrorStorage{createErr: errors.New("storage error")}
	form := NewFormModel(storage)
	form.fields[titleField] = "Test Task"
	form.fields[descriptionField] = "Test Description"
	form.fields[deadlineField] = ""

	err := form.submitForm()

	if err == nil {
		t.Errorf("Expected error from storage")
	}
	if !strings.Contains(err.Error(), "storage error") {
		t.Errorf("Expected 'storage error' in error message, got %v", err)
	}
}

// TestViewSuccessMessage tests view displays success message
func TestViewSuccessMessage(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.submitted = true

	view := form.View()

	if !strings.Contains(view, "Task created successfully") {
		t.Errorf("Expected success message in view")
	}
}

// TestViewDone tests view returns empty string when done
func TestViewDone(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.done = true

	view := form.View()

	if view != "" {
		t.Errorf("Expected empty view when done, got %q", view)
	}
}

// TestViewFormRendering tests view renders form correctly
func TestViewFormRendering(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.fields[titleField] = "Test Task"
	form.fields[descriptionField] = "Test Description"
	form.fields[deadlineField] = ""

	view := form.View()

	if !strings.Contains(view, "Create New Task") {
		t.Errorf("Expected 'Create New Task' in view")
	}
	if !strings.Contains(view, "Test Task") {
		t.Errorf("Expected task title in view")
	}
	if !strings.Contains(view, "Test Description") {
		t.Errorf("Expected task description in view")
	}
}

// TestViewErrorMessage tests view displays error message
func TestViewErrorMessage(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.err = errors.New("test error")

	view := form.View()

	if !strings.Contains(view, "Error: test error") {
		t.Errorf("Expected error message in view")
	}
}

// TestFormFieldBoundaries tests form field boundaries
func TestFormFieldBoundaries(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)

	// Test that can't go before titleField
	form.currentField = titleField
	msg := tea.KeyMsg{Type: tea.KeyShiftTab}
	form.Update(msg)
	if form.currentField != titleField {
		t.Errorf("Expected to stay at titleField")
	}

	// Test that can't go beyond deadlineField
	form.currentField = deadlineField
	msg = tea.KeyMsg{Type: tea.KeyTab}
	form.Update(msg)
	if form.currentField != deadlineField {
		t.Errorf("Expected to stay at deadlineField")
	}
}

// TestSubmitFormTrimsWhitespace tests form submission trims whitespace
func TestSubmitFormTrimsWhitespace(t *testing.T) {
	storage := &MockStorage{}
	form := NewFormModel(storage)
	form.fields[titleField] = "  Test Task  "
	form.fields[descriptionField] = "\t  Test Description  \n"
	form.fields[deadlineField] = ""

	err := form.submitForm()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	task := storage.tasks[0]
	if task.Title != "Test Task" {
		t.Errorf("Expected trimmed title 'Test Task', got %q", task.Title)
	}
	if task.Description != "Test Description" {
		t.Errorf("Expected trimmed description 'Test Description', got %q", task.Description)
	}
}
