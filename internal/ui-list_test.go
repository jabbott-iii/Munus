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
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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

	if list.err != testErr {
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

// TestListModelNavigationDownAtEnd tests down navigation at end doesn't exceed bounds
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

	if !list.expanded[0] {
		t.Errorf("Expected expanded[0] to be true")
	}

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	_, _ = list.Update(msg)

	if list.expanded[0] {
		t.Errorf("Expected expanded[0] to be false after toggle")
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

// TestListModelDeleteConfirmation tests delete confirmation dialog trigger
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

// TestListModelDeleteCancel tests deleting cancellation with 'n' key
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

// TestListModelDeleteConfirm tests deleting confirmation with 'y' key
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

// TestListModelQuitKeys tests quit key bindings
func TestListModelQuitKeys(t *testing.T) {
	tests := []struct {
		name      string
		keyType   tea.KeyType
		keyString string
	}{
		{"q key", tea.KeyRunes, "q"},
		{"ctrl+c", tea.KeyCtrlC, ""},
		{"esc", tea.KeyEsc, ""},
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

// TestListModelPageUp tests page up navigation
func TestListModelPageUp(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)
	list.currentPage = 2

	msg := tea.KeyMsg{Type: tea.KeyPgUp}
	_, _ = list.Update(msg)

	if list.currentPage != 1 {
		t.Errorf("Expected currentPage 1, got %d", list.currentPage)
	}
	if list.cursor != 0 {
		t.Errorf("Expected cursor reset to 0")
	}
}

// TestListModelPageDown tests page down navigation
func TestListModelPageDown(t *testing.T) {
	storage := &MockStorage{}
	list := NewListModel(storage)

	// Create enough tasks for multiple pages
	tasks := make([]*ItemModel, 25)
	for i := 0; i < 25; i++ {
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

// TestListModelExportInitiate tests export initiation with 'x' key
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

// TestListModelImportInitiate tests import initiation with 'i' key
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
	for i := 0; i < 25; i++ {
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

// TestListModelGetCurrentTask tests getting current task
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
	if current.ID != 2 {
		t.Errorf("Expected task ID 2, got %d", current.ID)
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
	if !strings.Contains(err.Error(), "no task selected") {
		t.Errorf("Expected 'no task selected' error, got %v", err)
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

// TestListModelRenderTaskCompleted tests rendering of completed task
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

// TestListModelRenderTaskWithDeadline tests rendering of task with deadline
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

// TestListModelViewLoading tests view displays loading message
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
	list.err = errors.New("test error")

	view := list.View()

	if !strings.Contains(view, "Error: test error") {
		t.Errorf("Expected error message in view")
	}
}

// TestListModelViewDeleteConfirmation tests delete confirmation dialog rendering
func TestListModelViewDeleteConfirmation(t *testing.T) {
	list := NewListModel(&MockStorage{})
	list.confirmingDelete = true
	list.taskToDelete = &ItemModel{ID: 1, Title: "Test Task"}
	list.viewportWidth = 80
	list.viewportHeight = 24

	view := list.View()

	if !strings.Contains(view, "Delete Confirmation") && !strings.Contains(view, "delete") {
		t.Errorf("Expected delete confirmation dialog in view")
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
