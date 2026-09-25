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
	"slices"
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

// MarkComplete marks the item as complete (status done).
func (t *ItemModel) MarkComplete() {
	t.setStatus(StatusDone, time.Now())
}

// MarkIncomplete marks the item as not complete (status todo).
func (t *ItemModel) MarkIncomplete() {
	t.setStatus(StatusTodo, time.Now())
}

// setStatus changes the status and keeps Completed/CompletedAt consistent.
func (t *ItemModel) setStatus(status TaskStatus, now time.Time) {
	if t.Status == status && t.Completed == (status == StatusDone) {
		return
	}
	t.Status = status
	t.Completed = status == StatusDone
	if t.Completed {
		t.CompletedAt = &now
	} else {
		t.CompletedAt = nil
	}
	t.UpdatedAt = now
}

// itemStatus returns t's status, deriving it from Completed when unset (for
// tasks that have not been saved yet).
func itemStatus(t *ItemModel) TaskStatus {
	if t.Status != "" {
		return t.Status
	}
	if t.Completed {
		return StatusDone
	}
	return StatusTodo
}

// nextStatus returns the status after s in the todo → doing → done cycle.
func nextStatus(s TaskStatus) TaskStatus {
	switch s {
	case StatusTodo, "":
		return StatusDoing
	case StatusDoing:
		return StatusDone
	default:
		return StatusTodo
	}
}

// isOverdueAt reports whether t is unfinished and past its deadline at now.
func isOverdueAt(t *ItemModel, now time.Time) bool {
	return t.Deadline != nil && !t.Completed && t.Deadline.Before(now)
}

// calendarDaysBetween returns the number of calendar days from now to
// deadline in now's location (0 = same day, 1 = tomorrow, -1 = yesterday).
// Whole days are counted on the calendar, so DST changes do not skew it.
func calendarDaysBetween(now, deadline time.Time) int {
	d := deadline.In(now.Location())
	dy, dm, dd := d.Date()
	ny, nm, nd := now.Date()
	from := time.Date(ny, nm, nd, 0, 0, 0, 0, time.UTC)
	to := time.Date(dy, dm, dd, 0, 0, 0, 0, time.UTC)
	return int(to.Sub(from).Hours() / 24)
}

// deadlineLabel describes deadline relative to now for the task list. urgent
// is true for overdue tasks and tasks due today.
func deadlineLabel(deadline, now time.Time) (label string, urgent bool) {
	days := calendarDaysBetween(now, deadline)
	switch {
	case deadline.Before(now) && days >= 0:
		return "Overdue", true
	case deadline.Before(now):
		return fmt.Sprintf("Overdue by %d %s", -days, plural(-days, "day", "days")), true
	case days == 0:
		return "Due today!", true
	case days == 1:
		return "Due tomorrow", false
	case days <= 3:
		return fmt.Sprintf("%d days left", days), false
	default:
		return deadline.In(now.Location()).Format("Jan 2, 3:04 PM"), false
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// taskFilter selects tasks for `munus list` and the TUI list. Set fields are
// combined with AND.
type taskFilter struct {
	pending   bool       // not done
	completed bool       // done
	overdue   bool       // not done and past its deadline
	status    TaskStatus // "" matches any status
	tags      []string   // every tag must be present
}

func (f taskFilter) matches(t *ItemModel, now time.Time) bool {
	if f.pending && t.Completed {
		return false
	}
	if f.completed && !t.Completed {
		return false
	}
	if f.overdue && !isOverdueAt(t, now) {
		return false
	}
	if f.status != "" && itemStatus(t) != f.status {
		return false
	}
	for _, tag := range f.tags {
		if !slices.Contains(t.Tags, tag) {
			return false
		}
	}
	return true
}

// filterItems returns the tasks matching f, keeping their order.
func filterItems(tasks []*ItemModel, f taskFilter, now time.Time) []*ItemModel {
	out := make([]*ItemModel, 0, len(tasks))
	for _, t := range tasks {
		if f.matches(t, now) {
			out = append(out, t)
		}
	}
	return out
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

// GetTasksWithoutDeadline returns tasks without a deadline
func GetTasksWithoutDeadline(tasks []*ItemModel) []*ItemModel {
	var noDeadlineTasks []*ItemModel
	for _, task := range tasks {
		if !task.Completed && task.Deadline == nil {
			noDeadlineTasks = append(noDeadlineTasks, task)
		}
	}
	return noDeadlineTasks
}
