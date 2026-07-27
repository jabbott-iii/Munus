package internal

import (
	"errors"
	"fmt"
	"sort"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Database owns the gorm connection for internal data access.
type Database struct {
	conn *gorm.DB
}

type Storage interface {
	CreateTask(task *ItemModel) error
	GetTaskByID(id uint) (*ItemModel, error)
	ListTasks() ([]*ItemModel, error)
	UpdateTask(task *ItemModel) error
	DeleteTask(id uint) error
}

// NewDatabase opens (or creates) the sqlite file and runs migrations.
func NewDatabase(path string) (*Database, error) {
	if path == "" {
		path = "munus.db"
	}

	conn, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// Persistence structs
	if err := conn.AutoMigrate(&ItemModel{}); err != nil {
		return nil, fmt.Errorf("auto-migrate schema: %w", err)
	}

	return &Database{conn: conn}, nil
}

// Conn exposes the raw gorm handle for advanced queries/transactions.
func (d *Database) Conn() *gorm.DB {
	return d.conn
}

// CreateTask persists a new task.
func (d *Database) CreateTask(task *ItemModel) error {
	if task == nil {
		return errors.New("task is nil")
	}
	return d.conn.Create(task).Error
}

// ListTasks returns all tasks.
func (d *Database) ListTasks() ([]*ItemModel, error) {
	var tasks []*ItemModel
	if err := d.conn.Order("id DESC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetTaskByID fetches a task by primary key.
func (d *Database) GetTaskByID(id uint) (*ItemModel, error) {
	var task ItemModel
	if err := d.conn.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTask saves changes to an existing task.
func (d *Database) UpdateTask(task *ItemModel) error {
	if task == nil {
		return errors.New("task is nil")
	}
	if task.ID == 0 {
		return errors.New("task id is required")
	}
	return d.conn.Save(task).Error
}

// DeleteTask deletes a task by id.
func (d *Database) DeleteTask(id uint) error {
	if id == 0 {
		return errors.New("task id is required")
	}
	return d.conn.Delete(&ItemModel{}, id).Error
}

// GetTopUpcomingTasks returns the top N todos with the closest deadline
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

// GetTasksWithoutDeadline returns todos without deadline
func GetTasksWithoutDeadline(tasks []*ItemModel) []*ItemModel {
	var noDeadlineTasks []*ItemModel
	for _, task := range tasks {
		if !task.Completed && task.Deadline == nil {
			noDeadlineTasks = append(noDeadlineTasks, task)
		}
	}
	return noDeadlineTasks
}
