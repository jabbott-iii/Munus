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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

//---------------------------------------------types export/import---------------------------------//

func NewTaskServiceAdapterFromEnv() (*TaskServiceAdapter, error) {
	dbPath := os.Getenv("MUNUS_DB_PATH")
	if dbPath == "" {
		dbPath = "munus.db"
		}

	db, err := NewDatabase(dbPath)
	if err != nil {
		return nil, err
		}
	
	return &TaskServiceAdapter {storage: db}, nil 
}

func (s *TaskServiceAdapter) ListTasks() ([]Task, error) {
	items, err := s.storage.ListTasks()
	if err != nil {
		return nil, err
	}

	out := make([]Task, 0, len(items))
	for _, item := range items {
		out = append(out, Task{
			ID:          fmt.Sprintf("%d", item.ID),
			Title:       item.Title,
			Description: item.Description,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return out, nil
}

func (s *TaskServiceAdapter) ReplaceAll(tasks []Task) error {
	if s.storage == nil {
		return fmt.Errorf("task storage is not configured")
	}

	// If your Database exposes the raw gorm DB, use a transaction for safety.
	db, ok := s.storage.(*Database)
	if !ok {
		return fmt.Errorf("storage does not support replace-all")
	}

	return db.Conn().Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&ItemModel{}).Error; err != nil {
			return err
		}

		for _, t := range tasks {
			item := &ItemModel{
				Title:       t.Title,
				Description: t.Description,
				Completed:   t.Completed,
				Deadline:    t.Deadline,
				CreatedAt:   t.CreatedAt,
				UpdatedAt:   t.UpdatedAt,
			}
			if err := tx.Create(item).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// func (s *TaskServiceAdapter) InsertFiles([]Task) error            { return nil }

//-----------------------------------Export-------------------------------//

func buildTaskService() (*TaskServiceAdapter, error) {
	dbPath := os.Getenv("MUNUS_DB_PATH")
	db, err := NewDatabase(dbPath)
	if err != nil {
		return nil, err
	}

	return &TaskServiceAdapter{
		storage: db,
	}, nil
}

func PlanExport(svc *TaskServiceAdapter, f ExportFilter) (ExportPlan, error) {
	tasks, err := svc.ListTasks()
	if err != nil {
		return ExportPlan{}, err
	}
	var p ExportPlan
	for _, t := range filterTasks(tasks, f) {
		p.Total++
		if t.Completed {
			p.Done++
		} else {
			p.Todo++
		}
	}
	return p, nil
}

func ExportToBytes(svc *TaskServiceAdapter, f ExportFilter, pretty bool) ([]byte, error) {
	tasks, err := svc.ListTasks()
	if err != nil {
		return nil, err
	}
	filtered := filterTasks(tasks, f)

	out := ExportBundle{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Tasks:      make([]TaskDTO, 0, len(filtered)),
	}
	for _, t := range filtered {
		out.Tasks = append(out.Tasks, toDTO(t))
	}

	if pretty {
		return json.MarshalIndent(out, "", "  ")
	}
	return json.Marshal(out)
}

func ExportToFile(svc *TaskServiceAdapter, f ExportFilter, path string, pretty bool) error {
	b, err := ExportToBytes(svc, f, pretty)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func toDTO(t Task) TaskDTO {
	return TaskDTO{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Priority:    t.Priority,
		Deadline:    t.Deadline,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

//------------------------------------------------import-------------------------------------------//

func PlanImport(svc *TaskServiceAdapter, file string, cfg ImportConfig) (ImportPlan, error) {
	incoming, version, err := readImportFile(file, cfg.Strict)
	if err != nil {
		return ImportPlan{}, err
	}
	current, err := svc.ListTasks()
	if err != nil {
		return ImportPlan{}, err
	}

	currByID := map[string]Task{}
	for _, t := range current {
		currByID[t.ID] = t
	}

	plan := ImportPlan{
		SchemaVersion: version,
		Incoming:      len(incoming),
		Current:       len(current),
	}

	for _, t := range incoming {
		if old, ok := currByID[t.ID]; !ok {
			plan.ToCreate++
		} else if equalTask(old, t) {
			plan.Unchanged++
		} else {
			plan.ToUpdate++
		}
	}
	return plan, nil
}

func ApplyImport(svc *TaskServiceAdapter, file string, cfg ImportConfig) (ImportResult, error) {
	incoming, _, err := readImportFile(file, cfg.Strict)
	if err != nil {
		return ImportResult{}, err
	}
	current, err := svc.ListTasks()
	if err != nil {
		return ImportResult{}, err
	}

	res := ImportResult{}
	if cfg.Backup {
		p, err := writeBackup(current)
		if err != nil {
			return res, err
		}
		res.BackupPath = p
	}

	switch cfg.Mode {
	case "replace":
		if err := svc.ReplaceAll(incoming); err != nil {
			return res, err
		}
		res.Created = len(incoming)
		return res, nil
	case "merge":
		merged, mr := merge(current, incoming, cfg)
		if err := svc.ReplaceAll(merged); err != nil {
			return res, err
		}
		return mr, nil
	default:
		return res, fmt.Errorf("invalid mode %q", cfg.Mode)
	}
}

func readImportFile(path string, strict bool) ([]Task, int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	var bundle ExportBundle
	if strict {
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&bundle); err != nil {
			return nil, 0, err
		}
	} else {
		if err := json.Unmarshal(b, &bundle); err != nil {
			return nil, 0, err
		}
	}

	if bundle.Version != 1 {
		return nil, 0, fmt.Errorf("unsupported import version: %d", bundle.Version)
	}

	out := make([]Task, 0, len(bundle.Tasks))
	seen := map[string]struct{}{}
	for i, dto := range bundle.Tasks {
		if dto.Title == "" {
			return nil, 0, fmt.Errorf("tasks[%d].title is required", i)
		}
		if dto.ID != "" {
			if _, ok := seen[dto.ID]; ok {
				return nil, 0, fmt.Errorf("duplicate id in import: %s", dto.ID)
			}
			seen[dto.ID] = struct{}{}
		}
		out = append(out, fromDTO(dto))
	}
	return out, bundle.Version, nil
}

func fromDTO(d TaskDTO) Task {
	return Task{
		ID:          d.ID,
		Title:       d.Title,
		Description: d.Description,
		Completed:   d.Completed,
		Priority:    d.Priority,
		Deadline:    d.Deadline,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func merge(current, incoming []Task, cfg ImportConfig) ([]Task, ImportResult) {
	res := ImportResult{}
	byID := map[string]Task{}
	order := make([]string, 0, len(current))

	for _, t := range current {
		byID[t.ID] = t
		order = append(order, t.ID)
	}

	for _, in := range incoming {
		id := in.ID
		if id == "" || cfg.IDStrategy == "regenerate" {
			id = newID()
			in.ID = id
		}

		ex, exists := byID[id]
		if !exists {
			byID[id] = in
			order = append(order, id)
			res.Created++
			continue
		}

		if equalTask(ex, in) {
			res.Unchanged++
			continue
		}

		switch cfg.OnConflict {
		case "skip":
			res.Skipped++
		case "rename":
			in.ID = newID()
			byID[in.ID] = in
			order = append(order, in.ID)
			res.Created++
		default: // overwrite
			byID[id] = in
			res.Updated++
		}
	}

	merged := make([]Task, 0, len(byID))
	for _, id := range order {
		if t, ok := byID[id]; ok {
			merged = append(merged, t)
		}
	}
	return merged, res
}

func writeBackup(tasks []Task) (string, error) {
	bundle := ExportBundle{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Tasks:      make([]TaskDTO, 0, len(tasks)),
	}
	for _, t := range tasks {
		bundle.Tasks = append(bundle.Tasks, toDTO(t))
	}
	b, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}

	dir := filepath.Join(os.Getenv("HOME"), ".munus", "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "tasks-"+time.Now().Format("20060102-150405")+".json")
	return path, os.WriteFile(path, b, 0o644)
}

func equalTask(a, b Task) bool {
	return a.Title == b.Title &&
		a.Description == b.Description &&
		a.Completed == b.Completed &&
		a.Priority == b.Priority
}

func filterTasks(in []Task, f ExportFilter) []Task {
	out := make([]Task, 0, len(in))

	for _, t := range in {
		if f.IncludeCompleted {
			out = append(out, t)
			continue
		}

		if !t.Completed {
			out = append(out, t)
		}
	}

	return out
}

func newID() string {
	return fmt.Sprintf("tsk_%d", time.Now().UnixNano())
}
