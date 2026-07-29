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

import "time"

// ItemModel Represents an item
type ItemModel struct {
	ID          uint       `gorm:"primaryKey"`
	Title       string     `gorm:"size:255;not null"`
	Description string     `gorm:"type:text"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	Completed   bool       `gorm:"default:false;not null"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ListModel represents the list view model
type ListModel struct {
	storage          Storage
	tasks            []*ItemModel
	topUpcoming      []*ItemModel
	tasksNoDeadline  []*ItemModel
	cursor           int
	expanded         map[int]bool
	currentPage      int
	showHelp         bool
	err              error
	loading          bool
	confirmingDelete bool
	taskToDelete     *ItemModel
}

// FormModel represents the form input model
type FormModel struct {
	storage      Storage
	fields       []string
	currentField formField
	cursor       int
	done         bool
	err          error
	submitted    bool
}

// DataLoadedMsg is emitted when tasks are loaded from storage.
type DataLoadedMsg struct {
	tasks []*ItemModel
}

type ErrMsg struct{ error }

// IsOverdue checks if the item is overdue
func (t *ItemModel) IsOverdue() bool {
	if t.Deadline == nil || t.Completed {
		return false
	}
	return t.Deadline.Before(time.Now())
}

// DaysUntilDeadline returns the number of days until the deadline
func (t *ItemModel) DaysUntilDeadline() int {
	if t.Deadline == nil {
		return -1
	}
	duration := time.Until(*t.Deadline)
	return int(duration.Hours() / 24)
}

// MarkComplete marks the item as complete
func (t *ItemModel) MarkComplete() {
	t.Completed = true
	now := time.Now()
	t.CompletedAt = &now
	t.UpdatedAt = now
}

// MarkIncomplete marks the item as incomplete
func (t *ItemModel) MarkIncomplete() {
	t.Completed = false
	t.CompletedAt = nil
	t.UpdatedAt = time.Now()
}
