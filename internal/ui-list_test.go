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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newVimListModel(storage Storage) *ListModel {
	return NewListModelWithOptions(storage, tuiOptions{vimEnabled: true})
}

// TestListModelInit tests the Init method of ListModel
func TestListModelInit(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)

	cmd := list.Init()
	if cmd == nil {
		t.Errorf("Init() expected non-nil command, got nil")
	}
}

// TestListModelUpdateWindowSize tests window size message handling
func TestListModelUpdateWindowSize(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	_, _ = list.Update(msg)

	if list.viewportWidth != 120 {
		t.Errorf("Expected viewportWidth 120, got %d", list.viewportWidth)
	}
	if list.viewportHeight != 40 {
		t.Errorf("Expected viewportHeight 40, got %d", list.viewportHeight)
	}
}

// TestListModelUpdateDataLoaded tests data loaded message handling
func TestListModelUpdateDataLoaded(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)
	list.loading = true

	now := time.Now()
	task1 := &ItemModel{ID: 1, Title: "Task 1", Description: "Desc 1", Completed: false}
	task2 := &ItemModel{ID: 2, Title: "Task 2", Description: "Desc 2", Deadline: &now, Completed: false}
	task3 := &ItemModel{ID: 3, Title: "Task 3", Description: "Desc 3", Completed: true}

	msg := DataLoadedMsg{tasks: []*ItemModel{task1, task2, task3}}
	_, _ = list.Update(msg)

	if list.loading {
		t.Errorf("Expected loading to be false after DataLoadedMsg")
	}
	if len(list.tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(list.tasks))
	}
	if len(list.topUpcoming) == 0 {
		t.Errorf("Expected topUpcoming to be populated")
	}
	if len(list.tasksNoDeadline) == 0 {
		t.Errorf("Expected tasksNoDeadline to be populated")
	}
}

// TestListModelUpdateError tests error message handling
func TestListModelUpdateError(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)
	list.loading = true

	testErr := errors.New("test error")
	msg := ErrMsg{testErr}
	_, _ = list.Update(msg)

	if !errors.Is(testErr, list.err) {
		t.Errorf("Expected err to be set, got %v", list.err)
	}
	if list.loading {
		t.Errorf("Expected loading to be false after error")
	}
}

// TestListModelNavigationUp tests up arrow key navigation
func TestListModelNavigationUp(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
		{ID: 2, Title: "Task 2"},
		{ID: 3, Title: "Task 3"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 2

	msg := tea.KeyMsg{Type: tea.KeyUp}
	_, _ = list.Update(msg)

	if list.cursor != 1 {
		t.Errorf("Expected cursor 1 after up, got %d", list.cursor)
	}
}

// TestListModelNavigationDown tests down arrow key navigation
func TestListModelNavigationDown(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
		{ID: 2, Title: "Task 2"},
		{ID: 3, Title: "Task 3"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 0

	msg := tea.KeyMsg{Type: tea.KeyDown}
	_, _ = list.Update(msg)

	if list.cursor != 1 {
		t.Errorf("Expected cursor 1 after down, got %d", list.cursor)
	}
}

func TestListModelNavigationVimJK(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
		{ID: 2, Title: "Task 2"},
		{ID: 3, Title: "Task 3"},
	}}
	list := newVimListModel(storage)
	list.tasks = storage.tasks
	list.loading = false

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if list.cursor != 1 {
		t.Fatalf("Expected cursor 1 after j, got %d", list.cursor)
	}

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if list.cursor != 0 {
		t.Fatalf("Expected cursor 0 after k, got %d", list.cursor)
	}
}

func TestListModelDefaultModeIgnoresVimNavigation(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
		{ID: 2, Title: "Task 2"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.loading = false

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if list.cursor != 0 {
		t.Fatalf("Expected cursor to remain at 0 when vim mode is disabled, got %d", list.cursor)
	}
}

// TestListModelNavigationUpAtStart tests up navigation at start doesn't go negative
func TestListModelNavigationUpAtStart(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
		{ID: 2, Title: "Task 2"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 0

	msg := tea.KeyMsg{Type: tea.KeyUp}
	_, _ = list.Update(msg)

	if list.cursor != 0 {
		t.Errorf("Expected cursor to stay at 0, got %d", list.cursor)
	}
}

// TestListModelNavigationDownAtEnd tests down navigation at the end doesn't exceed bounds
func TestListModelNavigationDownAtEnd(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
		{ID: 2, Title: "Task 2"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 1

	msg := tea.KeyMsg{Type: tea.KeyDown}
	_, _ = list.Update(msg)

	if list.cursor != 1 {
		t.Errorf("Expected cursor to stay at 1, got %d", list.cursor)
	}
}

// TestListModelExpandToggle tests expand/collapse toggle
func TestListModelExpandToggle(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1", Description: "Desc 1"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 0
	list.expanded = make(map[int]bool)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	_, _ = list.Update(msg)

	if !list.expanded[1] {
		t.Errorf("Expected task 1 to be expanded")
	}

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	_, _ = list.Update(msg)

	if list.expanded[1] {
		t.Errorf("Expected task 1 to be collapsed after toggle")
	}
}

// TestListModelCompleteToggle tests task completion toggle
func TestListModelCompleteToggle(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1", Completed: false},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 0

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	_, _ = list.Update(msg)

	// The actual completion state is managed through storage update
	// We're just testing that the command is executed
}

// TestListModelDeleteConfirmation tests delete the confirmation dialog trigger
func TestListModelDeleteConfirmation(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 0
	list.confirmingDelete = false

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
	_, _ = list.Update(msg)

	if !list.confirmingDelete {
		t.Errorf("Expected confirmingDelete to be true")
	}
	if list.taskToDelete.ID != 1 {
		t.Errorf("Expected taskToDelete ID 1, got %d", list.taskToDelete.ID)
	}
}

// TestListModelDeleteCancel tests deleting cancellation with the 'n' key
func TestListModelDeleteCancel(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.confirmingDelete = true
	list.taskToDelete = &ItemModel{ID: 1, Title: "Task 1"}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	_, _ = list.Update(msg)

	if list.confirmingDelete {
		t.Errorf("Expected confirmingDelete to be false")
	}
	if list.taskToDelete != nil {
		t.Errorf("Expected taskToDelete to be nil")
	}
}

func TestListModelDeleteCancelWithEscape(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.confirmingDelete = true
	list.deletePrimed = true
	list.taskToDelete = &ItemModel{ID: 1, Title: "Task 1"}

	_, cmd := list.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if list.confirmingDelete {
		t.Errorf("Expected confirmingDelete to be false")
	}
	if list.deletePrimed {
		t.Errorf("Expected deletePrimed to be false")
	}
	if list.taskToDelete != nil {
		t.Errorf("Expected taskToDelete to be nil")
	}
	if cmd != nil {
		t.Errorf("Expected no quit command on escape during delete confirmation")
	}
}

// TestListModelDeleteConfirm tests deleting confirmation with the 'y' key
func TestListModelDeleteConfirm(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.confirmingDelete = true
	list.taskToDelete = &ItemModel{ID: 1, Title: "Task 1"}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	_, _ = list.Update(msg)

	if list.confirmingDelete {
		t.Errorf("Expected confirmingDelete to be false")
	}
}

func TestListModelDeleteConfirmWithYAfterD(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
	}}
	list := NewListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 0

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if list.confirmingDelete {
		t.Fatalf("Expected confirmingDelete to be false after d then y")
	}
	if list.taskToDelete != nil {
		t.Fatalf("Expected taskToDelete to be cleared after d then y")
	}
}

func TestListModelDeleteConfirmWithSecondD(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
	}}
	list := newVimListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 0

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !list.confirmingDelete {
		t.Fatalf("Expected confirmingDelete to be true after first d")
	}

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if list.confirmingDelete {
		t.Errorf("Expected confirmingDelete to be false after second d")
	}
}

func TestListModelDeleteConfirmWithSecondDKeepsConfirmationOnError(t *testing.T) {
	deleteErr := errors.New("delete failed")
	storage := &MockStorage{
		tasks: []*ItemModel{{ID: 1, Title: "Task 1"}},
		err:   deleteErr,
	}
	list := newVimListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 0

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	_, cmd := list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if !list.confirmingDelete {
		t.Fatalf("Expected confirmingDelete to remain true after failed second d")
	}
	if list.taskToDelete == nil || list.taskToDelete.ID != 1 {
		t.Fatalf("Expected taskToDelete to remain set after failed second d, got %+v", list.taskToDelete)
	}
	if !errors.Is(list.err, deleteErr) {
		t.Fatalf("Expected delete error to be stored, got %v", list.err)
	}
	if cmd != nil {
		t.Fatalf("Expected no reload command after failed second d")
	}
}

func TestListModelDeleteConfirmRequiresConsecutiveD(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
	}}
	list := newVimListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 0

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})

	if !list.confirmingDelete {
		t.Fatalf("Expected confirmation to remain open after h")
	}

	_, cmd := list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		t.Fatalf("Expected first d after interruption to re-prime, not delete")
	}
	if !list.confirmingDelete {
		t.Fatalf("Expected confirmation to remain open after non-consecutive d")
	}
	if list.taskToDelete == nil || list.taskToDelete.ID != 1 {
		t.Fatalf("Expected taskToDelete to remain set, got %+v", list.taskToDelete)
	}
}

// TestListModelQuitKeys tests quit key bindings
func TestListModelQuitKeys(t *testing.T) {
	tests := []struct {
		name      string
		keyType   tea.KeyType
		keyString string
	}{
		{"q key", tea.KeyRunes, "q"},
		{"ctrl+c", tea.KeyCtrlC, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &MockStorage{}
			list := NewListModel(storage)

			var msg tea.KeyMsg
			if tt.keyType == tea.KeyRunes {
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.keyString)}
			} else {
				msg = tea.KeyMsg{Type: tt.keyType}
			}

			_, cmd := list.Update(msg)
			if cmd == nil {
				t.Errorf("Expected Quit command for %s", tt.name)
			}
		})
	}
}

func TestListModelEscapeDoesNotQuitOutsideDeleteConfirmation(t *testing.T) {
	list := NewListModel(&MockStorage{})

	_, cmd := list.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatalf("Expected escape to be a no-op outside cancel flows")
	}
}

// TestListModelRefresh tests refresh key binding
func TestListModelRefresh(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)
	list.loading = false

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	_, cmd := list.Update(msg)

	if !list.loading {
		t.Errorf("Expected loading to be true after refresh")
	}
	if cmd == nil {
		t.Errorf("Expected loadData command")
	}
}

// TestListModelHelpToggle tests help display toggle
func TestListModelHelpToggle(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)
	list.showHelp = false

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	_, _ = list.Update(msg)

	if !list.showHelp {
		t.Errorf("Expected showHelp to be true")
	}

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	_, _ = list.Update(msg)

	if list.showHelp {
		t.Errorf("Expected showHelp to be false after toggle")
	}
}

func TestListModelVimJumpTopBottom(t *testing.T) {
	tasks := make([]*ItemModel, 25)
	for i := range 25 {
		tasks[i] = &ItemModel{ID: i + 1, Title: "Task"}
	}

	list := newVimListModel(&MockStorage{tasks: tasks})
	list.tasks = tasks
	list.loading = false
	list.cursor = 5

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if list.cursor != len(list.GetVisibleTasks())-1 {
		t.Fatalf("Expected cursor at last task after G, got %d", list.cursor)
	}
	expectedPage := (len(list.GetVisibleTasks()) - 1) / pageSize
	if list.currentPage != expectedPage {
		t.Fatalf("Expected currentPage %d after G, got %d", expectedPage, list.currentPage)
	}

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if list.cursor != 0 {
		t.Fatalf("Expected cursor 0 after g, got %d", list.cursor)
	}
	if list.currentPage != 0 {
		t.Fatalf("Expected currentPage 0 after g, got %d", list.currentPage)
	}
}

func TestListModelVimExpand(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1", Description: "Desc 1"},
	}}
	list := newVimListModel(storage)
	list.tasks = storage.tasks
	list.loading = false
	list.cursor = 0

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if !list.expanded[1] {
		t.Fatalf("Expected selected task to expand after l")
	}
}

func TestListModelViewDocumentsVimBindings(t *testing.T) {
	list := newVimListModel(&MockStorage{})
	list.loading = false

	view := list.View()
	for _, expected := range []string{"Vim mode: j/k navigate", "g/G top/bottom", "dd or y confirms", "?: Help"} {
		if !strings.Contains(view, expected) {
			t.Errorf("Expected view to contain %q, got %q", expected, view)
		}
	}
}

// TestListModelPageUp tests page up navigation
func TestListModelPageUp(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)
	tasks := make([]*ItemModel, 25)
	for i := range 25 {
		tasks[i] = &ItemModel{ID: i + 1, Title: fmt.Sprintf("Task %02d", i+1)}
	}
	list.tasks = tasks
	list.tasksNoDeadline = tasks
	list.currentPage = 2
	list.cursor = 20

	msg := tea.KeyMsg{Type: tea.KeyPgUp}
	_, _ = list.Update(msg)

	if list.currentPage != 1 {
		t.Errorf("Expected currentPage 1, got %d", list.currentPage)
	}
	if list.cursor != pageSize {
		t.Errorf("Expected cursor on first row of page 2 (%d), got %d", pageSize, list.cursor)
	}
}

// TestListModelPageDown tests page down navigation
func TestListModelPageDown(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)

	// Create enough tasks for multiple pages
	tasks := make([]*ItemModel, 25)
	for i := range 25 {
		tasks[i] = &ItemModel{ID: i + 1, Title: "Task " + string(rune(i+1))}
	}
	list.tasks = tasks
	list.topUpcoming = tasks
	list.currentPage = 0
	list.cursor = 0

	msg := tea.KeyMsg{Type: tea.KeyPgDown}
	_, _ = list.Update(msg)

	if list.currentPage != 1 {
		t.Errorf("Expected currentPage 1, got %d", list.currentPage)
	}
}

// TestListModelExportInitiate tests export initiation with the 'x' key
func TestListModelExportInitiate(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)
	list.transfer = nil

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	_, _ = list.Update(msg)

	if list.transfer == nil {
		t.Errorf("Expected transfer state to be initialized")
	}
	if list.transfer.action != transferActionExport {
		t.Errorf("Expected transfer action to be export")
	}
	if list.transfer.stage != transferStageInput {
		t.Errorf("Expected transfer stage to be input")
	}
	if list.err != nil {
		t.Errorf("Expected err to be cleared")
	}
	if list.statusMessage != "" {
		t.Errorf("Expected statusMessage to be cleared")
	}
}

// TestListModelImportInitiate tests import initiation with the 'i' key
func TestListModelImportInitiate(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)
	list.transfer = nil

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	_, _ = list.Update(msg)

	if list.transfer == nil {
		t.Errorf("Expected transfer state to be initialized")
	}
	if list.transfer.action != transferActionImport {
		t.Errorf("Expected transfer action to be import")
	}
	if list.transfer.importMode != "merge" {
		t.Errorf("Expected importMode to be merge")
	}
	if list.transfer.skipExisting {
		t.Errorf("Expected skipExisting to default to false")
	}
	if !list.transfer.backup {
		t.Errorf("Expected backup to be true")
	}
}

// TestListModelGetVisibleTasks tests visible tasks aggregation
func TestListModelGetVisibleTasks(t *testing.T) {
	now := time.Now()
	list := NewListModel(&MockStorage{})

	task1 := &ItemModel{ID: 1, Title: "Task 1", Deadline: &now}
	task2 := &ItemModel{ID: 2, Title: "Task 2"}
	task3 := &ItemModel{ID: 3, Title: "Task 3", Completed: true}

	list.topUpcoming = []*ItemModel{task1}
	list.tasksNoDeadline = []*ItemModel{task2}
	list.tasks = []*ItemModel{task1, task2, task3}

	visible := list.GetVisibleTasks()

	if len(visible) != 3 {
		t.Errorf("Expected 3 visible tasks, got %d", len(visible))
	}
}

// TestListModelEnsureCursorVisible tests cursor visibility in pagination
func TestListModelEnsureCursorVisible(t *testing.T) {
	list := NewListModel(&MockStorage{})

	// Create enough tasks for multiple pages
	tasks := make([]*ItemModel, 25)
	for i := range 25 {
		tasks[i] = &ItemModel{ID: i + 1, Title: "Task " + string(rune(i+1))}
	}
	list.topUpcoming = tasks
	list.cursor = 15
	list.currentPage = 0

	list.EnsureCursorVisible()

	if list.currentPage != 1 {
		t.Errorf("Expected currentPage 1 for cursor position 15, got %d", list.currentPage)
	}
}

// TestListModelGetCurrentTask tests getting the current task
func TestListModelGetCurrentTask(t *testing.T) {
	list := NewListModel(&MockStorage{})

	task1 := &ItemModel{ID: 1, Title: "Task 1"}
	task2 := &ItemModel{ID: 2, Title: "Task 2"}

	list.topUpcoming = []*ItemModel{task1, task2}
	list.cursor = 1

	current := list.GetCurrentTask()

	if current == nil {
		t.Errorf("Expected current task, got nil")
	}
}

// TestListModelGetCurrentTaskNilWhenOutOfBounds tests nil return when cursor out of bounds
func TestListModelGetCurrentTaskNilWhenOutOfBounds(t *testing.T) {
	list := NewListModel(&MockStorage{})

	task1 := &ItemModel{ID: 1, Title: "Task 1"}
	list.topUpcoming = []*ItemModel{task1}
	list.cursor = 10

	current := list.GetCurrentTask()

	if current != nil {
		t.Errorf("Expected nil for out of bounds cursor, got %v", current)
	}
}

// TestListModelToggleComplete tests task completion toggle
func TestListModelToggleComplete(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1", Completed: false},
	}}
	list := NewListModel(storage)
	list.topUpcoming = storage.tasks
	list.cursor = 0

	err := list.ToggleComplete()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

// TestListModelToggleCompleteNoTask tests error when no task selected
func TestListModelToggleCompleteNoTask(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 1, Title: "Task 1"},
	}}
	list := NewListModel(storage)
	list.cursor = 10

	err := list.ToggleComplete()

	if err == nil {
		t.Errorf("Expected error when no task selected")
	}
}

// TestListModelRenderTask tests task rendering
func TestListModelRenderTask(t *testing.T) {
	list := NewListModel(&MockStorage{})
	task := &ItemModel{
		ID:          1,
		Title:       "Test Task",
		Description: "Test Description",
		Completed:   false,
	}

	// Create mock styles
	selectedStyle := lipgloss.NewStyle()
	normalStyle := lipgloss.NewStyle()
	completedStyle := lipgloss.NewStyle()
	overdueStyle := lipgloss.NewStyle()
	upcomingStyle := lipgloss.NewStyle()
	descriptionStyle := lipgloss.NewStyle()

	result := list.RenderTask(task, 0, false, selectedStyle, normalStyle, completedStyle, overdueStyle, upcomingStyle, descriptionStyle)

	if !strings.Contains(result, "Test Task") {
		t.Errorf("Expected task title in render output")
	}
	if !strings.Contains(result, "[ ]") {
		t.Errorf("Expected unchecked checkbox in render output")
	}
}

// TestListModelRenderTaskCompleted tests rendering of a completed task
func TestListModelRenderTaskCompleted(t *testing.T) {
	list := NewListModel(&MockStorage{})
	task := &ItemModel{
		ID:        1,
		Title:     "Test Task",
		Completed: true,
	}

	selectedStyle := lipgloss.NewStyle()
	normalStyle := lipgloss.NewStyle()
	completedStyle := lipgloss.NewStyle()
	overdueStyle := lipgloss.NewStyle()
	upcomingStyle := lipgloss.NewStyle()
	descriptionStyle := lipgloss.NewStyle()

	result := list.RenderTask(task, 0, false, selectedStyle, normalStyle, completedStyle, overdueStyle, upcomingStyle, descriptionStyle)

	if !strings.Contains(result, "[✔]") {
		t.Errorf("Expected checked checkbox in render output")
	}
}

// TestListModelRenderTaskWithDeadline tests rendering of a task with deadline
func TestListModelRenderTaskWithDeadline(t *testing.T) {
	list := NewListModel(&MockStorage{})
	deadline := time.Now().Add(24 * time.Hour)
	task := &ItemModel{
		ID:       1,
		Title:    "Test Task",
		Deadline: &deadline,
	}

	selectedStyle := lipgloss.NewStyle()
	normalStyle := lipgloss.NewStyle()
	completedStyle := lipgloss.NewStyle()
	overdueStyle := lipgloss.NewStyle()
	upcomingStyle := lipgloss.NewStyle()
	descriptionStyle := lipgloss.NewStyle()

	result := list.RenderTask(task, 0, false, selectedStyle, normalStyle, completedStyle, overdueStyle, upcomingStyle, descriptionStyle)

	if !strings.Contains(result, "1 days left") && !strings.Contains(result, "day") {
		t.Errorf("Expected deadline info in render output, got: %s", result)
	}
}

// TestListModelViewLoading tests view displays the loading message
func TestListModelViewLoading(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.loading = true

	view := list.View()

	if !strings.Contains(view, "Loading tasks") {
		t.Errorf("Expected 'Loading tasks' in view")
	}
}

// TestListModelViewError tests view displays error message
func TestListModelViewError(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.loading = false
	list.err = errors.New("test error")

	view := list.View()

	if !strings.Contains(view, "Error:") || !strings.Contains(view, "test error") {
		t.Errorf("Expected error message in view, got: %s", view)
	}
}

// TestListModelViewDeleteConfirmation tests delete confirmation dialog rendering
func TestListModelViewDeleteConfirmation(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.loading = false
	list.confirmingDelete = true
	list.taskToDelete = &ItemModel{ID: 1, Title: "Test Task"}
	list.viewportWidth = 80
	list.viewportHeight = 24

	view := list.View()

	// Check for the warning emoji variant and confirmation prompt
	if !strings.Contains(view, "Delete Confirmation") {
		t.Errorf("Expected delete confirmation dialog in view")
	}
	if !strings.Contains(view, "Are you sure you want to delete this task?") {
		t.Errorf("Expected delete confirmation prompt in view")
	}
	// Check for action buttons - they're rendered via lipgloss with colors
	if !strings.Contains(view, "[y]") || !strings.Contains(view, "[n]") {
		t.Errorf("Expected delete confirmation actions in view")
	}
}

// TestListModelHandleTransferKeyEscape tests escape key in transfer mode
func TestListModelHandleTransferKeyEscape(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.transfer = &transferState{
		action: transferActionExport,
		stage:  transferStageInput,
		path:   "export.json",
	}

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	_, _ = list.handleTransferKey(msg)

	if list.transfer != nil {
		t.Errorf("Expected transfer state to be cleared")
	}
}

// TestListModelHandleTransferKeyBackspace tests backspace in transfer mode
func TestListModelHandleTransferKeyBackspace(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.transfer = &transferState{
		action: transferActionExport,
		stage:  transferStageInput,
		path:   "export.json",
		cursor: 6,
	}

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	_, _ = list.handleTransferKey(msg)

	if list.transfer.path != "expor.json" {
		t.Errorf("Expected path 'expor.json', got %q", list.transfer.path)
	}
	if list.transfer.cursor != 5 {
		t.Errorf("Expected cursor 5, got %d", list.transfer.cursor)
	}
}

// TestListModelHandleTransferKeyCursorMovement tests cursor movement in transfer mode
func TestListModelHandleTransferKeyCursorMovement(t *testing.T) {
	tests := []struct {
		name           string
		keyType        tea.KeyType
		initialCursor  int
		expectedCursor int
		path           string
	}{
		{"left arrow", tea.KeyLeft, 5, 4, "export.json"},
		{"right arrow", tea.KeyRight, 5, 6, "export.json"},
		{"home key", tea.KeyHome, 5, 0, "export.json"},
		{"end key", tea.KeyEnd, 0, 11, "export.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := NewListModel(&MockStorage{})
			list.transfer = &transferState{
				action: transferActionExport,
				stage:  transferStageInput,
				path:   tt.path,
				cursor: tt.initialCursor,
			}

			msg := tea.KeyMsg{Type: tt.keyType}
			_, _ = list.handleTransferKey(msg)

			if list.transfer.cursor != tt.expectedCursor {
				t.Errorf("Expected cursor %d, got %d", tt.expectedCursor, list.transfer.cursor)
			}
		})
	}
}

// TestListModelHandleTransferKeyCharacterInput tests character input in transfer mode
func TestListModelHandleTransferKeyCharacterInput(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.transfer = &transferState{
		action: transferActionExport,
		stage:  transferStageInput,
		path:   "export",
		cursor: 6,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.', 'j', 's', 'o', 'n'}}
	for _, r := range msg.Runes {
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		_, _ = list.handleTransferKey(msg)
	}

	if list.transfer.path != "export.json" {
		t.Errorf("Expected path 'export.json', got %q", list.transfer.path)
	}
}

func TestListModelTransferInputKeepsVimRunesLiteral(t *testing.T) {
	list := newVimListModel(&MockStorage{})
	list.transfer = &transferState{
		action: transferActionExport,
		stage:  transferStageInput,
		path:   "",
		cursor: 0,
	}

	for _, r := range []rune{'j', 'k', 'g', 'G', 'l', 'd'} {
		_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if list.transfer.path != "jkgGld" {
		t.Fatalf("Expected Vim runes to be inserted literally in transfer input, got %q", list.transfer.path)
	}
}

func TestListModelHandleTransferKeyToggleSkipExisting(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.transfer = &transferState{
		action: transferActionImport,
		stage:  transferStageInput,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}}
	msg.Alt = true
	_, _ = list.handleTransferKey(msg)

	if !list.transfer.skipExisting {
		t.Errorf("Expected skipExisting to toggle on")
	}
}

func TestListModelLoadData(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		storage := &MockStorage{tasks: []*ItemModel{{ID: 1, Title: "Task 1"}}}
		list := NewListModel(storage)

		msg := list.loadData()
		loaded, ok := msg.(DataLoadedMsg)
		if !ok {
			t.Fatalf("expected DataLoadedMsg, got %T", msg)
		}
		if len(loaded.tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(loaded.tasks))
		}
	})

	t.Run("error", func(t *testing.T) {
		storageErr := errors.New("storage unavailable")
		storage := &MockStorage{err: storageErr}
		list := NewListModel(storage)

		msg := list.loadData()
		errMsg, ok := msg.(ErrMsg)
		if !ok {
			t.Fatalf("expected ErrMsg, got %T", msg)
		}
		if !errors.Is(errMsg.err, storageErr) {
			t.Fatalf("expected %v, got %v", storageErr, errMsg.err)
		}
	})
}

func TestListModelExportFromTransfer(t *testing.T) {
	t.Run("nil transfer", func(t *testing.T) {
		list := NewListModel(&MockStorage{})
		_, cmd := list.exportFromTransfer()
		if cmd != nil {
			t.Fatalf("expected nil command")
		}
	})

	t.Run("empty path", func(t *testing.T) {
		list := NewListModel(&MockStorage{})
		list.transfer = &transferState{
			action: transferActionExport,
			stage:  transferStageInput,
			path:   "   ",
		}

		list.exportFromTransfer()
		if list.transfer.operationError == nil {
			t.Fatalf("expected operation error for empty path")
		}
	})

	t.Run("plan export error", func(t *testing.T) {
		list := NewListModel(&MockStorage{err: errors.New("list failed")})
		list.transfer = &transferState{
			action: transferActionExport,
			stage:  transferStageInput,
			path:   filepath.Join(t.TempDir(), "export.json"),
		}

		list.exportFromTransfer()
		if list.transfer.operationError == nil {
			t.Fatalf("expected operation error")
		}
	})

	t.Run("export write error", func(t *testing.T) {
		list := NewListModel(&MockStorage{tasks: []*ItemModel{{ID: 1, Title: "Task 1", Description: "Desc"}}})
		list.transfer = &transferState{
			action: transferActionExport,
			stage:  transferStageInput,
			path:   t.TempDir(), // directory path should fail
		}

		list.exportFromTransfer()
		if list.transfer.operationError == nil {
			t.Fatalf("expected operation error")
		}
	})

	t.Run("success", func(t *testing.T) {
		list := NewListModel(&MockStorage{tasks: []*ItemModel{
			{ID: 1, Title: "Task 1", Description: "Desc 1"},
			{ID: 2, Title: "Task 2", Description: "Desc 2"},
		}})
		outPath := filepath.Join(t.TempDir(), "export.json")
		list.transfer = &transferState{
			action:           transferActionExport,
			stage:            transferStageInput,
			path:             outPath,
			includeCompleted: true,
		}

		list.exportFromTransfer()
		if list.transfer != nil {
			t.Fatalf("expected transfer state cleared after export")
		}
		if !strings.Contains(list.statusMessage, "Exported 2 tasks") {
			t.Fatalf("expected success status, got %q", list.statusMessage)
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("expected export file, got error: %v", err)
		}
		if !strings.Contains(string(data), `"tasks"`) {
			t.Fatalf("expected tasks JSON payload, got %q", string(data))
		}
	})
}

func TestListModelPlanImportFromTransfer(t *testing.T) {
	t.Run("nil transfer", func(t *testing.T) {
		list := NewListModel(&MockStorage{})
		_, cmd := list.planImportFromTransfer()
		if cmd != nil {
			t.Fatalf("expected nil command")
		}
	})

	t.Run("empty path", func(t *testing.T) {
		list := NewListModel(&MockStorage{})
		list.transfer = &transferState{
			action: transferActionImport,
			stage:  transferStageInput,
			path:   " ",
		}
		list.planImportFromTransfer()
		if list.transfer.operationError == nil {
			t.Fatalf("expected operation error for empty path")
		}
	})

	t.Run("plan error", func(t *testing.T) {
		tmpDir := t.TempDir()
		badFile := filepath.Join(tmpDir, "bad.json")
		if err := os.WriteFile(badFile, []byte("{"), 0o644); err != nil {
			t.Fatalf("failed to write bad file: %v", err)
		}

		list := NewListModel(&MockStorage{})
		list.transfer = &transferState{
			action:     transferActionImport,
			stage:      transferStageInput,
			path:       badFile,
			importMode: "merge",
		}
		list.planImportFromTransfer()
		if list.transfer.operationError == nil {
			t.Fatalf("expected operation error")
		}
	})

	t.Run("success", func(t *testing.T) {
		now := time.Now()
		filePath := writeTestImportBundle(t, ExportBundle{
			Version:    1,
			ExportedAt: now,
			Tasks: []TaskDTO{
				{
					ID:          "1",
					Title:       "Imported Task",
					Description: "Desc",
					CreatedAt:   now,
					UpdatedAt:   now,
				},
			},
		})

		list := NewListModel(&MockStorage{})
		list.transfer = &transferState{
			action:     transferActionImport,
			stage:      transferStageInput,
			path:       "  " + filePath + "  ",
			importMode: "merge",
		}

		list.planImportFromTransfer()
		if list.transfer.plan == nil {
			t.Fatalf("expected plan to be populated")
		}
		if list.transfer.stage != transferStageConfirm {
			t.Fatalf("expected confirm stage, got %v", list.transfer.stage)
		}
		if list.transfer.path != filePath {
			t.Fatalf("expected trimmed path %q, got %q", filePath, list.transfer.path)
		}
	})
}

func TestListModelApplyImportFromTransfer(t *testing.T) {
	// Backups are written under the home directory; keep them out of the real one.
	setTestHome(t)

	t.Run("nil transfer", func(t *testing.T) {
		list := NewListModel(&MockStorage{})
		_, cmd := list.applyImportFromTransfer()
		if cmd != nil {
			t.Fatalf("expected nil command")
		}
	})

	t.Run("apply error", func(t *testing.T) {
		list := NewListModel(&MockStorage{})
		list.transfer = &transferState{
			action:     transferActionImport,
			stage:      transferStageConfirm,
			path:       filepath.Join(t.TempDir(), "missing.json"),
			importMode: "merge",
		}
		list.applyImportFromTransfer()
		if list.transfer.operationError == nil {
			t.Fatalf("expected operation error")
		}
	})

	t.Run("success", func(t *testing.T) {
		storage := NewMockModel()
		if err := storage.CreateTask(&ItemModel{Title: "Existing", Description: "Local"}); err != nil {
			t.Fatalf("failed to seed local task: %v", err)
		}
		now := time.Now()
		filePath := writeTestImportBundle(t, ExportBundle{
			Version:    1,
			ExportedAt: now,
			Tasks: []TaskDTO{
				{
					ID:          "1",
					Title:       "Existing",
					Description: "Imported",
					CreatedAt:   now,
					UpdatedAt:   now,
				},
				{
					ID:          "2",
					Title:       "New",
					Description: "New Desc",
					CreatedAt:   now,
					UpdatedAt:   now,
				},
			},
		})

		list := NewListModel(storage)
		list.transfer = &transferState{
			action:       transferActionImport,
			stage:        transferStageConfirm,
			path:         filePath,
			importMode:   "merge",
			skipExisting: true,
			backup:       true,
		}

		_, cmd := list.applyImportFromTransfer()
		if list.transfer != nil {
			t.Fatalf("expected transfer to be cleared")
		}
		if !list.loading {
			t.Fatalf("expected loading state enabled")
		}
		if cmd == nil {
			t.Fatalf("expected reload command")
		}
		if !strings.Contains(list.statusMessage, "skipped=1") {
			t.Fatalf("expected skipped count in status, got %q", list.statusMessage)
		}
		if !strings.Contains(list.statusMessage, "skipped IDs=1") {
			t.Fatalf("expected skipped IDs in status, got %q", list.statusMessage)
		}

		msg := cmd()
		if _, ok := msg.(DataLoadedMsg); !ok {
			t.Fatalf("expected DataLoadedMsg from reload command, got %T", msg)
		}
	})

	t.Run("success includes backup path for replace mode", func(t *testing.T) {
		storage := NewMockModel()
		if err := storage.CreateTask(&ItemModel{Title: "Existing", Description: "Local"}); err != nil {
			t.Fatalf("failed to seed local task: %v", err)
		}
		now := time.Now()
		filePath := writeTestImportBundle(t, ExportBundle{
			Version:    1,
			ExportedAt: now,
			Tasks: []TaskDTO{
				{
					ID:          "3",
					Title:       "Replacement",
					Description: "Imported Desc",
					CreatedAt:   now,
					UpdatedAt:   now,
				},
			},
		})

		list := NewListModel(storage)
		list.transfer = &transferState{
			action:     transferActionImport,
			stage:      transferStageConfirm,
			path:       filePath,
			importMode: "replace",
			backup:     true,
		}

		_, cmd := list.applyImportFromTransfer()
		if cmd == nil {
			t.Fatalf("expected reload command")
		}
		if !strings.Contains(list.statusMessage, "backup=") {
			t.Fatalf("expected backup path in status, got %q", list.statusMessage)
		}
	})
}

func TestListModelRenderTransferOverlayShowsSkipExistingAndConflicts(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.viewportWidth = 100
	list.viewportHeight = 30
	list.transfer = &transferState{
		action:       transferActionImport,
		stage:        transferStageConfirm,
		path:         "import.json",
		importMode:   "merge",
		skipExisting: true,
		backup:       true,
		plan: &ImportPlan{
			Incoming:    2,
			Current:     1,
			ToCreate:    1,
			ToUpdate:    0,
			Unchanged:   0,
			Conflicts:   1,
			ConflictIDs: []string{"1"},
		},
	}

	view := list.renderTransferOverlay("base")
	if !strings.Contains(view, "Skip existing ID collisions: yes") {
		t.Errorf("expected skip-existing setting in overlay, got %q", view)
	}
	if !strings.Contains(view, "Conflicts: 1") {
		t.Errorf("expected conflict count in overlay, got %q", view)
	}
	if !strings.Contains(view, "Conflict IDs: 1") {
		t.Errorf("expected conflict IDs in overlay, got %q", view)
	}
}

// TestListModelAddTransferCursor tests transfer cursor rendering
func TestListModelAddTransferCursor(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.transfer = &transferState{
		path:   "export.json",
		cursor: 6,
	}

	result := list.addTransferCursor("export.json")

	if !strings.Contains(result, "█") {
		t.Errorf("Expected cursor character in result")
	}
}

// TestYesNoLabel tests yes/no label formatting
func TestYesNoLabel(t *testing.T) {
	tests := []struct {
		value    bool
		expected string
	}{
		{true, "yes"},
		{false, "no"},
	}

	for _, tt := range tests {
		result := yesNoLabel(tt.value)
		if result != tt.expected {
			t.Errorf("yesNoLabel(%v) = %q, want %q", tt.value, result, tt.expected)
		}
	}
}

// ============================== P-004 regressions ==============================

func loadList(t *testing.T, list *ListModel) {
	t.Helper()
	_, _ = list.Update(list.loadData())
}

func TestListModelShowsAllDeadlinedTasks(t *testing.T) {
	storage := &MockStorage{}
	for i := 1; i <= 13; i++ {
		deadline := time.Now().Add(time.Duration(i) * time.Hour)
		storage.tasks = append(storage.tasks, &ItemModel{ID: i, Title: fmt.Sprintf("T%02d", i), Deadline: &deadline})
	}
	list := NewListModel(storage)
	loadList(t, list)

	if got := len(list.GetVisibleTasks()); got != 13 {
		t.Fatalf("expected all 13 deadlined tasks to be visible, got %d", got)
	}
	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if !strings.Contains(list.View(), "T13") {
		t.Fatalf("expected the 13th task on page 2, got view:\n%s", list.View())
	}
}

func TestListModelPageDownSelectsTaskOnNewPage(t *testing.T) {
	storage := &MockStorage{}
	for i := 1; i <= 12; i++ {
		storage.tasks = append(storage.tasks, &ItemModel{ID: i, Title: fmt.Sprintf("N%02d", i)})
	}
	list := NewListModel(storage)
	loadList(t, list)

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	current := list.GetCurrentTask()
	if list.currentPage != 1 || list.cursor != pageSize || current == nil {
		t.Fatalf("expected cursor %d on page 1, got cursor=%d page=%d task=%v", pageSize, list.cursor, list.currentPage, current)
	}
	visible := list.GetVisibleTasks()
	if current != visible[pageSize] {
		t.Fatalf("expected the first task of page 2 to be selected")
	}

	// Moving within the page must not jump back to page 1.
	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyDown})
	if list.currentPage != 1 {
		t.Fatalf("expected to stay on page 2 after moving down, got page %d", list.currentPage)
	}
}

func TestListModelCompleteWithoutSelectionIsNoOp(t *testing.T) {
	list := NewListModel(&MockStorage{})
	loadList(t, list)

	_, cmd := list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd != nil || list.err != nil {
		t.Fatalf("expected no command and no error, got cmd=%v err=%v", cmd != nil, list.err)
	}
}

func TestListModelErrorIsDismissedByKeyAndClearedByReload(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{{ID: 1, Title: "A"}}}
	list := NewListModel(storage)
	loadList(t, list)

	list.err = errors.New("boom")
	if !strings.Contains(list.View(), "boom") {
		t.Fatalf("expected error in view")
	}
	_, cmd := list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if list.err != nil || cmd != nil {
		t.Fatalf("expected first key to dismiss the error only, err=%v", list.err)
	}
	if !strings.Contains(list.View(), "A") {
		t.Fatalf("expected list view after dismissing error")
	}

	list.err = errors.New("stale")
	loadList(t, list)
	if list.err != nil {
		t.Fatalf("expected successful reload to clear stale error, got %v", list.err)
	}
}

func TestListModelErrorScreenStillQuits(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.err = errors.New("boom")
	_, cmd := list.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command from error screen")
	}
}

func TestListModelCursorClampedAfterDeletingLastRow(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{{ID: 2, Title: "B"}, {ID: 1, Title: "A"}}}
	list := NewListModel(storage)
	loadList(t, list)

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	_, cmd := list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("expected reload after delete")
	}
	_, _ = list.Update(cmd())

	if list.cursor != 0 || list.GetCurrentTask() == nil || list.GetCurrentTask().Title != "B" {
		t.Fatalf("expected cursor clamped onto remaining task, got cursor=%d task=%v", list.cursor, list.GetCurrentTask())
	}
}

func TestListModelExpandedStateFollowsTask(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{
		{ID: 2, Title: "Beta", Description: "beta-desc"},
		{ID: 1, Title: "Alpha", Description: "alpha-desc"},
	}}
	list := NewListModel(storage)
	loadList(t, list)

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}) // expand Beta (row 0)
	storage.tasks[0].Completed = true                                      // Beta moves to the completed section
	loadList(t, list)

	view := list.View()
	if !strings.Contains(view, "beta-desc") {
		t.Fatalf("expected Beta to stay expanded after it moved")
	}
	if strings.Contains(view, "alpha-desc") {
		t.Fatalf("expected Alpha (now row 0) to stay collapsed")
	}
}

func TestListModelDeleteDialogBlocksOtherActions(t *testing.T) {
	storage := &MockStorage{tasks: []*ItemModel{{ID: 1, Title: "Keep"}}}
	list := NewListModel(storage)
	loadList(t, list)

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	for _, key := range []rune{'i', 'x', 'c', 'e', 'r'} {
		_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
	}
	if list.transfer != nil {
		t.Fatalf("expected no transfer overlay while delete dialog is open")
	}
	if storage.tasks[0].Completed {
		t.Fatalf("expected no completion while delete dialog is open")
	}
	if !list.confirmingDelete {
		t.Fatalf("expected delete dialog to remain open")
	}

	_, cmd := list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil || len(storage.tasks) != 0 {
		t.Fatalf("expected y to confirm deletion, tasks=%d", len(storage.tasks))
	}
}

type failingUpdateStorage struct {
	MockStorage
}

func (f *failingUpdateStorage) UpdateTask(*ItemModel) error { return errors.New("disk full") }

func TestListModelFailedToggleKeepsStoredState(t *testing.T) {
	storage := &failingUpdateStorage{MockStorage{tasks: []*ItemModel{{ID: 1, Title: "A"}}}}
	list := NewListModel(storage)
	loadList(t, list)

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if list.err == nil {
		t.Fatal("expected the update error to be shown")
	}
	if task := list.GetCurrentTask(); task == nil || task.Completed || task.CompletedAt != nil {
		t.Fatalf("expected in-memory task to stay incomplete after failed save, got %+v", task)
	}
}

type notFoundDeleteStorage struct {
	MockStorage
}

func (s *notFoundDeleteStorage) DeleteTask(id int) error {
	return fmt.Errorf("%w: %d", ErrTaskNotFound, id)
}

func TestListModelDeletingAlreadyRemovedTaskClosesDialog(t *testing.T) {
	storage := &notFoundDeleteStorage{MockStorage{tasks: []*ItemModel{{ID: 1, Title: "Gone"}}}}
	list := NewListModel(storage)
	loadList(t, list)

	_, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	_, cmd := list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if list.confirmingDelete || list.err != nil || cmd == nil {
		t.Fatalf("expected dialog closed and reload, got confirming=%v err=%v cmd=%v", list.confirmingDelete, list.err, cmd != nil)
	}
}
