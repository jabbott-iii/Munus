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
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ExportJSON function
func (s *Service) ExportJSON(path string) error {
	tasks, err := s.repo.List()
	if err != nil {
		return err
	}

	out := ExportBundle{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Tasks:      make([]TaskJSON, 0, len(tasks)),
	}

	for _, t := range tasks {
		out.Tasks = append(out.Tasks, toDTO(t))
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path) // atomic on same filesystem
}

// ImportJSON function
func (s *Service) ImportJSON(path, mode string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var in ExportBundle
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	if in.Version != 1 {
		return fmt.Errorf("unsupported import version: %d", in.Version)
	}

	imported := make([]Task, 0, len(in.Tasks))
	seen := map[string]struct{}{}
	for _, dto := range in.Tasks {
		if dto.Title == "" {
			return fmt.Errorf("task id=%q missing title", dto.ID)
		}
		if _, ok := seen[dto.ID]; ok && dto.ID != "" {
			return fmt.Errorf("duplicate id in import: %s", dto.ID)
		}
		seen[dto.ID] = struct{}{}
		imported = append(imported, fromDTO(dto))
	}

	switch mode {
	case "replace":
		if err := s.repo.ReplaceAll(imported); err != nil {
			return err
		}
	case "merge":
		existing, err := s.repo.List()
		if err != nil {
			return err
		}
		merged := mergeTasks(existing, imported) // keep existing or overwrite by id (choose rule)
		if err := s.repo.ReplaceAll(merged); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid mode: %s", mode)
	}

	return nil
}

//---------------------------------------------types export/import---------------------------------//

type Task struct {
	ID          string
	Title       string
	Description string
	Status      string // todo|doing|done
	Priority    string
	Tags        []string
	DueAt       *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TaskServiceAdapter struct{}

// replace with real service
func NewTaskServiceAdapterFromEnv() (*TaskServiceAdapter, error) { return &TaskServiceAdapter{}, nil }
func (s *TaskServiceAdapter) ListTasks() ([]Task, error)         { return nil, nil }
func (s *TaskServiceAdapter) ReplaceAll([]Task) error            { return nil }
func (s *TaskServiceAdapter) UpsertMany([]Task) error            { return nil }

type ExportBundle struct {
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exported_at"`
	Tasks      []TaskDTO `json:"tasks"`
}

type TaskDTO struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ExportFilter struct {
	IncludeCompleted bool
	Tags             []string
	Statuses         []string
}

type ExportPlan struct {
	Total int
	Todo  int
	Doing int
	Done  int
}

type ImportConfig struct {
	Mode       string
	OnConflict string
	IDStrategy string
	Strict     bool
	DryRun     bool
	Backup     bool
}

type ImportPlan struct {
	SchemaVersion int
	Incoming      int
	Current       int
	ToCreate      int
	ToUpdate      int
	Unchanged     int
	Conflicts     int
}

type ImportResult struct {
	Created    int
	Updated    int
	Unchanged  int
	Skipped    int
	Conflicted int
	BackupPath string
}
