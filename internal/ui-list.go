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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const pageSize = 10

// NewListModel creates a new list model
func NewListModel(storage Storage) *ListModel {
	m := &ListModel{
		storage:          storage,
		expanded:         make(map[int]bool),
		loading:          true,
		confirmingDelete: false,
		taskToDelete:     nil,
	}
	return m
}

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
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit

		case "up", "ctrl+w":
			if m.cursor > 0 {
				m.cursor--
				m.EnsureCursorVisible()
			}

		case "down", "ctrl+s":
			if m.cursor < len(m.GetVisibleTasks())-1 {
				m.cursor++
				m.EnsureCursorVisible()
			}

		case "Space":
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
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8B5CF6")).
		Bold(true).
		MarginTop(1).
		MarginBottom(1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f8c623")).
		Background(lipgloss.Color("#8efff6")).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Padding(0, 1)

	completeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f8c623")).
		Strikethrough(true).
		Padding(0, 1)

	overdueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		Bold(true)

	upcomingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F59E0B"))

	descriptionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f8c623")).
		PaddingLeft(3)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f8c623")).
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
	s.WriteString(helpStyle.Render("Commands:\n"))
	s.WriteString(helpStyle.Render("ctrl+w/↑/ctrl+s/↓: Navigate • Space: Expand • c: Complete • d: Delete • n: New • r: Refresh • ctrl+c: Quit"))

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
