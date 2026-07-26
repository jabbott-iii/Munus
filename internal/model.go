package internal

import "time"

// Munus Represents an item
type Munus struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	Completed   bool       `json:"completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsOverdue checks if the item is overdue
func (t *Munus) IsOverdue() bool {
	if t.Deadline == nil || t.Completed {
		return false
	}
	return t.Deadline.Before(time.Now())
}

// DaysUntilDeadline returns the number of days until the deadline
func (t *Munus) DaysUntilDeadline() int {
	if t.Deadline == nil {
		return -1
	}
	duration := time.Until(*t.Deadline)
	return int(duration.Hours() / 24)
}

// MarkComplete marks the item as complete
func (t *Munus) MarkComplete() {
	t.Completed = true
	now := time.Now()
	t.CompletedAt = &now
	t.UpdatedAt = now
}

// MarkIncomplete marks the item as incomplete
func (t *Munus) MarkIncomplete() {
	t.Completed = false
	t.CompletedAt = nil
	t.UpdatedAt = time.Now()
}
