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
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Database owns the gorm connection for internal data access.
type Database struct {
	conn *gorm.DB
}

//--------------------------------------Models-------------------------------------//

// ItemModel Represents an item
type ItemModel struct {
	ID          uint       `gorm:"primaryKey"`
	Title       string     `gorm:"size:255;not null"`
	Description string     `gorm:"type:text"`
	Deadline    *time.Time `gorm:"column:deadline"`
	Completed   bool       `gorm:"default:false;not null"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
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
	viewportWidth    int
	viewportHeight   int
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

// NewFormModel creates a new form model
func NewFormModel(storage Storage) *FormModel {
	return &FormModel{
		storage:      storage,
		fields:       make([]string, 3),
		currentField: titleField,
	}
}

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

//JSON models

//export structure
type ExportBundle struct {
	Version    int        `json:"version"`
	ExportedAt time.Time  `json:"exported_at"`
	Tasks      []TaskJSON `json:"tasks"`
}

type TaskJSON struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      string     `json:"status"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

//-----------------------------------interface tasks-------------------------------//

// Storage database sql interface
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
