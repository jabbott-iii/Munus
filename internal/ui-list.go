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
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const pageSize = 10

// Init initializes the list model
func (m *ListModel) Init() tea.Cmd {
	return m.loadData
}

// The TUI event loop is the top-level boundary for interactive operations, so
// storage calls made from it use context.Background().

func (m *ListModel) loadData() tea.Msg {
	tasks, err := m.storage.ListTasks(context.Background())
	if err != nil {
		return ErrMsg{err}
	}
	return DataLoadedMsg{tasks: tasks}
}

func (m *ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewportWidth = msg.Width
		m.viewportHeight = msg.Height
		return m, nil

	case DataLoadedMsg:
		m.loading = false
		m.err = nil
		m.allTasks = msg.tasks
		m.applyFilter()
		return m, nil

	case ErrMsg:
		m.err = msg.err
		m.loading = false // Add this line if missing
		m.statusMessage = ""
		return m, nil

	case tea.KeyMsg:
		if m.transfer != nil {
			// Transfer input is its own text-entry mode, so list-level bindings such
			// as j/k/g/G/l must never intercept typed characters here.
			return m.handleTransferKey(msg)
		}

		// An error replaces the list view; the next key dismisses it.
		if m.err != nil {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			m.err = nil
			return m, nil
		}

		if m.confirmingDelete && msg.String() == "esc" {
			m.confirmingDelete = false
			m.deletePrimed = false
			m.taskToDelete = nil
			return m, nil
		}

		// The delete dialog is modal: only its own keys (and quit) act while it
		// is open, so no other overlay or action can start behind it.
		if m.confirmingDelete {
			switch msg.String() {
			case "d", "y", "n", "q", "ctrl+c":
			default:
				m.deletePrimed = false
				return m, nil
			}
		} else if msg.String() != "d" {
			m.deletePrimed = false
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "up", "shift+tab":
			if m.cursor > 0 {
				m.cursor--
				m.EnsureCursorVisible()
			}

		case "k":
			if m.vimEnabled && m.cursor > 0 {
				m.cursor--
				m.EnsureCursorVisible()
			}

		case "down", "tab":
			if len(m.GetVisibleTasks()) == 0 && len(m.tasks) > 0 {
				m.cursor = min(m.cursor+1, len(m.tasks)-1)
				return m, nil
			}
			if m.cursor < len(m.GetVisibleTasks())-1 {
				m.cursor++
				m.EnsureCursorVisible()
			}

		case "j":
			if !m.vimEnabled {
				return m, nil
			}
			if len(m.GetVisibleTasks()) == 0 && len(m.tasks) > 0 {
				m.cursor = min(m.cursor+1, len(m.tasks)-1)
				return m, nil
			}
			if m.cursor < len(m.GetVisibleTasks())-1 {
				m.cursor++
				m.EnsureCursorVisible()
			}

		case "e":
			if task := m.GetCurrentTask(); task != nil {
				m.expanded[task.ID] = !m.expanded[task.ID]
			}

		case "l":
			if task := m.GetCurrentTask(); m.vimEnabled && task != nil {
				m.expanded[task.ID] = true
			}

		case "g":
			if m.vimEnabled && len(m.GetVisibleTasks()) > 0 {
				m.cursor = 0
				m.EnsureCursorVisible()
			}

		case "G":
			visibleTasks := m.GetVisibleTasks()
			if m.vimEnabled && len(visibleTasks) > 0 {
				m.cursor = len(visibleTasks) - 1
				m.EnsureCursorVisible()
			}

		case "c":
			if m.GetCurrentTask() == nil {
				return m, nil
			}
			if err := m.ToggleComplete(); err != nil && !errors.Is(err, ErrTaskNotFound) {
				m.err = err
				return m, nil
			}
			// A task deleted elsewhere simply disappears on reload.
			return m, m.loadData

		case "s":
			if m.GetCurrentTask() == nil {
				return m, nil
			}
			if err := m.CycleStatus(); err != nil && !errors.Is(err, ErrTaskNotFound) {
				m.err = err
				return m, nil
			}
			return m, m.loadData

		case "u":
			if task := m.GetCurrentTask(); task != nil {
				return m.openEditForm(task)
			}

		case "F":
			m.filter = (m.filter + 1) % (filterDone + 1)
			m.cursor, m.currentPage = 0, 0
			m.applyFilter()

		case "#":
			m.tagFilter = nextTag(m.knownTags(), m.tagFilter)
			m.cursor, m.currentPage = 0, 0
			m.applyFilter()

		case "d":
			if m.vimEnabled && m.confirmingDelete && m.taskToDelete != nil && m.deletePrimed {
				m.deletePrimed = false
				return m.confirmDelete()
			}
			if !m.confirmingDelete {
				task := m.GetCurrentTask()
				if task != nil {
					m.confirmingDelete = true
					m.taskToDelete = task
					m.deletePrimed = m.vimEnabled
				}
				return m, nil
			}
			if m.vimEnabled {
				m.deletePrimed = true
			}
			return m, nil

		case "n":
			if m.confirmingDelete {
				m.confirmingDelete = false
				m.deletePrimed = false
				m.taskToDelete = nil
				return m, nil
			}
			fm := NewFormModelWithOptions(m.storage, tuiOptions{vimEnabled: m.vimEnabled})
			fm.viewportWidth, fm.viewportHeight = m.viewportWidth, m.viewportHeight
			fm.listFilter, fm.listTagFilter = m.filter, m.tagFilter
			return fm, nil

		case "y":
			if m.confirmingDelete && m.taskToDelete != nil {
				return m.confirmDelete()
			}
			return m, nil

		case "r":
			m.loading = true
			return m, m.loadData

		case "?", "h":
			m.showHelp = !m.showHelp

		case "pgup", "b":
			if m.currentPage > 0 {
				m.currentPage--
				m.cursor = m.currentPage * pageSize
				m.clampCursor()
			}

		case "pgdown", "f":
			visibleTasks := m.GetVisibleTasks()
			if (m.currentPage+1)*pageSize < len(visibleTasks) {
				m.currentPage++
				m.cursor = m.currentPage * pageSize
				m.clampCursor()
			}

		case "x":
			m.err = nil
			m.statusMessage = ""
			m.transfer = &transferState{
				action:           transferActionExport,
				stage:            transferStageInput,
				path:             fmt.Sprintf("munus-export-%s.json", time.Now().Format("20060102")),
				includeCompleted: true,
			}
			m.transfer.cursor = len(m.transfer.path)
			return m, nil

		case "i":
			m.err = nil
			m.statusMessage = ""
			m.transfer = &transferState{
				action:       transferActionImport,
				stage:        transferStageInput,
				importMode:   "merge",
				skipExisting: false,
				backup:       true,
			}
			return m, nil

		}
	}

	return m, nil
}

// View renders the list
func (m *ListModel) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Render("Error: " + sanitizeForTerminal(m.err.Error(), true) + "\n\nPress any key to continue.")
	}

	if m.loading {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9333EA")).
			Render("Loading tasks...")
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f7cf79")).
		Bold(true).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8B5CF6")).
		Bold(true).
		MarginTop(1).
		MarginBottom(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#f7cf79")).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a1a1a0")).
		Padding(0, 1)

	completeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a1a1a0")).
		Strikethrough(true).
		Padding(0, 1)

	overdueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d11212")).
		Bold(true)

	upcomingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a1a1a0"))

	descriptionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f7cf79")).
		PaddingLeft(3)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f7cf79")).
		PaddingLeft(1)

	var s strings.Builder

	s.WriteString(titleStyle.Render(" Task List"))
	if label := m.filterLabel(); label != "" {
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render("  Filter: " + label))
	}

	s.WriteString(sectionStyle.Render(" Upcoming Deadlines"))
	s.WriteString("\n")

	visibleTasks := m.GetVisibleTasks()
	start := m.currentPage * pageSize
	end := start + pageSize

	currentIndex := 0

	// Render top upcoming tasks
	for _, task := range m.topUpcoming {
		if currentIndex >= start && currentIndex < end {
			s.WriteString(m.RenderTask(task, currentIndex, currentIndex == m.cursor,
				selectedStyle, normalStyle, completeStyle, overdueStyle, upcomingStyle, descriptionStyle))
			s.WriteString("\n")
		}
		currentIndex++
	}

	// Tasks without deadline section
	s.WriteString(sectionStyle.Render(" No Deadline"))
	s.WriteString("\n")

	for _, task := range m.tasksNoDeadline {
		if currentIndex >= start && currentIndex < end {
			s.WriteString(m.RenderTask(task, currentIndex, currentIndex == m.cursor,
				selectedStyle, normalStyle, completeStyle, overdueStyle, upcomingStyle, descriptionStyle))
			s.WriteString("\n")
		}
		currentIndex++
	}

	// Completed tasks section
	completedCount := 0
	for _, task := range m.tasks {
		if task.Completed {
			if completedCount == 0 && currentIndex >= 0 {
				s.WriteString("\n")
				s.WriteString(sectionStyle.Render("🗹 Completed"))
				s.WriteString("\n")
			}
			if currentIndex >= start && currentIndex < end {
				s.WriteString(m.RenderTask(task, currentIndex, currentIndex == m.cursor,
					selectedStyle, normalStyle, completeStyle, overdueStyle, upcomingStyle, descriptionStyle))
				s.WriteString("\n")
			}
			currentIndex++
			completedCount++
		}
	}

	if len(visibleTasks) > pageSize {
		pageInfo := fmt.Sprintf("\n Page %d/%d", m.currentPage+1, (len(visibleTasks)+pageSize-1)/pageSize)
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render(pageInfo))
	}

	s.WriteString("\n")
	s.WriteString(helpStyle.Render("Commands:"))
	if m.vimEnabled {
		s.WriteString(helpStyle.Render("\n\nVim mode: j/k navigate • g/G top/bottom • l expand • d opens delete • dd or y confirms • n/esc cancels delete"))
		s.WriteString(helpStyle.Render("\nshift+tab/↑ | tab/↓: Navigate • e: Expand • c: Complete • n: New • r: Refresh • ctrl+c: Quit"))
	} else {
		s.WriteString(helpStyle.Render("\n\nshift+tab/↑ | tab/↓: Navigate • e: Expand • c: Complete • d: Delete prompt • y: Confirm delete • esc: Cancel delete • n: New/No • r: Refresh • ctrl+c: Quit"))
	}
	s.WriteString(helpStyle.Render("\n?: Help • u: Edit • s: Status • F: Filter • #: Tag filter • x: Export • i: Import"))
	if m.showHelp {
		s.WriteString("\n")
		s.WriteString(helpStyle.Render(m.helpText()))
	}

	if m.statusMessage != "" {
		s.WriteString("\n")
		s.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4CAF50")).
			PaddingLeft(1).
			Render(m.statusMessage))
	}

	if m.confirmingDelete && m.taskToDelete != nil {

		dialogStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#FF6B6B")).
			Padding(1, 2).
			Background(lipgloss.Color("#1A1A2E")).
			Foreground(lipgloss.Color("#FFFFFF"))

		warningStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Bold(true)

		modalTitleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

		var dialog strings.Builder
		dialog.WriteString(warningStyle.Render("Delete Confirmation"))
		dialog.WriteString("\n\n")
		dialog.WriteString("Are you sure you want to delete this task?\n\n")
		dialog.WriteString(modalTitleStyle.Render("Title: "))
		dialog.WriteString(sanitizeForTerminal(m.taskToDelete.Title, false))
		dialog.WriteString("\n\n")
		dialog.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#4CAF50")).Render("[y] Yes  "))
		dialog.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Render("[n] No  "))
		dialog.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Render("[esc] Cancel"))

		dialogContent := dialogStyle.Render(dialog.String())

		viewW := m.viewportWidth
		viewH := m.viewportHeight
		if viewW <= 0 {
			viewW = 80
		}
		if viewH <= 0 {
			viewH = 24
		}

		// Base layer (dimmed)
		base := lipgloss.NewStyle().
			Width(viewW).
			Height(viewH).
			Foreground(lipgloss.Color("#6B7280")).
			Render(s.String())

		// Modal layer centered in full viewport
		modalLayer := lipgloss.Place(
			viewW,
			viewH,
			lipgloss.Center,
			lipgloss.Center,
			dialogContent,
			lipgloss.WithWhitespaceChars(" "),
		)

		// Draw modal over base by resetting cursor to top-left before modal output.
		return base + "\x1b[H" + modalLayer
	}
	if m.transfer != nil {
		return m.renderTransferOverlay(s.String())
	}

	return s.String()
}

func (m *ListModel) RenderTask(task *ItemModel, index int, isSelected bool,
	selectedStyle, normalStyle, completedStyle, overdueStyle, upcomingStyle, descriptionStyle lipgloss.Style,
) string {
	var s strings.Builder

	checkbox := "[ ]"
	switch itemStatus(task) {
	case StatusDone:
		checkbox = "[✔]"
	case StatusDoing:
		checkbox = "[~]"
	}

	deadlineInfo := ""
	if task.Deadline != nil && !task.Completed {
		label, urgent := deadlineLabel(*task.Deadline, m.clock())
		if urgent {
			deadlineInfo = overdueStyle.Render(" (" + label + ")")
		} else {
			deadlineInfo = upcomingStyle.Render(" (" + label + ")")
		}
	}

	tagInfo := ""
	if len(task.Tags) > 0 {
		tagInfo = sanitizeForTerminal(" #"+strings.Join(task.Tags, " #"), false)
	}

	line := fmt.Sprintf("%s %s%s%s", checkbox, sanitizeForTerminal(task.Title, false), tagInfo, deadlineInfo)

	if isSelected {
		s.WriteString(selectedStyle.Render(line))
	} else if task.Completed {
		s.WriteString(completedStyle.Render(line))
	} else {
		s.WriteString(normalStyle.Render(line))
	}

	if m.expanded[task.ID] && task.Description != "" {
		s.WriteString("\n")
		s.WriteString(descriptionStyle.Render(sanitizeForTerminal(task.Description, true)))
		s.WriteString("\n")
	}

	return s.String()
}

func (m *ListModel) GetVisibleTasks() []*ItemModel {
	if len(m.topUpcoming) == 0 && len(m.tasksNoDeadline) == 0 && len(m.tasks) > 0 {
		return m.tasks
	}

	var visible []*ItemModel
	visible = append(visible, m.topUpcoming...)
	visible = append(visible, m.tasksNoDeadline...)
	for _, task := range m.tasks {
		if task.Completed {
			visible = append(visible, task)
		}
	}
	return visible
}

func (m *ListModel) EnsureCursorVisible() {
	visibleCount := len(m.GetVisibleTasks())
	pageCount := (visibleCount + pageSize - 1) / pageSize

	targetPage := m.cursor / pageSize
	if targetPage != m.currentPage && targetPage < pageCount {
		m.currentPage = targetPage
	}
}

// clock returns the current time used for deadline labels and filters.
func (m *ListModel) clock() time.Time {
	if m.now == nil {
		return time.Now()
	}
	return m.now()
}

// applyFilter recomputes the visible sections from allTasks and the active
// status and tag filters.
func (m *ListModel) applyFilter() {
	if m.filter == filterAll && m.tagFilter == "" {
		m.tasks = m.allTasks
	} else {
		m.tasks = filterItems(m.allTasks, m.currentFilter(), m.clock())
	}
	// Every incomplete task with a deadline is listed (soonest first) so
	// none become unreachable once there are more than a page of them.
	m.topUpcoming = GetTopUpcomingTasks(m.tasks, len(m.tasks))
	m.tasksNoDeadline = GetTasksWithoutDeadline(m.tasks)
	m.clampCursor()
}

func (m *ListModel) currentFilter() taskFilter {
	var f taskFilter
	switch m.filter {
	case filterPending:
		f.pending = true
	case filterDoing:
		f.status = StatusDoing
	case filterOverdue:
		f.overdue = true
	case filterDone:
		f.completed = true
	}
	if m.tagFilter != "" {
		f.tags = []string{m.tagFilter}
	}
	return f
}

// filterLabel describes the active filters, or "" when none is active.
func (m *ListModel) filterLabel() string {
	var parts []string
	switch m.filter {
	case filterPending:
		parts = append(parts, "pending")
	case filterDoing:
		parts = append(parts, "in progress")
	case filterOverdue:
		parts = append(parts, "overdue")
	case filterDone:
		parts = append(parts, "done")
	}
	if m.tagFilter != "" {
		parts = append(parts, "#"+sanitizeForTerminal(m.tagFilter, false))
	}
	return strings.Join(parts, " · ")
}

// knownTags returns the sorted, distinct tags of all loaded tasks.
func (m *ListModel) knownTags() []string {
	seen := map[string]struct{}{}
	var tags []string
	for _, t := range m.allTasks {
		for _, tag := range t.Tags {
			if _, ok := seen[tag]; !ok {
				seen[tag] = struct{}{}
				tags = append(tags, tag)
			}
		}
	}
	sort.Strings(tags)
	return tags
}

// nextTag returns the tag after current in tags, or "" (no tag filter) after
// the last one.
func nextTag(tags []string, current string) string {
	if current == "" {
		if len(tags) == 0 {
			return ""
		}
		return tags[0]
	}
	for i, tag := range tags {
		if tag == current && i+1 < len(tags) {
			return tags[i+1]
		}
	}
	return ""
}

// helpText lists every list-view key binding.
func (m *ListModel) helpText() string {
	move, expand, del := "↑/↓, tab/shift+tab", "e", "d, then y"
	if m.vimEnabled {
		move, expand, del = "↑/↓, j/k, g/G", "e or l", "d, then y or d"
	}
	lines := []string{
		"Keys:",
		"  " + move + " — move",
		"  pgup/pgdown or b/f — previous/next page",
		"  " + expand + " — expand the selected task",
		"  c — toggle complete",
		"  s — cycle status: todo → doing → done",
		"  u — edit the selected task",
		"  " + del + " — delete (n or esc cancels)",
		"  n — new task",
		"  F — cycle filter: all → pending → in progress → overdue → done",
		"  # — cycle tag filter",
		"  x / i — export / import",
		"  r — refresh",
		"  ? or h — toggle this help",
		"  q or ctrl+c — quit",
	}
	return strings.Join(lines, "\n")
}

// openEditForm switches to the task form pre-filled with task.
func (m *ListModel) openEditForm(task *ItemModel) (tea.Model, tea.Cmd) {
	fm := NewFormModelWithOptions(m.storage, tuiOptions{vimEnabled: m.vimEnabled})
	fm.editingID = task.ID
	// Control characters stored before validation existed are dropped, so
	// saving the edit also cleans the task.
	fm.fields[titleField] = stripControlCharacters(task.Title, false)
	fm.fields[descriptionField] = stripControlCharacters(task.Description, true)
	if task.Deadline != nil {
		fm.fields[deadlineField] = task.Deadline.In(time.Local).Format(deadlineInputLayout)
	}
	fm.originalDeadline = fm.fields[deadlineField]
	fm.cursor = utf8.RuneCountInString(fm.fields[titleField])
	fm.viewportWidth, fm.viewportHeight = m.viewportWidth, m.viewportHeight
	fm.listFilter, fm.listTagFilter = m.filter, m.tagFilter
	return fm, nil
}

// clampCursor keeps the cursor on an existing row (for example after the last
// row is deleted) and keeps the current page in range.
func (m *ListModel) clampCursor() {
	count := len(m.GetVisibleTasks())
	if m.cursor >= count {
		m.cursor = count - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	lastPage := 0
	if count > 0 {
		lastPage = (count - 1) / pageSize
	}
	if m.currentPage > lastPage {
		m.currentPage = lastPage
	}
	m.EnsureCursorVisible()
}

func (m *ListModel) GetCurrentTask() *ItemModel {
	visible := m.GetVisibleTasks()
	if m.cursor >= 0 && m.cursor < len(visible) {
		return visible[m.cursor]
	}
	return nil
}

func (m *ListModel) ToggleComplete() error {
	task := m.GetCurrentTask()
	if task == nil {
		return fmt.Errorf("no task selected")
	}

	previous := *task
	if task.Completed {
		task.MarkIncomplete()
	} else {
		task.MarkComplete()
	}

	if err := m.storage.UpdateTask(context.Background(), task); err != nil {
		*task = previous // keep the list consistent with what is stored
		return err
	}
	return nil
}

// CycleStatus moves the selected task to the next status (todo → doing →
// done → todo). On a storage error the task keeps its previous state.
func (m *ListModel) CycleStatus() error {
	task := m.GetCurrentTask()
	if task == nil {
		return fmt.Errorf("no task selected")
	}
	previous := *task
	task.setStatus(nextStatus(itemStatus(task)), time.Now())
	if err := m.storage.UpdateTask(context.Background(), task); err != nil {
		*task = previous
		return err
	}
	return nil
}

func (m *ListModel) confirmDelete() (tea.Model, tea.Cmd) {
	if m.taskToDelete == nil {
		return m, nil
	}

	// A task already removed elsewhere is treated as deleted.
	if err := m.storage.DeleteTask(context.Background(), m.taskToDelete.ID); err != nil && !errors.Is(err, ErrTaskNotFound) {
		m.err = err
		return m, nil
	}

	m.confirmingDelete = false
	m.deletePrimed = false
	m.taskToDelete = nil
	return m, m.loadData
}

//------------------------------------------export | import-----------------------------------------------------------------------------------------//

func (m *ListModel) handleTransferKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.transfer == nil {
		return m, nil
	}

	state := m.transfer

	// Only an explicit "y" applies an import; Enter is ignored here so a
	// double Enter on the path prompt cannot confirm (for example) a replace.
	if state.stage == transferStageConfirm {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "esc", "n", "q":
			m.transfer = nil
			return m, nil
		case "y":
			return m.applyImportFromTransfer()
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.transfer = nil
		return m, nil
	case "enter":
		if state.action == transferActionExport {
			return m.exportFromTransfer()
		}
		return m.planImportFromTransfer()
	case "backspace":
		if state.cursor > 0 {
			state.path = state.path[:state.cursor-1] + state.path[state.cursor:]
			state.cursor--
		}
	case "left":
		if state.cursor > 0 {
			state.cursor--
		}
	case "right":
		if state.cursor < len(state.path) {
			state.cursor++
		}
	case "home":
		state.cursor = 0
	case "end":
		state.cursor = len(state.path)
	case "alt+c":
		if state.action == transferActionExport {
			state.includeCompleted = !state.includeCompleted
		}
	case "alt+m":
		if state.action == transferActionImport {
			if state.importMode == "merge" {
				state.importMode = "replace"
			} else {
				state.importMode = "merge"
			}
		}
	case "alt+b":
		if state.action == transferActionImport {
			state.backup = !state.backup
		}
	case "alt+o":
		if state.action == transferActionImport {
			state.skipExisting = !state.skipExisting
		}
	case "alt+s":
		if state.action == transferActionImport {
			state.strict = !state.strict
		}
	default:
		if len(msg.String()) == 1 {
			state.path = state.path[:state.cursor] + msg.String() + state.path[state.cursor:]
			state.cursor++
		}
	}

	state.operationError = nil
	return m, nil
}

func (m *ListModel) exportFromTransfer() (tea.Model, tea.Cmd) {
	if m.transfer == nil {
		return m, nil
	}

	path := strings.TrimSpace(m.transfer.path)
	if path == "" {
		m.transfer.operationError = fmt.Errorf("file path is required")
		return m, nil
	}

	filter := ExportFilter{IncludeCompleted: m.transfer.includeCompleted}
	svc := &TaskServiceAdapter{storage: m.storage}
	ctx := context.Background()
	plan, err := PlanExport(ctx, svc, filter)
	if err != nil {
		m.transfer.operationError = err
		return m, nil
	}
	if err := ExportToFile(ctx, svc, filter, path, true); err != nil {
		m.transfer.operationError = err
		return m, nil
	}

	m.statusMessage = fmt.Sprintf("✓ Exported %d tasks to %s", plan.Total, path)
	m.transfer = nil
	return m, nil
}

func (m *ListModel) planImportFromTransfer() (tea.Model, tea.Cmd) {
	if m.transfer == nil {
		return m, nil
	}

	path := strings.TrimSpace(m.transfer.path)
	if path == "" {
		m.transfer.operationError = fmt.Errorf("file path is required")
		return m, nil
	}

	data, err := readImportSource(path, nil)
	if err != nil {
		m.transfer.operationError = err
		return m, nil
	}
	plan, err := planImportData(context.Background(), &TaskServiceAdapter{storage: m.storage}, data, ImportConfig{
		Mode:         m.transfer.importMode,
		SkipExisting: m.transfer.skipExisting,
		OnConflict:   "overwrite",
		IDStrategy:   "preserve",
		Strict:       m.transfer.strict,
		Backup:       m.transfer.backup,
	})
	if err != nil {
		m.transfer.operationError = err
		return m, nil
	}

	m.transfer.path = path
	m.transfer.cursor = len(path)
	m.transfer.plan = &plan
	m.transfer.data = data
	m.transfer.stage = transferStageConfirm
	m.transfer.operationError = nil
	return m, nil
}

func (m *ListModel) applyImportFromTransfer() (tea.Model, tea.Cmd) {
	if m.transfer == nil {
		return m, nil
	}

	// Apply exactly the bytes that were previewed, even if the file changed
	// since; read the file only if no preview data is held.
	data := m.transfer.data
	if data == nil {
		var err error
		if data, err = readImportSource(m.transfer.path, nil); err != nil {
			m.transfer.operationError = err
			return m, nil
		}
	}
	res, err := applyImportData(context.Background(), &TaskServiceAdapter{storage: m.storage}, data, ImportConfig{
		Mode:         m.transfer.importMode,
		SkipExisting: m.transfer.skipExisting,
		OnConflict:   "overwrite",
		IDStrategy:   "preserve",
		Strict:       m.transfer.strict,
		Backup:       m.transfer.backup,
	})
	if err != nil {
		m.transfer.operationError = err
		return m, nil
	}

	status := fmt.Sprintf("✓ Import complete: created=%d updated=%d unchanged=%d skipped=%d conflicted=%d", res.Created, res.Updated, res.Unchanged, res.Skipped, res.Conflicted)
	if len(res.SkippedIDs) > 0 {
		status += fmt.Sprintf(" • skipped IDs=%s", formatTaskIDs(res.SkippedIDs))
	}
	if res.BackupPath != "" {
		status += fmt.Sprintf(" • backup=%s", res.BackupPath)
	}

	m.statusMessage = status
	m.transfer = nil
	m.loading = true
	m.err = nil
	return m, m.loadData
}

func (m *ListModel) renderTransferOverlay(baseView string) string {
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#8B5CF6")).
		Padding(1, 2).
		Background(lipgloss.Color("#1A1A2E")).
		Foreground(lipgloss.Color("#FFFFFF"))

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f7cf79")).
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444"))

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#8B5CF6")).
		Padding(0, 1).
		Width(60)

	var dialog strings.Builder

	if m.transfer.action == transferActionExport {
		dialog.WriteString(titleStyle.Render("Export Tasks"))
		dialog.WriteString("\n\n")
		dialog.WriteString("Path:\n")
		dialog.WriteString(inputStyle.Render(m.addTransferCursor(m.transfer.path)))
		dialog.WriteString("\n\n")
		_, err := fmt.Fprintf(&dialog, "Include completed: %s (press alt+c to toggle)\n\n", yesNoLabel(m.transfer.includeCompleted))
		if err != nil {
			return ""
		}
		dialog.WriteString(helpStyle.Render("[enter] Export  [esc] Cancel"))
	} else {
		dialog.WriteString(titleStyle.Render("Import Tasks"))
		dialog.WriteString("\n\n")
		if m.transfer.stage == transferStageConfirm && m.transfer.plan != nil {
			_, err := fmt.Fprintf(&dialog, "File: %s\n", m.transfer.path)
			if err != nil {
				return ""
			}
			_, err = fmt.Fprintf(&dialog, "Mode: %s", m.transfer.importMode)
			if err != nil {
				return ""
			}
			if m.transfer.importMode == "replace" {
				dialog.WriteString(" (will replace all local tasks)")
			}
			dialog.WriteString("\n")
			_, err = fmt.Fprintf(&dialog, "Skip existing ID collisions: %s\n", yesNoLabel(m.transfer.skipExisting))
			if err != nil {
				return ""
			}
			_, err = fmt.Fprintf(&dialog, "Backup before import: %s\n", yesNoLabel(m.transfer.backup))
			if err != nil {
				return ""
			}
			_, err = fmt.Fprintf(&dialog, "Strict parsing: %s\n\n", yesNoLabel(m.transfer.strict))
			if err != nil {
				return ""
			}
			dialog.WriteString("Plan:\n")
			_, err = fmt.Fprintf(&dialog, "  Incoming: %d\n", m.transfer.plan.Incoming)
			if err != nil {
				return ""
			}

			_, err = fmt.Fprintf(&dialog, "  Current: %d\n", m.transfer.plan.Current)
			if err != nil {
				return ""
			}

			_, err = fmt.Fprintf(&dialog, "  Create: %d\n", m.transfer.plan.ToCreate)
			if err != nil {
				return ""
			}

			_, err = fmt.Fprintf(&dialog, "  Update: %d\n", m.transfer.plan.ToUpdate)
			if err != nil {
				return ""
			}

			_, err = fmt.Fprintf(&dialog, "  Unchanged: %d\n", m.transfer.plan.Unchanged)
			if err != nil {
				return ""
			}

			_, err = fmt.Fprintf(&dialog, "  Conflicts: %d\n", m.transfer.plan.Conflicts)
			if err != nil {
				return ""
			}
			if len(m.transfer.plan.ConflictIDs) > 0 {
				_, err = fmt.Fprintf(&dialog, "  Conflict IDs: %s\n", formatTaskIDs(m.transfer.plan.ConflictIDs))
				if err != nil {
					return ""
				}
			}
			dialog.WriteString("\n")
			dialog.WriteString(helpStyle.Render("[y] Import  [n/esc] Cancel"))
		} else {
			dialog.WriteString("Path:\n")
			dialog.WriteString(inputStyle.Render(m.addTransferCursor(m.transfer.path)))
			dialog.WriteString("\n\n")
			_, err := fmt.Fprintf(&dialog, "Mode: %s (press alt+m to toggle)\n", m.transfer.importMode)
			if err != nil {
				return ""
			}
			_, err = fmt.Fprintf(&dialog, "Skip existing ID collisions: %s (press alt+o to toggle)\n", yesNoLabel(m.transfer.skipExisting))
			if err != nil {
				return ""
			}
			_, err = fmt.Fprintf(&dialog, "Backup before import: %s (press alt+b to toggle)\n", yesNoLabel(m.transfer.backup))
			if err != nil {
				return ""
			}
			_, err = fmt.Fprintf(&dialog, "Strict parsing: %s (press alt+s to toggle)\n\n", yesNoLabel(m.transfer.strict))
			if err != nil {
				return ""
			}
			dialog.WriteString(helpStyle.Render("[enter] Preview Import  [esc] Cancel"))
		}
	}

	if m.transfer.operationError != nil {
		dialog.WriteString("\n\n")
		dialog.WriteString(errorStyle.Render("Error: " + sanitizeForTerminal(m.transfer.operationError.Error(), true)))
	}

	dialogContent := dialogStyle.Render(dialog.String())

	viewW := m.viewportWidth
	viewH := m.viewportHeight
	if viewW <= 0 {
		viewW = 80
	}
	if viewH <= 0 {
		viewH = 24
	}

	base := lipgloss.NewStyle().
		Width(viewW).
		Height(viewH).
		Foreground(lipgloss.Color("#6B7280")).
		Render(baseView)

	modalLayer := lipgloss.Place(
		viewW,
		viewH,
		lipgloss.Center,
		lipgloss.Center,
		dialogContent,
		lipgloss.WithWhitespaceChars(" "),
	)

	return base + "\x1b[H" + modalLayer
}

func (m *ListModel) addTransferCursor(text string) string {
	if m.transfer == nil {
		return text
	}
	if m.transfer.cursor >= len(text) {
		return text + "█"
	}
	return text[:m.transfer.cursor] + "█" + text[m.transfer.cursor:]
}

func yesNoLabel(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
