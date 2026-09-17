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

type formField int

const (
	titleField formField = iota
	descriptionField
	deadlineField
)

// Character limits
const (
	MaxTitleLength       = 100
	MaxDescriptionLength = 500
	MaxDeadlineLength    = 365
)

// Init initializes the form model
func (m *FormModel) Init() tea.Cmd {
	return nil
}

func (m *FormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if model, cmd, handled := m.handleVimKey(msg); handled {
			return model, cmd
		}

		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit

		case "ctrl+l":
			lm := NewListModelWithOptions(m.storage, tuiOptions{vimEnabled: m.vimEnabled})
			return lm, lm.Init()

		case "tab", "down":
			if m.currentField < deadlineField {
				m.currentField++
				m.cursor = len(m.fields[m.currentField])
			}

		case "shift+tab", "up":
			if m.currentField > titleField {
				m.currentField--
				m.cursor = len(m.fields[m.currentField])
			}

		case "enter":
			if m.currentField < deadlineField {
				m.currentField++
				m.cursor = 0
			} else {
				if err := m.submitForm(); err != nil {
					m.err = err
				} else {
					m.submitted = true
					if m.vimEnabled {
						lm := NewListModelWithOptions(m.storage, tuiOptions{vimEnabled: m.vimEnabled})
						lm.statusMessage = "🗹 Task created successfully!"
						return lm, lm.Init()
					}
					fm := NewFormModel(m.storage)
					return fm, fm.Init()
				}
			}

		case "backspace":
			if m.cursor > 0 {
				field := m.fields[m.currentField]
				m.fields[m.currentField] = field[:m.cursor-1] + field[m.cursor:]
				m.cursor--
			}

		case "left":
			if m.cursor > 0 {
				m.cursor--
			}

		case "right":
			if m.cursor < len(m.fields[m.currentField]) {
				m.cursor++
			}

		case "home":
			m.cursor = 0

		case "end":
			m.cursor = len(m.fields[m.currentField])

		default:
			if len(msg.String()) == 1 {
				var canAddChar bool
				switch m.currentField {
				case titleField:
					canAddChar = len(m.fields[titleField]) < MaxTitleLength
				case descriptionField:
					canAddChar = len(m.fields[descriptionField]) < MaxDescriptionLength
				case deadlineField:
					canAddChar = len(m.fields[deadlineField]) < MaxDeadlineLength
				default:
					canAddChar = false
				}
				if canAddChar {
					field := m.fields[m.currentField]
					m.fields[m.currentField] = field[:m.cursor] + msg.String() + field[m.cursor:]
					m.cursor++
				}
			}
		}
	}
	return m, nil
}

func (m *FormModel) handleVimKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	if !m.vimEnabled {
		return m, nil, false
	}

	if m.formMode == formModeInsert {
		if msg.Type == tea.KeyEsc {
			m.formMode = formModeNormal
			return m, nil, true
		}
		return m, nil, false
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		m.done = true
		return m, tea.Quit, true
	case tea.KeyCtrlL:
		return m.returnToList()
	case tea.KeyEsc:
		return m.returnToList()
	case tea.KeyTab, tea.KeyDown:
		if m.currentField < deadlineField {
			m.currentField++
			m.cursor = len(m.fields[m.currentField])
		}
		return m, nil, true
	case tea.KeyShiftTab, tea.KeyUp:
		if m.currentField > titleField {
			m.currentField--
			m.cursor = len(m.fields[m.currentField])
		}
		return m, nil, true
	case tea.KeyEnter:
		return m.advanceOrSubmit()
	case tea.KeyRunes:
		switch msg.String() {
		case "i", "a":
			m.formMode = formModeInsert
			return m, nil, true
		case "o":
			if m.currentField < deadlineField {
				m.currentField++
				m.cursor = len(m.fields[m.currentField])
			}
			m.formMode = formModeInsert
			return m, nil, true
		case "j":
			if m.currentField < deadlineField {
				m.currentField++
				m.cursor = len(m.fields[m.currentField])
			}
			return m, nil, true
		case "k":
			if m.currentField > titleField {
				m.currentField--
				m.cursor = len(m.fields[m.currentField])
			}
			return m, nil, true
		case "h":
			return m.returnToList()
		case "l":
			return m.advanceOrSubmit()
		default:
			return m, nil, true
		}
	default:
		return m, nil, true
	}
}

// View renders the form
func (m *FormModel) View() string {
	if m.submitted {
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4CAF50")).
			Bold(true)
		return successStyle.Render("🗹Task created successfully!")
	}

	if m.done {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f7cf79")).
		Bold(true).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9333EA")).
		Width(15)

	activityStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#8B5CF6")).
		Padding(0, 1).
		Width(60)

	inactiveStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#8B5CF6")).
		Padding(0, 1).
		Width(60)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444")).
		MarginTop(1)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f7cf79")).
		MarginTop(2)

	deadlineHelpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f7cf79")).
		PaddingLeft(2)

	var s strings.Builder
	s.WriteString(titleStyle.Render("Create New Task"))
	s.WriteString("\n\n")

	if m.vimEnabled {
		modeLabel := "-- INSERT --"
		if m.formMode == formModeNormal {
			modeLabel = "-- NORMAL --"
		}
		s.WriteString(helpStyle.Render(modeLabel))
		s.WriteString("\n\n")
	}

	titleLabel := fmt.Sprintf("Title * (%d/%d)", len(m.fields[titleField]), MaxTitleLength)
	s.WriteString(labelStyle.Render(titleLabel))
	s.WriteString("\n")
	titleContent := m.fields[titleField]
	if m.currentField == titleField {
		titleContent = m.addCursor(titleContent)
		s.WriteString(activityStyle.Render(titleContent))
	} else {
		if titleContent == "" {
			titleContent = "Enter a title (required)"
		}
		s.WriteString(inactiveStyle.Render(titleContent))
	}
	s.WriteString("\n\n")

	descLabel := fmt.Sprintf("Description * (%d/%d)", len(m.fields[descriptionField]), MaxDescriptionLength)
	s.WriteString(labelStyle.Render(descLabel))
	s.WriteString("\n")
	descContent := m.fields[descriptionField]
	if m.currentField == descriptionField {
		descContent = m.addCursor(descContent)
		s.WriteString(activityStyle.Render(descContent))
	} else {
		if descContent == "" {
			descContent = "Enter a description (required)"
		}
		s.WriteString(inactiveStyle.Render(descContent))
	}
	s.WriteString("\n\n")

	s.WriteString(labelStyle.Render("Deadline"))
	s.WriteString("\n")
	deadlineContent := m.fields[deadlineField]
	if m.currentField == deadlineField {
		deadlineContent = m.addCursor(deadlineContent)
		s.WriteString(activityStyle.Render(deadlineContent))
		s.WriteString("\n")
		s.WriteString(deadlineHelpStyle.
			Render("Examples: 2026-07-26 13:30, 2d, 1h 30m, 1w 2d"))
	} else {
		if deadlineContent == "" {
			deadlineContent = "e.g., 2026-07-26 13:30 or 2d 3h (optional)"
		}
		s.WriteString(inactiveStyle.Render(deadlineContent))
	}

	if m.err != nil {
		s.WriteString("\n")
		s.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	}

	s.WriteString("\n")
	if m.vimEnabled {
		if m.formMode == formModeInsert {
			s.WriteString(helpStyle.Render("Vim insert • Esc: normal • Tab/↑/↓: field navigation • Enter: next/submit • ctrl+l: List • ctrl+c: Quit"))
		} else {
			s.WriteString(helpStyle.Render("Vim normal • j/k: fields • h/esc: list • l/Enter: next/submit • i/a/o: insert • ctrl+c: Quit"))
		}
	} else {
		s.WriteString(helpStyle.Render("shift+tab/↑ | tab/↓: Navigation • Enter: Submit • ctrl+l: List • ctrl+c: Quit"))
		s.WriteString(helpStyle.Render("\nTyping is always literal in form fields; list-only Vim navigation is disabled while editing."))
	}

	return s.String()
}

func (m *FormModel) addCursor(text string) string {
	if m.cursor >= len(text) {
		return text + "█"
	}
	return text[:m.cursor] + "█" + text[m.cursor:]
}

func (m *FormModel) returnToList() (tea.Model, tea.Cmd, bool) {
	lm := NewListModelWithOptions(m.storage, tuiOptions{vimEnabled: m.vimEnabled})
	return lm, lm.Init(), true
}

func (m *FormModel) advanceOrSubmit() (tea.Model, tea.Cmd, bool) {
	if m.currentField < deadlineField {
		m.currentField++
		m.cursor = len(m.fields[m.currentField])
		return m, nil, true
	}

	if err := m.submitForm(); err != nil {
		m.err = err
		return m, nil, true
	}

	m.submitted = true
	lm := NewListModelWithOptions(m.storage, tuiOptions{vimEnabled: m.vimEnabled})
	lm.statusMessage = "🗹 Task created successfully!"
	return lm, lm.Init(), true
}

func (m *FormModel) submitForm() error {
	if strings.TrimSpace(m.fields[titleField]) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(m.fields[descriptionField]) == "" {
		return fmt.Errorf("description is required")
	}

	if len(m.fields[titleField]) > MaxTitleLength {
		return fmt.Errorf("title exceeds maximum length of %d characters", MaxTitleLength)
	}
	if len(m.fields[descriptionField]) > MaxDescriptionLength {
		return fmt.Errorf("description exceeds maximum length of %d characters", MaxDescriptionLength)
	}

	var deadline *time.Time
	if strings.TrimSpace(m.fields[deadlineField]) != "" {
		parsed, err := ParseDeadline(strings.TrimSpace(m.fields[deadlineField]))
		if err != nil {
			return err
		}
		deadline = parsed
	}

	now := time.Now()
	task := ItemModel{
		Title:       strings.TrimSpace(m.fields[titleField]),
		Description: strings.TrimSpace(m.fields[descriptionField]),
		Deadline:    deadline,
		CreatedAt:   now,
		UpdatedAt:   now,
		Completed:   false,
	}

	return m.storage.CreateTask(&task)
}
