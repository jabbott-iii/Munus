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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// maxImportFileSize caps how much data an import will read into memory.
const maxImportFileSize = 32 << 20 // 32 MiB

// maxImportedTaskID bounds IDs kept from an import file. Larger IDs are treated
// like non-numeric IDs (a new ID is assigned) so a crafted file cannot push
// SQLite's AUTOINCREMENT counter to its limit and block future inserts.
const maxImportedTaskID = 1<<31 - 1

// stdinImportPath is the --file value that reads an import from standard input.
const stdinImportPath = "-"

// databaseFiler is implemented by storage backed by a file on disk.
type databaseFiler interface {
	databaseFile() string
}

//---------------------------------------------types export/import---------------------------------//

func taskFromItem(item *ItemModel) Task {
	return Task{
		ID:          strconv.Itoa(item.ID),
		Title:       item.Title,
		Description: item.Description,
		Completed:   item.Completed,
		Deadline:    item.Deadline,
		CompletedAt: item.CompletedAt,
		Status:      item.Status,
		Tags:        slices.Clone(item.Tags),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

// itemFromTask converts t for storage. Tasks whose ID is a positive integer
// keep that ID; any other ID (empty, or a placeholder from rename/regenerate)
// receives a new database-assigned ID.
func itemFromTask(t Task) *ItemModel {
	id, _ := parseTaskID(t.ID)
	return &ItemModel{
		ID:          id,
		Title:       t.Title,
		Description: t.Description,
		Completed:   t.Completed,
		Deadline:    t.Deadline,
		CompletedAt: t.CompletedAt,
		Status:      taskStatusOf(t),
		Tags:        slices.Clone(t.Tags),
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// taskStatusOf returns t's status, deriving it from Completed when unset.
func taskStatusOf(t Task) TaskStatus {
	if t.Status != "" {
		return t.Status
	}
	if t.Completed {
		return StatusDone
	}
	return StatusTodo
}

func (s *TaskServiceAdapter) ListTasks(ctx context.Context) ([]Task, error) {
	items, err := s.storage.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Task, 0, len(items))
	for _, item := range items {
		out = append(out, taskFromItem(item))
	}
	return out, nil
}

// ReplaceAll replaces every stored task with tasks (see itemFromTask for IDs).
// Database storage performs the replacement in a single transaction.
func (s *TaskServiceAdapter) ReplaceAll(ctx context.Context, tasks []Task) error {
	if s.storage == nil {
		return fmt.Errorf("task storage is not configured")
	}
	items := make([]*ItemModel, 0, len(tasks))
	for _, t := range tasks {
		items = append(items, itemFromTask(t))
	}
	return s.storage.ReplaceAllTasks(ctx, items)
}

// replaceAllFunc reads the current tasks and replaces them with fn's result as
// one atomic storage operation.
func (s *TaskServiceAdapter) replaceAllFunc(ctx context.Context, fn func(current []Task) ([]Task, error)) error {
	if s.storage == nil {
		return fmt.Errorf("task storage is not configured")
	}
	return s.storage.ReplaceAllTasksFunc(ctx, func(items []*ItemModel) ([]*ItemModel, error) {
		current := make([]Task, 0, len(items))
		for _, item := range items {
			current = append(current, taskFromItem(item))
		}
		next, err := fn(current)
		if err != nil {
			return nil, err
		}
		out := make([]*ItemModel, 0, len(next))
		for _, t := range next {
			out = append(out, itemFromTask(t))
		}
		return out, nil
	})
}

// parseTaskID returns the numeric database ID encoded in an exported task ID.
func parseTaskID(id string) (int, bool) {
	n, err := strconv.Atoi(id)
	if err != nil || n <= 0 || n > maxImportedTaskID {
		return 0, false
	}
	return n, true
}

//-----------------------------------Export-------------------------------//

func PlanExport(ctx context.Context, svc *TaskServiceAdapter, f ExportFilter) (ExportPlan, error) {
	tasks, err := svc.ListTasks(ctx)
	if err != nil {
		return ExportPlan{}, err
	}
	var p ExportPlan
	for _, t := range filterTasks(tasks, f) {
		p.Total++
		switch taskStatusOf(t) {
		case StatusDone:
			p.Done++
		case StatusDoing:
			p.Doing++
		default:
			p.Todo++
		}
	}
	return p, nil
}

func ExportToBytes(ctx context.Context, svc *TaskServiceAdapter, f ExportFilter, pretty bool) ([]byte, error) {
	tasks, err := svc.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	return marshalBundle(filterTasks(tasks, f), pretty)
}

func marshalBundle(tasks []Task, pretty bool) ([]byte, error) {
	out := ExportBundle{
		Version:    exportSchemaVersion,
		ExportedAt: time.Now().UTC(),
		Tasks:      make([]TaskDTO, 0, len(tasks)),
	}
	for _, t := range tasks {
		t.Status = taskStatusOf(t)
		out.Tasks = append(out.Tasks, toDTO(TaskDTO(t)))
	}
	if pretty {
		return json.MarshalIndent(out, "", "  ")
	}
	return json.Marshal(out)
}

func ExportToFile(ctx context.Context, svc *TaskServiceAdapter, f ExportFilter, path string, pretty bool) error {
	if err := refuseDatabaseTarget(svc, path); err != nil {
		return err
	}
	b, err := ExportToBytes(ctx, svc, f, pretty)
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
		return nil // the database file cannot be inspected, so it cannot be compared
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
			// Best-effort cleanup of the temp file; the original error is returned.
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		// The write error is what matters; the deferred cleanup removes the file.
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
		Status:      t.Status,
		Tags:        slices.Clone(t.Tags),
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

// PlanImport previews ApplyImport for the file at path without changing storage.
func PlanImport(ctx context.Context, svc *TaskServiceAdapter, file string, cfg ImportConfig) (ImportPlan, error) {
	cfg, err := normalizeImportConfig(cfg)
	if err != nil {
		return ImportPlan{}, err
	}
	data, err := readImportSource(file, nil)
	if err != nil {
		return ImportPlan{}, err
	}
	return planImportData(ctx, svc, data, cfg)
}

// planImportData previews applyImportData. It uses the same merge logic so the
// preview matches the result.
func planImportData(ctx context.Context, svc *TaskServiceAdapter, data []byte, cfg ImportConfig) (ImportPlan, error) {
	cfg, err := normalizeImportConfig(cfg)
	if err != nil {
		return ImportPlan{}, err
	}
	incoming, version, err := parseImportData(data, cfg.Strict)
	if err != nil {
		return ImportPlan{}, err
	}
	current, err := svc.ListTasks(ctx)
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

	_, res := mergeVersion(current, incoming, cfg, version)
	plan.ToCreate = res.Created
	plan.ToUpdate = res.Updated
	plan.Unchanged = res.Unchanged
	plan.Conflicts = res.Conflicted
	plan.ConflictIDs = res.ConflictIDs
	return plan, nil
}

// ApplyImport imports the file at path.
func ApplyImport(ctx context.Context, svc *TaskServiceAdapter, file string, cfg ImportConfig) (ImportResult, error) {
	cfg, err := normalizeImportConfig(cfg)
	if err != nil {
		return ImportResult{}, err
	}
	data, err := readImportSource(file, nil)
	if err != nil {
		return ImportResult{}, err
	}
	return applyImportData(ctx, svc, data, cfg)
}

// applyImportData imports data. The current tasks are read, backed up and
// replaced in one storage transaction, so a write made concurrently is either
// part of the snapshot or happens after the import; it is never lost.
func applyImportData(ctx context.Context, svc *TaskServiceAdapter, data []byte, cfg ImportConfig) (ImportResult, error) {
	cfg, err := normalizeImportConfig(cfg)
	if err != nil {
		return ImportResult{}, err
	}
	incoming, version, err := parseImportData(data, cfg.Strict)
	if err != nil {
		return ImportResult{}, err
	}

	var res ImportResult
	err = svc.replaceAllFunc(ctx, func(current []Task) ([]Task, error) {
		res = ImportResult{}
		if cfg.Backup {
			p, err := writeBackup(ctx, current)
			if err != nil {
				return nil, err
			}
			res.BackupPath = p
		}

		if cfg.Mode == "replace" {
			next := slices.Clone(incoming)
			if cfg.IDStrategy == "regenerate" {
				for i := range next {
					next[i].ID = ""
				}
			}
			res.Created = len(next)
			return next, nil
		}

		merged, mr := mergeVersion(current, incoming, cfg, version)
		mr.BackupPath = res.BackupPath
		res = mr
		return merged, nil
	})
	return res, err
}

// readImportSource reads at most maxImportFileSize bytes from the file at path,
// or from stdin when path is "-".
func readImportSource(path string, stdin io.Reader) ([]byte, error) {
	if path == stdinImportPath {
		if stdin == nil {
			return nil, errors.New("reading an import from standard input is not supported here")
		}
		return readLimited(stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// Read-only file: a close error cannot lose data.
	defer func() { _ = f.Close() }()
	return readLimited(f)
}

func readLimited(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxImportFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxImportFileSize {
		return nil, fmt.Errorf("import file exceeds maximum size of %d bytes", maxImportFileSize)
	}
	return b, nil
}

// readImportFile reads and validates the import file at path.
func readImportFile(path string, strict bool) ([]Task, int, error) {
	data, err := readImportSource(path, nil)
	if err != nil {
		return nil, 0, err
	}
	return parseImportData(data, strict)
}

// parseImportData decodes and validates an export bundle (schema version 1 or 2).
func parseImportData(b []byte, strict bool) ([]Task, int, error) {
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

	if bundle.Version != 1 && bundle.Version != exportSchemaVersion {
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
		if err := validateTaskText(dto.Title, dto.Description); err != nil {
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

		status, err := importedStatus(dto, bundle.Version, strict)
		if err != nil {
			return nil, 0, fmt.Errorf("tasks[%d]: %w", i, err)
		}
		dto.Status = status
		dto.Completed = status == StatusDone

		tags, err := normalizeTags(dto.Tags)
		if err != nil {
			return nil, 0, fmt.Errorf("tasks[%d]: %w", i, err)
		}
		dto.Tags = tags

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

// importedStatus returns the status of an imported task. Version 1 files have
// no status, so it is derived from completed. In version 2 files status wins
// over completed. Strict mode rejects an unknown status or a contradiction;
// otherwise an unknown status falls back to the one derived from completed.
func importedStatus(dto TaskDTO, version int, strict bool) (TaskStatus, error) {
	derived := StatusTodo
	if dto.Completed {
		derived = StatusDone
	}
	if version == 1 || dto.Status == "" {
		return derived, nil
	}
	status, err := parseTaskStatus(string(dto.Status))
	if err != nil {
		if strict {
			return "", err
		}
		return derived, nil
	}
	if strict && (status == StatusDone) != dto.Completed {
		return "", fmt.Errorf("completed=%t contradicts status %q", dto.Completed, status)
	}
	return status, nil
}

func fromDTO(d Task) Task {
	return Task{
		ID:          d.ID,
		Title:       d.Title,
		Description: d.Description,
		Completed:   d.Completed,
		Deadline:    d.Deadline,
		CompletedAt: d.CompletedAt,
		Status:      d.Status,
		Tags:        slices.Clone(d.Tags),
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

// merge combines current tasks with incoming tasks from a current-schema file.
func merge(current, incoming []Task, cfg ImportConfig) ([]Task, ImportResult) {
	return mergeVersion(current, incoming, cfg, exportSchemaVersion)
}

// mergeVersion combines current and incoming tasks. Tasks that need a new
// database ID get a placeholder ID ("tsk_new_<n>") that is unique within this
// merge; the storage layer replaces it with a real ID on write. Version 1
// files carry no tags and no "doing" status, so for them an existing task
// keeps its tags and its "doing" status unless the file marks it done.
func mergeVersion(current, incoming []Task, cfg ImportConfig, version int) ([]Task, ImportResult) {
	res := ImportResult{}
	conflictPolicy := effectiveOnConflict(cfg)
	byID := map[string]Task{}
	order := make([]string, 0, len(current))

	// Reserve every ID already in play so a placeholder can never collide
	// with a current or incoming task ID.
	reserved := make(map[string]struct{}, len(current)+len(incoming))
	for _, t := range current {
		byID[t.ID] = t
		order = append(order, t.ID)
		reserved[t.ID] = struct{}{}
	}
	for _, t := range incoming {
		reserved[t.ID] = struct{}{}
	}
	next := 0
	newPlaceholder := func() string {
		for {
			next++
			id := "tsk_new_" + strconv.Itoa(next)
			if _, taken := reserved[id]; !taken {
				reserved[id] = struct{}{}
				return id
			}
		}
	}

	for _, in := range incoming {
		id := in.ID
		if id == "" || cfg.IDStrategy == "regenerate" {
			id = newPlaceholder()
			in.ID = id
		}

		ex, exists := byID[id]
		if !exists {
			byID[id] = in
			order = append(order, id)
			res.Created++
			continue
		}

		if version == 1 {
			in.Tags = slices.Clone(ex.Tags)
			if taskStatusOf(ex) == StatusDoing && taskStatusOf(in) == StatusTodo {
				in.Status = StatusDoing
			}
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
			in.ID = newPlaceholder()
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

// writeBackup writes tasks as an export bundle to a new, unique, owner-only
// file in ~/.munus/backups and returns its path.
func writeBackup(ctx context.Context, tasks []Task) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	b, err := marshalBundle(tasks, true)
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
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if info.Mode().Perm()&0o077 != 0 {
		// A directory needs its execute bit to be entered, so 0700 (not 0600)
		// is its owner-only mode; gosec's G302 assumes a regular file.
		if err := os.Chmod(dir, 0o700); err != nil { // #nosec G302 -- owner-only directory mode, see above
			return "", fmt.Errorf("restrict backup directory permissions: %w", err)
		}
	}

	// CreateTemp gives each backup a unique, owner-only (0600) file, so two
	// backups in the same second never overwrite each other.
	f, err := os.CreateTemp(dir, "tasks-"+time.Now().Format("20060102-150405")+"-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(b); err != nil {
		// Best-effort cleanup of the partial backup; the write error is returned.
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		// Best-effort cleanup of the partial backup; the close error is returned.
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func equalTask(a, b Task) bool {
	return a.Title == b.Title &&
		a.Description == b.Description &&
		taskStatusOf(a) == taskStatusOf(b) &&
		equalDeadline(a.Deadline, b.Deadline) &&
		slices.Equal(sortedTags(a.Tags), sortedTags(b.Tags))
}

func sortedTags(tags []string) []string {
	out := slices.Clone(tags)
	slices.Sort(out)
	return out
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
		if f.IncludeCompleted || taskStatusOf(t) != StatusDone {
			out = append(out, t)
		}
	}
	return out
}
