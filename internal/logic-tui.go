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
	"sort"
	"time"
)

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

// GetTopUpcomingTasks returns the top N tasks with the closest deadline
func GetTopUpcomingTasks(tasks []*ItemModel, limit int) []*ItemModel {
	var upcomingTasks []*ItemModel
	for _, todo := range tasks {
		if !todo.Completed && todo.Deadline != nil {
			upcomingTasks = append(upcomingTasks, todo)
		}
	}

	sort.Slice(upcomingTasks, func(i, j int) bool {
		if upcomingTasks[i].Deadline == nil || upcomingTasks[j].Deadline == nil {
			return false
		}
		return upcomingTasks[i].Deadline.Before(*upcomingTasks[j].Deadline)
	})

	if len(upcomingTasks) > limit {
		return upcomingTasks[:limit]
	}
	return upcomingTasks
}

// GetTasksWithoutDeadline returns tasks without deadline
func GetTasksWithoutDeadline(tasks []*ItemModel) []*ItemModel {
	var noDeadlineTasks []*ItemModel
	for _, task := range tasks {
		if !task.Completed && task.Deadline == nil {
			noDeadlineTasks = append(noDeadlineTasks, task)
		}
	}
	return noDeadlineTasks
}
