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
)

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

// --------------------------------------------------------------------------------------------------------------------------------------think about it?
func NewTaskServiceAdapterFromEnv() (*TaskServiceAdapter, error) { return &TaskServiceAdapter{}, nil }
func (s *TaskServiceAdapter) ListTasks() ([]Task, error)         { return nil, nil }
func (s *TaskServiceAdapter) ReplaceAll([]Task) error            { return nil }
func (s *TaskServiceAdapter) UpsertMany([]Task) error            { return nil }

type ExportFilter struct {
	IncludeCompleted bool
	Tags             []string
	Status           []string
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

type exportOpts struct {
	File             string
	Pretty           bool
	Stdout           bool
	IncludeCompleted bool
	Tags             []string
	Status           []string
	DryRun           bool
}

type importOpts struct {
	File       string
	Mode       string // merge|replace
	OnConflict string // skip|overwrite|rename
	IDStrategy string // preserve|regenerate
	DryRun     bool
	Yes        bool
	Strict     bool
	Backup     bool
}

//-----------------------------------Export-------------------------------//

func buildTaskService() (*TaskServiceAdapter, error) {
	return NewTaskServiceAdapterFromEnv() // ------------------------------------------------------------------------------------------think about it?
}

func PlanExport(svc *TaskServiceAdapter, f ExportFilter) (ExportPlan, error) {
	tasks, err := svc.ListTasks()
	if err != nil {
		return ExportPlan{}, err
	}
	var p ExportPlan
	for _, t := range filterTasks(tasks, f) {
		p.Total++
		switch t.Status {
		case "todo":
			p.Todo++
		case "doing":
			p.Doing++
		case "done":
			p.Done++
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
		Status:      t.Status,
		Priority:    t.Priority,
		Tags:        t.Tags,
		DueAt:       t.DueAt,
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
		if dto.Status != "todo" && dto.Status != "doing" && dto.Status != "done" {
			return nil, 0, fmt.Errorf("tasks[%d].status invalid: %q", i, dto.Status)
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
		Status:      d.Status,
		Priority:    d.Priority,
		Tags:        d.Tags,
		DueAt:       d.DueAt,
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
		a.Status == b.Status &&
		a.Priority == b.Priority
}

func filterTasks(in []Task, f ExportFilter) []Task {
	out := make([]Task, 0, len(in))
	statusOK := map[string]bool{}
	tagOK := map[string]bool{}
	for _, s := range f.Status {
		statusOK[s] = true
	}
	for _, t := range f.Tags {
		tagOK[t] = true
	}

	for _, t := range in {
		if !f.IncludeCompleted && t.Status == "done" {
			continue
		}
		if len(statusOK) > 0 && !statusOK[t.Status] {
			continue
		}
		if len(tagOK) > 0 {
			match := false
			for _, tg := range t.Tags {
				if tagOK[tg] {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

func newID() string {
	return fmt.Sprintf("tsk_%d", time.Now().UnixNano())
}
