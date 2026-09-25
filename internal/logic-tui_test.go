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
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // fixed zone data so DST tests do not depend on the machine
)

// TestIsOverdue tests the IsOverdue method
func TestIsOverdue(t *testing.T) {
	tests := []struct {
		name     string
		item     *ItemModel
		expected bool
	}{
		{
			name: "overdue task with past deadline",
			item: &ItemModel{
				Deadline:  new(time.Now().Add(-24 * time.Hour)),
				Completed: false,
			},
			expected: true,
		},
		{
			name: "not overdue task with future deadline",
			item: &ItemModel{
				Deadline:  new(time.Now().Add(24 * time.Hour)),
				Completed: false,
			},
			expected: false,
		},
		{
			name: "completed task is not overdue",
			item: &ItemModel{
				Deadline:  new(time.Now().Add(-24 * time.Hour)),
				Completed: true,
			},
			expected: false,
		},
		{
			name: "task with nil deadline is not overdue",
			item: &ItemModel{
				Deadline:  nil,
				Completed: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.item.IsOverdue()
			if result != tt.expected {
				t.Errorf("IsOverdue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestDaysUntilDeadline tests the DaysUntilDeadline method
func TestDaysUntilDeadline(t *testing.T) {
	tests := []struct {
		name     string
		item     *ItemModel
		checkFn  func(int) bool
		errorMsg string
	}{
		{
			name: "deadline in 5 days",
			item: &ItemModel{
				Deadline: new(time.Now().Add(5 * 24 * time.Hour)),
			},
			checkFn: func(days int) bool {
				return days >= 4 && days <= 5 // Allow for timing variance
			},
			errorMsg: "expected days to be 4 or 5",
		},
		{
			name: "deadline in 1 day",
			item: &ItemModel{
				Deadline: new(time.Now().Add(24 * time.Hour)),
			},
			checkFn: func(days int) bool {
				return days >= 0 && days <= 1
			},
			errorMsg: "expected days to be 0 or 1",
		},
		{
			name: "deadline in the past",
			item: &ItemModel{
				Deadline: new(time.Now().Add(-24 * time.Hour)),
			},
			checkFn: func(days int) bool {
				return days < 0
			},
			errorMsg: "expected negative days",
		},
		{
			name: "nil deadline returns -1",
			item: &ItemModel{
				Deadline: nil,
			},
			checkFn: func(days int) bool {
				return days == -1
			},
			errorMsg: "expected -1 for nil deadline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.item.DaysUntilDeadline()
			if !tt.checkFn(result) {
				t.Errorf("%s, got %d", tt.errorMsg, result)
			}
		})
	}
}

// TestMarkComplete tests the MarkComplete method
func TestMarkComplete(t *testing.T) {
	item := &ItemModel{
		Completed: false,
	}

	before := time.Now()
	item.MarkComplete()
	after := time.Now()

	if !item.Completed {
		t.Errorf("MarkComplete() failed to set Completed to true")
	}

	if item.CompletedAt == nil {
		t.Errorf("MarkComplete() did not set CompletedAt")
	}

	if item.CompletedAt.Before(before) || item.CompletedAt.After(after) {
		t.Errorf("MarkComplete() set CompletedAt to %v, expected between %v and %v", item.CompletedAt, before, after)
	}

	if item.UpdatedAt.Before(before) || item.UpdatedAt.After(after) {
		t.Errorf("MarkComplete() set UpdatedAt to %v, expected between %v and %v", item.UpdatedAt, before, after)
	}
}

// TestMarkIncomplete tests the MarkIncomplete method
func TestMarkIncomplete(t *testing.T) {
	now := time.Now()
	item := &ItemModel{
		Completed:   true,
		CompletedAt: &now,
	}

	before := time.Now()
	item.MarkIncomplete()
	after := time.Now()

	if item.Completed {
		t.Errorf("MarkIncomplete() failed to set Completed to false")
	}

	if item.CompletedAt != nil {
		t.Errorf("MarkIncomplete() did not clear CompletedAt")
	}

	if item.UpdatedAt.Before(before) || item.UpdatedAt.After(after) {
		t.Errorf("MarkIncomplete() set UpdatedAt to %v, expected between %v and %v", item.UpdatedAt, before, after)
	}
}

// TestGetTopUpcomingTasks tests the GetTopUpcomingTasks function
func TestGetTopUpcomingTasks(t *testing.T) {
	now := time.Now()
	tests := []struct {
		title    string
		tasks    []*ItemModel
		limit    int
		expected int
	}{
		{
			title:    "empty task list",
			tasks:    []*ItemModel{},
			limit:    5,
			expected: 0,
		},
		{
			title: "no incomplete tasks with deadline",
			tasks: []*ItemModel{
				{
					Completed: true,
					Deadline:  new(now.Add(24 * time.Hour)),
				},
			},
			limit:    5,
			expected: 0,
		},
		{
			title: "all tasks have no deadline",
			tasks: []*ItemModel{
				{
					Completed: false,
					Deadline:  nil,
				},
				{
					Completed: false,
					Deadline:  nil,
				},
			},
			limit:    5,
			expected: 0,
		},
		{
			title: "return top 3 of 5 upcoming tasks",
			tasks: []*ItemModel{
				{
					Title:     "Task 1",
					Completed: false,
					Deadline:  new(now.Add(96 * time.Hour)), // 4 days
				},
				{
					Title:     "Task 2",
					Completed: false,
					Deadline:  new(now.Add(24 * time.Hour)), // 1 day
				},
				{
					Title:     "Task 3",
					Completed: false,
					Deadline:  new(now.Add(72 * time.Hour)), // 3 days
				},
				{
					Title:     "Task 4",
					Completed: false,
					Deadline:  new(now.Add(48 * time.Hour)), // 2 days
				},
				{
					Title:     "Task 5",
					Completed: false,
					Deadline:  new(now.Add(120 * time.Hour)), // 5 days
				},
			},
			limit:    3,
			expected: 3,
		},
		{
			title: "all upcoming tasks when limit exceeds list size",
			tasks: []*ItemModel{
				{
					Title:     "Task 1",
					Completed: false,
					Deadline:  new(now.Add(24 * time.Hour)),
				},
				{
					Title:     "Task 2",
					Completed: false,
					Deadline:  new(now.Add(48 * time.Hour)),
				},
			},
			limit:    5,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			result := GetTopUpcomingTasks(tt.tasks, tt.limit)
			if len(result) != tt.expected {
				t.Errorf("GetTopUpcomingTasks() returned %d tasks, expected %d", len(result), tt.expected)
			}

			// Verify tasks are sorted by deadline
			for i := 1; i < len(result); i++ {
				if result[i].Deadline.Before(*result[i-1].Deadline) {
					t.Errorf("GetTopUpcomingTasks() returned tasks in wrong order: %s before %s", result[i].Title, result[i-1].Title)
				}
			}
		})
	}
}

// TestGetTopUpcomingTasksOrdering verifies that tasks are properly sorted by deadline
func TestGetTopUpcomingTasksOrdering(t *testing.T) {
	now := time.Now()
	tasks := []*ItemModel{
		{
			Title:     "Task D",
			Completed: false,
			Deadline:  new(now.Add(96 * time.Hour)), // 4 days
		},
		{
			Title:     "Task A",
			Completed: false,
			Deadline:  new(now.Add(24 * time.Hour)), // 1 day
		},
		{
			Title:     "Task C",
			Completed: false,
			Deadline:  new(now.Add(72 * time.Hour)), // 3 days
		},
		{
			Title:     "Task B",
			Completed: false,
			Deadline:  new(now.Add(48 * time.Hour)), // 2 days
		},
	}

	result := GetTopUpcomingTasks(tasks, 10)

	expectedOrder := []string{"Task A", "Task B", "Task C", "Task D"}
	for i, expectedName := range expectedOrder {
		if result[i].Title != expectedName {
			t.Errorf("Task at position %d is %s, expected %s", i, result[i].Title, expectedName)
		}
	}
}

// TestGetTasksWithoutDeadline tests the GetTasksWithoutDeadline function
func TestGetTasksWithoutDeadline(t *testing.T) {
	now := time.Now()
	tests := []struct {
		title    string
		tasks    []*ItemModel
		expected int
	}{
		{
			title:    "empty task list",
			tasks:    []*ItemModel{},
			expected: 0,
		},
		{
			title: "all tasks have deadlines",
			tasks: []*ItemModel{
				{
					Title:     "Task 1",
					Completed: false,
					Deadline:  new(now.Add(24 * time.Hour)),
				},
				{
					Title:     "Task 2",
					Completed: false,
					Deadline:  new(now.Add(48 * time.Hour)),
				},
			},
			expected: 0,
		},
		{
			title: "all tasks have no deadline",
			tasks: []*ItemModel{
				{
					Title:     "Task 1",
					Completed: false,
					Deadline:  nil,
				},
				{
					Title:     "Task 2",
					Completed: false,
					Deadline:  nil,
				},
			},
			expected: 2,
		},
		{
			title: "mix of tasks with and without deadlines",
			tasks: []*ItemModel{
				{
					Title:     "Task 1",
					Completed: false,
					Deadline:  new(now.Add(24 * time.Hour)),
				},
				{
					Title:     "Task 2",
					Completed: false,
					Deadline:  nil,
				},
				{
					Title:     "Task 3",
					Completed: false,
					Deadline:  nil,
				},
				{
					Title:     "Task 4",
					Completed: false,
					Deadline:  new(now.Add(48 * time.Hour)),
				},
			},
			expected: 2,
		},
		{
			title: "completed tasks without deadline are excluded",
			tasks: []*ItemModel{
				{
					Title:     "Task 1",
					Completed: true,
					Deadline:  nil,
				},
				{
					Title:     "Task 2",
					Completed: false,
					Deadline:  nil,
				},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			result := GetTasksWithoutDeadline(tt.tasks)
			if len(result) != tt.expected {
				t.Errorf("GetTasksWithoutDeadline() returned %d tasks, expected %d", len(result), tt.expected)
			}
		})
	}
}

func TestDeadlineLabel(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York") // embedded via time/tzdata
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	at := func(loc *time.Location, y int, m time.Month, d, h, min int) time.Time {
		return time.Date(y, m, d, h, min, 0, 0, loc)
	}
	cases := []struct {
		name       string
		now        time.Time
		deadline   time.Time
		want       string
		wantUrgent bool
	}{
		{"overdue by hours, same day", at(time.UTC, 2026, 3, 10, 18, 0), at(time.UTC, 2026, 3, 10, 13, 0), "Overdue", true},
		{"overdue since yesterday", at(time.UTC, 2026, 3, 10, 9, 0), at(time.UTC, 2026, 3, 9, 23, 0), "Overdue by 1 day", true},
		{"overdue by 5 days", at(time.UTC, 2026, 3, 10, 9, 0), at(time.UTC, 2026, 3, 5, 9, 0), "Overdue by 5 days", true},
		{"later today", at(time.UTC, 2026, 3, 10, 9, 0), at(time.UTC, 2026, 3, 10, 23, 0), "Due today!", true},
		{"23h ahead crosses midnight", at(time.UTC, 2026, 3, 10, 9, 0), at(time.UTC, 2026, 3, 11, 8, 0), "Due tomorrow", false},
		{"just after midnight", at(time.UTC, 2026, 3, 10, 23, 30), at(time.UTC, 2026, 3, 11, 0, 30), "Due tomorrow", false},
		{"three days", at(time.UTC, 2026, 3, 10, 9, 0), at(time.UTC, 2026, 3, 13, 9, 0), "3 days left", false},
		{"far future", at(time.UTC, 2026, 3, 10, 9, 0), at(time.UTC, 2026, 3, 20, 15, 4), "Mar 20, 3:04 PM", false},
		// US DST starts 2026-03-08 02:00 local: the day before has only 23 hours.
		{"across DST start", at(ny, 2026, 3, 7, 12, 0), at(ny, 2026, 3, 9, 1, 0), "2 days left", false},
		{"across DST end", at(ny, 2026, 10, 31, 23, 0), at(ny, 2026, 11, 1, 23, 30), "Due tomorrow", false},
		{"deadline in another zone uses now's calendar", at(ny, 2026, 3, 10, 21, 0), at(time.UTC, 2026, 3, 11, 3, 0), "Due today!", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, urgent := deadlineLabel(tc.deadline, tc.now)
			if got != tc.want || urgent != tc.wantUrgent {
				t.Fatalf("deadlineLabel = %q/%v, want %q/%v", got, urgent, tc.want, tc.wantUrgent)
			}
		})
	}
}

func TestTaskFilterMatches(t *testing.T) {
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	tasks := map[string]*ItemModel{
		"todo-overdue": {Status: StatusTodo, Deadline: &past, Tags: []string{"work"}},
		"doing":        {Status: StatusDoing, Deadline: &future, Tags: []string{"home", "work"}},
		"done-past":    {Status: StatusDone, Completed: true, Deadline: &past},
		"legacy-done":  {Completed: true},
	}
	cases := []struct {
		name   string
		filter taskFilter
		want   []string
	}{
		{"no filter", taskFilter{}, []string{"doing", "done-past", "legacy-done", "todo-overdue"}},
		{"pending", taskFilter{pending: true}, []string{"doing", "todo-overdue"}},
		{"completed", taskFilter{completed: true}, []string{"done-past", "legacy-done"}},
		{"overdue excludes done", taskFilter{overdue: true}, []string{"todo-overdue"}},
		{"status doing", taskFilter{status: StatusDoing}, []string{"doing"}},
		{"status done derives legacy rows", taskFilter{status: StatusDone}, []string{"done-past", "legacy-done"}},
		{"tag", taskFilter{tags: []string{"work"}}, []string{"doing", "todo-overdue"}},
		{"all tags must match", taskFilter{tags: []string{"work", "home"}}, []string{"doing"}},
		{"combined AND", taskFilter{pending: true, tags: []string{"work"}, overdue: true}, []string{"todo-overdue"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for name, task := range tasks {
				if tc.filter.matches(task, now) {
					got = append(got, name)
				}
			}
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStatusTransitions(t *testing.T) {
	if got := []TaskStatus{nextStatus(StatusTodo), nextStatus(StatusDoing), nextStatus(StatusDone), nextStatus("")}; got[0] != StatusDoing || got[1] != StatusDone || got[2] != StatusTodo || got[3] != StatusDoing {
		t.Fatalf("unexpected status cycle %v", got)
	}

	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	task := &ItemModel{}
	task.setStatus(StatusDone, now)
	if !task.Completed || task.CompletedAt == nil || !task.CompletedAt.Equal(now) {
		t.Fatalf("setStatus(done) = %+v", task)
	}
	task.setStatus(StatusDoing, now)
	if task.Completed || task.CompletedAt != nil || task.Status != StatusDoing {
		t.Fatalf("setStatus(doing) = %+v", task)
	}

	if _, err := parseTaskStatus("blocked"); err == nil {
		t.Fatal("expected invalid status error")
	}
	if s, err := parseTaskStatus(" Done "); err != nil || s != StatusDone {
		t.Fatalf("parseTaskStatus(\" Done \") = %q, %v", s, err)
	}
}
