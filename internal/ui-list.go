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
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const pageSize = 10

// Init initializes the list model
func (m *ListModel) Init() tea.Cmd {
	return m.loadData
}

func (m *ListModel) loadData() tea.Msg {
	tasks, err := m.storage.ListTasks()
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
		m.tasks = msg.tasks
		m.topUpcoming = GetTopUpcomingTasks(m.tasks, 10)
		m.tasksNoDeadline = GetTasksWithoutDeadline(m.tasks)
		return m, nil

	case ErrMsg:
		m.err = msg.error
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		if m.transfer != nil {
			return m.handleTransferKey(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit

		case "up", "shift+tab":
			if m.cursor > 0 {
				m.cursor--
				m.EnsureCursorVisible()
			}

		case "down", "tab":
			if m.cursor < len(m.GetVisibleTasks())-1 {
				m.cursor++
				m.EnsureCursorVisible()
			}

		case "e":
			m.expanded[m.cursor] = !m.expanded[m.cursor]

		case "c":
			if err := m.ToggleComplete(); err != nil {
				m.err = err
			}
			return m, m.loadData

		case "d":
			if !m.confirmingDelete {
				task := m.GetCurrentTask()
				if task != nil {
					m.confirmingDelete = true
					m.taskToDelete = task
				}
			}
			return m, nil

		case "n":
			if m.confirmingDelete {
				m.confirmingDelete = false
				m.taskToDelete = nil
				return m, nil
			}
			return NewFormModel(m.storage), nil

		case "y":
			if m.confirmingDelete && m.taskToDelete != nil {
				if err := m.storage.DeleteTask(m.taskToDelete.ID); err != nil {
					m.err = err
				}
				m.confirmingDelete = false
				m.taskToDelete = nil
				return m, m.loadData
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
				m.cursor = 0
			}

		case "pgdown", "f":
			visibleTasks := m.GetVisibleTasks()
			if (m.currentPage+1)*pageSize < len(visibleTasks) {
				m.currentPage++
				m.cursor = 0
			}

		case "x":
			m.err = nil
			m.statusMessage = ""
			m.transfer = &transferState{
				action: transferActionExport,
				stage:  transferStageInput,
				path:   fmt.Sprintf("munus-export-%s.json", time.Now().Format("20060102")),
			}
			m.transfer.cursor = len(m.transfer.path)
			return m, nil

		case "i":
			m.err = nil
			m.statusMessage = ""
			m.transfer = &transferState{
				action:     transferActionImport,
				stage:      transferStageInput,
				importMode: "merge",
				backup:     true,
			}
			return m, nil

		}
	}

	return m, nil
}

// View renders the list
func (m *ListModel) View() string {
	if m.loading {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9333EA")).
			Render("Loading tasks...")
	}

	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Render("Error: " + m.err.Error())
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

	s.WriteString(sectionStyle.Render(" Upcoming Deadlines (Top 10)"))
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
	s.WriteString(helpStyle.Render("\n\nshift+tab/↑ | tab/↓: Navigate • e: Expand • c: Complete • d: Delete • n: New • r: Refresh • ctrl+c: Quit"))
	s.WriteString(helpStyle.Render("\nx: Export to File • i: Import from File"))

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
		dialog.WriteString(warningStyle.Render("⚠  Delete Confirmation"))
		dialog.WriteString("\n\n")
		dialog.WriteString("Are you sure you want to delete this task?\n\n")
		dialog.WriteString(modalTitleStyle.Render("Title: "))
		dialog.WriteString(m.taskToDelete.Title)
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
	if task.Completed {
		checkbox = "[✔]"
	}

	deadlineInfo := ""
	if task.Deadline != nil && !task.Completed {
		days := task.DaysUntilDeadline()
		if days < 0 {
			deadlineInfo = overdueStyle.Render(fmt.Sprintf(" (Overdue by %d days)", -days))
		} else if days == 0 {
			deadlineInfo = overdueStyle.Render(" (Due today!)")
		} else if days <= 3 {
			deadlineInfo = upcomingStyle.Render(fmt.Sprintf(" (%d days left)", days))
		} else {
			deadlineInfo = fmt.Sprintf(" (%s)", task.Deadline.Format("Jan 2, 3:04 PM"))
		}
	}

	line := fmt.Sprintf("%s %s%s", checkbox, task.Title, deadlineInfo)

	if isSelected {
		s.WriteString(selectedStyle.Render(line))
	} else if task.Completed {
		s.WriteString(completedStyle.Render(line))
	} else {
		s.WriteString(normalStyle.Render(line))
	}

	if m.expanded[index] && task.Description != "" {
		s.WriteString("\n")
		s.WriteString(descriptionStyle.Render(task.Description))
		s.WriteString("\n")
	}

	return s.String()
}

func (m *ListModel) GetVisibleTasks() []*ItemModel {
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

	if task.Completed {
		task.MarkIncomplete()
	} else {
		task.MarkComplete()
	}

	return m.storage.UpdateTask(task)
}

//------------------------------------------export | import-----------------------------------------------------------------------------------------//

func (m *ListModel) handleTransferKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.transfer == nil {
		return m, nil
	}

	state := m.transfer

	if state.stage == transferStageConfirm {
		switch msg.String() {
		case "esc", "n":
			m.transfer = nil
			return m, nil
		case "y", "enter":
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
	plan, err := PlanExport(svc, filter)
	if err != nil {
		m.transfer.operationError = err
		return m, nil
	}
	if err := ExportToFile(svc, filter, path, true); err != nil {
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

	plan, err := PlanImport(&TaskServiceAdapter{storage: m.storage}, path, ImportConfig{
		Mode:       m.transfer.importMode,
		OnConflict: "overwrite",
		IDStrategy: "preserve",
		Strict:     m.transfer.strict,
		Backup:     m.transfer.backup,
	})
	if err != nil {
		m.transfer.operationError = err
		return m, nil
	}

	m.transfer.path = path
	m.transfer.cursor = len(path)
	m.transfer.plan = &plan
	m.transfer.stage = transferStageConfirm
	m.transfer.operationError = nil
	return m, nil
}

func (m *ListModel) applyImportFromTransfer() (tea.Model, tea.Cmd) {
	if m.transfer == nil {
		return m, nil
	}

	res, err := ApplyImport(&TaskServiceAdapter{storage: m.storage}, m.transfer.path, ImportConfig{
		Mode:       m.transfer.importMode,
		OnConflict: "overwrite",
		IDStrategy: "preserve",
		Strict:     m.transfer.strict,
		Backup:     m.transfer.backup,
	})
	if err != nil {
		m.transfer.operationError = err
		return m, nil
	}

	status := fmt.Sprintf("✓ Import complete: created=%d updated=%d unchanged=%d skipped=%d", res.Created, res.Updated, res.Unchanged, res.Skipped)
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
		dialog.WriteString(fmt.Sprintf("Include completed: %s (press alt+c to toggle)\n\n", yesNoLabel(m.transfer.includeCompleted)))
		dialog.WriteString(helpStyle.Render("[enter] Export  [esc] Cancel"))
	} else {
		dialog.WriteString(titleStyle.Render("Import Tasks"))
		dialog.WriteString("\n\n")
		if m.transfer.stage == transferStageConfirm && m.transfer.plan != nil {
			dialog.WriteString(fmt.Sprintf("File: %s\n", m.transfer.path))
			dialog.WriteString(fmt.Sprintf("Mode: %s", m.transfer.importMode))
			if m.transfer.importMode == "replace" {
				dialog.WriteString(" (will replace all local tasks)")
			}
			dialog.WriteString("\n")
			dialog.WriteString(fmt.Sprintf("Backup before import: %s\n", yesNoLabel(m.transfer.backup)))
			dialog.WriteString(fmt.Sprintf("Strict parsing: %s\n\n", yesNoLabel(m.transfer.strict)))
			dialog.WriteString("Plan:\n")
			dialog.WriteString(fmt.Sprintf("  Incoming: %d\n", m.transfer.plan.Incoming))
			dialog.WriteString(fmt.Sprintf("  Current: %d\n", m.transfer.plan.Current))
			dialog.WriteString(fmt.Sprintf("  Create: %d\n", m.transfer.plan.ToCreate))
			dialog.WriteString(fmt.Sprintf("  Update: %d\n", m.transfer.plan.ToUpdate))
			dialog.WriteString(fmt.Sprintf("  Unchanged: %d\n\n", m.transfer.plan.Unchanged))
			dialog.WriteString(helpStyle.Render("[y] Import  [n] Cancel"))
		} else {
			dialog.WriteString("Path:\n")
			dialog.WriteString(inputStyle.Render(m.addTransferCursor(m.transfer.path)))
			dialog.WriteString("\n\n")
			dialog.WriteString(fmt.Sprintf("Mode: %s (press alt+m to toggle)\n", m.transfer.importMode))
			dialog.WriteString(fmt.Sprintf("Backup before import: %s (press alt+b to toggle)\n", yesNoLabel(m.transfer.backup)))
			dialog.WriteString(fmt.Sprintf("Strict parsing: %s (press alt+s to toggle)\n\n", yesNoLabel(m.transfer.strict)))
			dialog.WriteString(helpStyle.Render("[enter] Preview Import  [esc] Cancel"))
		}
	}

	if m.transfer.operationError != nil {
		dialog.WriteString("\n\n")
		dialog.WriteString(errorStyle.Render("Error: " + m.transfer.operationError.Error()))
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
