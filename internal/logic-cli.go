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
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func PrintList(tasks []ItemModel) {
	for _, t := range tasks {
		fmt.Printf(" %v- %s: %s\n -Complete: %t", t.ID, t.Title, t.Description, t.Completed)
	}
}

// -------------------------------------- export ------------------------------------------- //

type exportOpts struct {
	File             string
	Pretty           bool
	Stdout           bool
	IncludeCompleted bool
	Tags             []string
	Statuses         []string
	DryRun           bool
}

func NewExportCmd() *cobra.Command {
	opts := exportOpts{}

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export tasks to JSON",
		Long:  "Export tasks to a versioned JSON file for backup/migration.",
		Example: `  munus export -f tasks.json
  munus export --status todo --tag work --stdout > work-todo.json
  munus export --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve default filename if not stdout
			if !opts.Stdout && opts.File == "" {
				opts.File = fmt.Sprintf("munus-export-%s.json", time.Now().Format("20060102"))
			}

			// Build filter
			filter := jsonio.ExportFilter{
				IncludeCompleted: opts.IncludeCompleted,
				Tags:             opts.Tags,
				Statuses:         opts.Statuses,
			}

			// service/repo wiring - adapt to your existing constructors
			svc, err := buildTaskService()
			if err != nil {
				return err
			}

			plan, err := jsonio.PlanExport(svc, filter)
			if err != nil {
				return err
			}

			if opts.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(),
					"Would export %d tasks (todo:%d doing:%d done:%d)\n",
					plan.Total, plan.Todo, plan.Doing, plan.Done)
				return nil
			}

			if opts.Stdout {
				payload, err := jsonio.ExportToBytes(svc, filter, opts.Pretty)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(payload)
				return err
			}

			if err := jsonio.ExportToFile(svc, filter, opts.File, opts.Pretty); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "✓ Exported %d tasks to %s (v1)\n", plan.Total, opts.File)
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.File, "file", "f", "", "Output JSON file path")
	cmd.Flags().BoolVar(&opts.Pretty, "pretty", true, "Pretty-print JSON output")
	cmd.Flags().BoolVar(&opts.Stdout, "stdout", false, "Write JSON to stdout")
	cmd.Flags().BoolVar(&opts.IncludeCompleted, "include-completed", false, "Include completed tasks")
	cmd.Flags().StringSliceVar(&opts.Tags, "tag", nil, "Filter by tag (repeatable)")
	cmd.Flags().StringSliceVar(&opts.Statuses, "status", nil, "Filter by status (repeatable: todo|doing|done)")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show what would be exported without writing")

	return cmd
}

func buildTaskService() (*jsonio.TaskServiceAdapter, error) {
	return jsonio.NewTaskServiceAdapterFromEnv()
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

//---------------------------------------------import-----------------------------------------//

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

func newImportCmd() *cobra.Command {
	opts := importOpts{}

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import tasks from JSON",
		Long:  "Import tasks from a versioned JSON export.",
		Example: `  munus import -f tasks.json
  munus import -f tasks.json --mode replace --yes --backup
  munus import -f tasks.json --dry-run --strict`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.File == "" {
				return errors.New("required flag: --file")
			}

			svc, err := buildTaskService()
			if err != nil {
				return err
			}

			cfg := jsonio.ImportConfig{
				Mode:       opts.Mode,
				OnConflict: opts.OnConflict,
				IDStrategy: opts.IDStrategy,
				Strict:     opts.Strict,
				DryRun:     opts.DryRun,
				Backup:     opts.Backup,
			}

			plan, err := jsonio.PlanImport(svc, opts.File, cfg)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Importing: %s\nSchema: v%d\nIncoming tasks: %d\nMode: %s (conflict=%s, ids=%s)\n\n",
				opts.File, plan.SchemaVersion, plan.Incoming, opts.Mode, opts.OnConflict, opts.IDStrategy)
			fmt.Fprintf(cmd.OutOrStdout(), "Plan:\n  Create: %d\n  Update: %d\n  Unchanged: %d\n  Conflicts: %d\n\n",
				plan.ToCreate, plan.ToUpdate, plan.Unchanged, plan.Conflicts)

			if opts.DryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "Dry-run only. No changes applied.")
				return nil
			}

			if opts.Mode == "replace" && !opts.Yes {
				ok, err := confirm(cmd, fmt.Sprintf(
					"This will replace all local tasks (current: %d, incoming: %d). Continue? [y/N]: ",
					plan.Current, plan.Incoming,
				))
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("aborted by user")
				}
			}

			res, err := jsonio.ApplyImport(svc, opts.File, cfg)
			if err != nil {
				return err
			}

			if res.BackupPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Backup created: %s\n", res.BackupPath)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"✓ Import complete: created=%d updated=%d unchanged=%d skipped=%d conflicted=%d\n",
				res.Created, res.Updated, res.Unchanged, res.Skipped, res.Conflicted)

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.File, "file", "f", "", "Input JSON file path ('-' for stdin if implemented)")
	cmd.Flags().StringVar(&opts.Mode, "mode", "merge", "Import mode: merge|replace")
	cmd.Flags().StringVar(&opts.OnConflict, "on-conflict", "overwrite", "Conflict policy: skip|overwrite|rename")
	cmd.Flags().StringVar(&opts.IDStrategy, "id-strategy", "preserve", "ID policy: preserve|regenerate")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Validate and show plan without applying")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&opts.Strict, "strict", false, "Fail on unknown fields/invalid enums")
	cmd.Flags().BoolVar(&opts.Backup, "backup", false, "Create backup before applying changes")

	return cmd
}

func confirm(cmd *cobra.Command, prompt string) (bool, error) {
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	r := bufio.NewReader(cmd.InOrStdin())
	s, err := r.ReadString('\n')
	if err != nil {
		return false, err
	}
	return s == "y\n" || s == "Y\n" || s == "yes\n" || s == "YES\n", nil
}

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
	for _, s := range f.Statuses {
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
