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
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// MaxImportFileSize caps how much data an import will read into memory.
const MaxImportFileSize = 32 << 20 // 32 MiB

// maxImportedTaskID bounds IDs kept from an import file. Larger IDs are treated
// like non-numeric IDs (a new ID is assigned) so a crafted file cannot push
// SQLite's AUTOINCREMENT counter to its limit and block future inserts.
const maxImportedTaskID = 1<<31 - 1

// databaseFiler is implemented by storage backed by a file on disk.
type databaseFiler interface {
	databaseFile() string
}

//---------------------------------------------types export/import---------------------------------//

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
			Completed:   item.Completed,
			Deadline:    item.Deadline,
			CompletedAt: item.CompletedAt,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}
	return out, nil
}

// ReplaceAll replaces every stored task with tasks. Tasks whose ID is a
// positive integer keep that ID; any other ID (empty, or a generated
// placeholder from rename/regenerate) receives a new database-assigned ID.
// Database storage performs the replacement in a single transaction.
func (s *TaskServiceAdapter) ReplaceAll(tasks []Task) error {
	if s.storage == nil {
		return fmt.Errorf("task storage is not configured")
	}

	items := make([]*ItemModel, 0, len(tasks))
	for _, t := range tasks {
		id, _ := parseTaskID(t.ID)
		items = append(items, &ItemModel{
			ID:          id,
			Title:       t.Title,
			Description: t.Description,
			Completed:   t.Completed,
			Deadline:    t.Deadline,
			CompletedAt: t.CompletedAt,
			CreatedAt:   t.CreatedAt,
			UpdatedAt:   t.UpdatedAt,
		})
	}

	return s.storage.ReplaceAllTasks(items)
}

// parseTaskID returns the numeric database ID encoded in an exported task ID.
func parseTaskID(id string) (int, bool) {
	n, err := strconv.Atoi(id)
	if err != nil || n <= 0 || n > maxImportedTaskID {
		return 0, false
	}
	return n, true
}

// func (s *TaskServiceAdapter) InsertFiles([]Task) error { return nil }

//-----------------------------------Export-------------------------------//

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
		out.Tasks = append(out.Tasks, toDTO(TaskDTO(t)))
	}

	if pretty {
		return json.MarshalIndent(out, "", "  ")
	}
	return json.Marshal(out)
}

func ExportToFile(svc *TaskServiceAdapter, f ExportFilter, path string, pretty bool) error {
	if err := refuseDatabaseTarget(svc, path); err != nil {
		return err
	}
	b, err := ExportToBytes(svc, f, pretty)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, b)
}

// refuseDatabaseTarget stops an export from overwriting the live database.
func refuseDatabaseTarget(svc *TaskServiceAdapter, path string) error {
	if svc == nil {
		return nil
	}
	filer, ok := svc.storage.(databaseFiler)
	if !ok || filer.databaseFile() == "" {
		return nil
	}
	target, err := os.Stat(path)
	if err != nil {
		return nil // nothing exists at path, so it cannot be the database
	}
	dbInfo, err := os.Stat(filer.databaseFile())
	if err != nil {
		return nil
	}
	if os.SameFile(target, dbInfo) {
		return fmt.Errorf("refusing to export over the active database %q", path)
	}
	return nil
}

// writeFileAtomic writes data to a new owner-only temp file in the target
// directory and renames it into place. The temp name is unpredictable and
// created exclusively, so a pre-planted file or symlink is never followed.
func writeFileAtomic(path string, data []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".munus-export-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func toDTO(t TaskDTO) TaskDTO {
	return TaskDTO{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Deadline:    t.Deadline,
		Completed:   t.Completed,
		CompletedAt: t.CompletedAt,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

//------------------------------------------------import-------------------------------------------//

// normalizeImportConfig fills defaults and rejects unknown option values
// before any file is read or backup is written.
func normalizeImportConfig(cfg ImportConfig) (ImportConfig, error) {
	if cfg.Mode == "" {
		cfg.Mode = "merge"
	}
	if cfg.IDStrategy == "" {
		cfg.IDStrategy = "preserve"
	}
	switch cfg.Mode {
	case "merge", "replace":
	default:
		return cfg, fmt.Errorf("invalid mode %q (use merge or replace)", cfg.Mode)
	}
	switch cfg.OnConflict {
	case "", "skip", "overwrite", "rename":
	default:
		return cfg, fmt.Errorf("invalid conflict policy %q (use skip, overwrite or rename)", cfg.OnConflict)
	}
	switch cfg.IDStrategy {
	case "preserve", "regenerate":
	default:
		return cfg, fmt.Errorf("invalid id strategy %q (use preserve or regenerate)", cfg.IDStrategy)
	}
	return cfg, nil
}

// PlanImport previews ApplyImport without changing storage. It uses the same
// merge logic as ApplyImport so the preview matches the result.
func PlanImport(svc *TaskServiceAdapter, file string, cfg ImportConfig) (ImportPlan, error) {
	cfg, err := normalizeImportConfig(cfg)
	if err != nil {
		return ImportPlan{}, err
	}
	incoming, version, err := readImportFile(file, cfg.Strict)
	if err != nil {
		return ImportPlan{}, err
	}
	current, err := svc.ListTasks()
	if err != nil {
		return ImportPlan{}, err
	}

	plan := ImportPlan{
		SchemaVersion: version,
		Incoming:      len(incoming),
		Current:       len(current),
	}

	if cfg.Mode == "replace" {
		plan.ToCreate = len(incoming)
		return plan, nil
	}

	_, res := merge(current, incoming, cfg)
	plan.ToCreate = res.Created
	plan.ToUpdate = res.Updated
	plan.Unchanged = res.Unchanged
	plan.Conflicts = res.Conflicted
	plan.ConflictIDs = res.ConflictIDs
	return plan, nil
}

func ApplyImport(svc *TaskServiceAdapter, file string, cfg ImportConfig) (ImportResult, error) {
	cfg, err := normalizeImportConfig(cfg)
	if err != nil {
		return ImportResult{}, err
	}
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
		if cfg.IDStrategy == "regenerate" {
			for i := range incoming {
				incoming[i].ID = ""
			}
		}
		if err := svc.ReplaceAll(incoming); err != nil {
			return res, err
		}
		res.Created = len(incoming)
		return res, nil
	default: // merge
		merged, mr := merge(current, incoming, cfg)
		if err := svc.ReplaceAll(merged); err != nil {
			return res, err
		}
		mr.BackupPath = res.BackupPath
		return mr, nil
	}
}

func readImportFile(path string, strict bool) ([]Task, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = f.Close() }()

	b, err := io.ReadAll(io.LimitReader(f, MaxImportFileSize+1))
	if err != nil {
		return nil, 0, err
	}
	if len(b) > MaxImportFileSize {
		return nil, 0, fmt.Errorf("import file exceeds maximum size of %d bytes", MaxImportFileSize)
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

	now := time.Now()
	out := make([]Task, 0, len(bundle.Tasks))
	seen := map[string]struct{}{}
	for i, dto := range bundle.Tasks {
		if dto.Title == "" {
			return nil, 0, fmt.Errorf("tasks[%d].title is required", i)
		}
		// Strict mode rejects control characters; otherwise they are removed so
		// exports/backups of older data always re-import.
		if !strict {
			dto.Title = stripControlCharacters(dto.Title, false)
			dto.Description = stripControlCharacters(dto.Description, true)
		}
		if err := ValidateTaskText(dto.Title, dto.Description); err != nil {
			return nil, 0, fmt.Errorf("tasks[%d]: %w", i, err)
		}
		// Canonicalise numeric IDs so "01" and "1" refer to the same task.
		if n, ok := parseTaskID(dto.ID); ok {
			dto.ID = strconv.Itoa(n)
		}
		if dto.ID != "" {
			if _, ok := seen[dto.ID]; ok {
				return nil, 0, fmt.Errorf("duplicate id in import: %q", dto.ID)
			}
			seen[dto.ID] = struct{}{}
		}
		// Keep completion time consistent with the completed flag; files
		// written before completed_at existed fall back to updated_at.
		if !dto.Completed {
			dto.CompletedAt = nil
		} else if dto.CompletedAt == nil {
			completedAt := dto.UpdatedAt
			if completedAt.IsZero() {
				completedAt = now
			}
			dto.CompletedAt = &completedAt
		}
		out = append(out, fromDTO(Task(dto)))
	}
	return out, bundle.Version, nil
}

func fromDTO(d Task) Task {
	return Task{
		ID:          d.ID,
		Title:       d.Title,
		Description: d.Description,
		Completed:   d.Completed,
		Deadline:    d.Deadline,
		CompletedAt: d.CompletedAt,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

// merge combines current and incoming tasks. Tasks that end up with a
// generated placeholder ID (see newID) are assigned a real ID on write.
func merge(current, incoming []Task, cfg ImportConfig) ([]Task, ImportResult) {
	res := ImportResult{}
	conflictPolicy := effectiveOnConflict(cfg)
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

		res.Conflicted++
		res.ConflictIDs = append(res.ConflictIDs, id)

		switch conflictPolicy {
		case "skip":
			res.Skipped++
			res.SkippedIDs = append(res.SkippedIDs, id)
		case "rename":
			in.ID = newID()
			byID[in.ID] = in
			order = append(order, in.ID)
			res.Created++
		default: // overwrite
			// A task that stays completed keeps its original completion time.
			if ex.Completed && in.Completed && ex.CompletedAt != nil {
				in.CompletedAt = ex.CompletedAt
			}
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

func effectiveOnConflict(cfg ImportConfig) string {
	if cfg.SkipExisting && (cfg.OnConflict == "" || cfg.OnConflict == "overwrite") {
		return "skip"
	}
	if cfg.OnConflict == "" {
		return "overwrite"
	}
	return cfg.OnConflict
}

func formatTaskIDs(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return strings.Join(ids, ", ")
}

func writeBackup(tasks []Task) (string, error) {
	bundle := ExportBundle{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Tasks:      make([]TaskDTO, 0, len(tasks)),
	}
	for _, t := range tasks {
		bundle.Tasks = append(bundle.Tasks, toDTO(TaskDTO(t)))
	}
	b, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory for backup: %w", err)
	}
	dir := filepath.Join(home, ".munus", "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	// Tighten a directory created with broader permissions by older versions.
	if info, err := os.Stat(dir); err == nil && info.Mode().Perm()&0o077 != 0 {
		_ = os.Chmod(dir, 0o700)
	}

	// CreateTemp gives each backup a unique, owner-only (0600) file, so two
	// backups in the same second never overwrite each other.
	f, err := os.CreateTemp(dir, "tasks-"+time.Now().Format("20060102-150405")+"-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func equalTask(a, b Task) bool {
	return a.Title == b.Title &&
		a.Description == b.Description &&
		a.Completed == b.Completed &&
		equalDeadline(a.Deadline, b.Deadline)
}

func equalDeadline(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
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

var newIDCounter atomic.Uint64

// newID returns a unique placeholder ID for tasks that need a new database ID.
// The counter keeps IDs unique even when the clock does not advance.
func newID() string {
	return fmt.Sprintf("tsk_%d_%d", time.Now().UnixNano(), newIDCounter.Add(1))
}
