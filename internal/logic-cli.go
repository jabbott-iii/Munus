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
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// Command output follows the repository convention `_, _ = fmt.Fprint…(cmd.OutOrStdout(), …)`:
// a failed write to the terminal or a closed pipe has nowhere better to be
// reported, so those errors are deliberately ignored.

//--------------------------------------CORE----------------------------------------------------------------------------//

// NewRootCmd tui main entry point
func NewRootCmd(db *Database) *cobra.Command {
	var vim bool

	cmd := &cobra.Command{
		Use: "munus",
		// The database is opened only once a command actually runs, so --help,
		// --version, help and completion never create a database file.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if skipsDatabase(cmd) {
				return nil
			}
			if err := db.open(); err != nil {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to initialize database: %w", err)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := tuiOptions{vimEnabled: vim}
			initialModel := tea.Model(NewFormModelWithOptions(db, opts))
			if vim {
				initialModel = NewListModelWithOptions(db, opts)
			}
			p := tea.NewProgram(initialModel, tea.WithAltScreen(), tea.WithContext(cmd.Context()))
			_, err := p.Run()
			return err
		},
	}

	cmd.AddCommand(NewAddCmd(db))
	cmd.AddCommand(NewEditCmd(db))
	cmd.AddCommand(NewListCmd(db))
	cmd.AddCommand(DeleteTaskCmd(db))
	cmd.AddCommand(CompleteTaskCmd(db))
	cmd.AddCommand(NewExportCmd(db))
	cmd.AddCommand(NewImportCmd(db))
	cmd.Flags().BoolVar(&vim, "vim", false, "enable vim keybindings in the TUI")

	return cmd
}

// skipsDatabase reports whether cmd is one of cobra's built-in help or shell
// completion commands, which never touch task data.
func skipsDatabase(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		switch name := c.Name(); {
		case name == "help", name == "completion", strings.HasPrefix(name, "__complete"):
			return true
		}
	}
	return false
}

// -------------------------------------- export ------------------------------------------------------------------------------------ //

func NewExportCmd(db *Database) *cobra.Command {
	opts := exportOpts{}

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export tasks to JSON",
		Long: "Export tasks to a versioned JSON file for backup/migration.\n" +
			"All tasks are exported, including completed ones; use --pending-only to skip completed tasks.",
		Example: `	munus export -f tasks.json
	munus export --stdout > tasks.json
	munus export --pending-only
	munus export --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			// Resolve default filename if not stdout
			if !opts.Stdout && opts.File == "" {
				opts.File = fmt.Sprintf("munus-export-%s.json", time.Now().Format("20060102"))
			}

			filter := ExportFilter{IncludeCompleted: !opts.PendingOnly}
			svc := &TaskServiceAdapter{storage: db}

			plan, err := PlanExport(ctx, svc, filter)
			if err != nil {
				return err
			}

			if opts.DryRun {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"Would export %d tasks (todo:%d doing:%d done:%d)\n",
					plan.Total, plan.Todo, plan.Doing, plan.Done)
				return nil
			}

			if opts.Stdout {
				payload, err := ExportToBytes(ctx, svc, filter, opts.Pretty)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(payload)
				return err
			}

			if err := ExportToFile(ctx, svc, filter, opts.File, opts.Pretty); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Exported %d tasks to %s (v%d)\n", plan.Total, opts.File, exportSchemaVersion)
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.File, "file", "f", "", "Output JSON file path")
	cmd.Flags().BoolVar(&opts.Pretty, "pretty", true, "Pretty-print JSON output")
	cmd.Flags().BoolVar(&opts.Stdout, "stdout", false, "Write JSON to stdout")
	cmd.Flags().BoolVar(&opts.PendingOnly, "pending-only", false, "Skip completed tasks")
	cmd.Flags().BoolVarP(&opts.IncludeCompleted, "include-completed", "i", false, "Include completed tasks (default; kept for compatibility)")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show what would be exported without writing")
	// -i/--include-completed still parses so existing scripts keep working, but
	// completed tasks are now exported by default, so it no longer changes anything.
	// MarkDeprecated only fails for an unknown flag name, and the flag is defined above.
	_ = cmd.Flags().MarkDeprecated("include-completed", "completed tasks are exported by default")
	cmd.MarkFlagsMutuallyExclusive("include-completed", "pending-only")

	return cmd
}

//---------------------------------------------import---------------------------------------------------------------------------//

func NewImportCmd(db *Database) *cobra.Command {
	opts := importOpts{}

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import tasks from JSON",
		Long:  "Import tasks from a versioned JSON export (schema version 1 or 2).",
		Example: `	munus import -f tasks.json
	munus import -f tasks.json --skip-existing
	munus import -f tasks.json --mode replace --yes --backup
	munus import -f tasks.json --dry-run --strict
	munus import -f tasks.json --mode merge --on-conflict rename --id-strategy regenerate
	munus export --stdout | munus import -f - --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if opts.File == "" {
				return errors.New("required flag: --file")
			}
			if opts.SkipExisting && cmd.Flags().Changed("on-conflict") {
				return errors.New("--skip-existing cannot be combined with --on-conflict")
			}
			// Standard input carries the import data, so it cannot also answer
			// the replace confirmation prompt.
			if opts.File == stdinImportPath && opts.Mode == "replace" && !opts.Yes && !opts.DryRun {
				return errors.New("--mode replace with --file - requires --yes")
			}

			svc := &TaskServiceAdapter{storage: db}

			cfg := ImportConfig{
				Mode:         opts.Mode,
				OnConflict:   opts.OnConflict,
				SkipExisting: opts.SkipExisting,
				IDStrategy:   opts.IDStrategy,
				Strict:       opts.Strict,
				DryRun:       opts.DryRun,
				Backup:       opts.Backup,
			}
			cfg, err := normalizeImportConfig(cfg)
			if err != nil {
				return err
			}
			conflictPolicy := effectiveOnConflict(cfg)

			// Read the input once so standard input can be planned and applied.
			data, err := readImportSource(opts.File, cmd.InOrStdin())
			if err != nil {
				return err
			}

			plan, err := planImportData(ctx, svc, data, cfg)
			if err != nil {
				return err
			}

			source := opts.File
			if source == stdinImportPath {
				source = "standard input"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Importing: %s\nSchema: v%d\nIncoming tasks: %d\nMode: %s (conflict=%s, ids=%s)\n\n",
				source, plan.SchemaVersion, plan.Incoming, cfg.Mode, conflictPolicy, cfg.IDStrategy)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Plan:\n  Create: %d\n  Update: %d\n  Unchanged: %d\n  Conflicts: %d\n\n",
				plan.ToCreate, plan.ToUpdate, plan.Unchanged, plan.Conflicts)
			if len(plan.ConflictIDs) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Conflicting task IDs: %s\n\n", formatTaskIDs(plan.ConflictIDs))
			}

			if opts.DryRun {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Dry-run only. No changes applied.")
				return nil
			}

			if cfg.Mode == "replace" && !opts.Yes {
				ok, err := Confirm(cmd, fmt.Sprintf(
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

			res, err := applyImportData(ctx, svc, data, cfg)
			if err != nil {
				return err
			}

			if res.BackupPath != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✓ Backup created: %s\n", res.BackupPath)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"✓ Import complete: created=%d updated=%d unchanged=%d skipped=%d conflicted=%d\n",
				res.Created, res.Updated, res.Unchanged, res.Skipped, res.Conflicted)
			if len(res.SkippedIDs) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Skipped existing task IDs: %s\n", formatTaskIDs(res.SkippedIDs))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.File, "file", "f", "", "Input JSON file path ('-' reads standard input)")
	cmd.Flags().StringVar(&opts.Mode, "mode", "merge", "Import mode: merge|replace")
	cmd.Flags().StringVar(&opts.OnConflict, "on-conflict", "overwrite", "Conflict policy: skip|overwrite|rename")
	cmd.Flags().BoolVar(&opts.SkipExisting, "skip-existing", false, "Keep existing tasks when imported task IDs collide")
	cmd.Flags().StringVar(&opts.IDStrategy, "id-strategy", "preserve", "ID policy: preserve|regenerate")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Validate and show plan without applying")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&opts.Strict, "strict", false, "Fail on unknown fields, unknown statuses, status/completed contradictions and control characters")
	cmd.Flags().BoolVar(&opts.Backup, "backup", false, "Create backup before applying changes")

	return cmd
}

// Confirm prints prompt and reports whether the whole answer line is "y" or
// "yes" (any case). End of input counts as "no".
func Confirm(cmd *cobra.Command, prompt string) (bool, error) {
	_, _ = fmt.Fprint(cmd.OutOrStdout(), prompt)
	r := bufio.NewReader(cmd.InOrStdin())
	s, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.TrimSpace(s)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

//------------------------------------------------------------add / list tasks----------------------------------------------------------------//

// NewAddCmd adding tasks
func NewAddCmd(db *Database) *cobra.Command {
	var (
		title       string
		description string
		deadline    string
		tags        []string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new task",
		Example: `	munus add -t "Meeting" -d "Team sync" -n "2025-11-20 14:00" --tag work

	Deadline formats:
	- Absolute: YYYY-MM-DD HH:MM (e.g., 2025-11-16 14:30)
	- Relative units:
		• m: minutes (30m = 30 minutes from now)
		• h: hours (2h = 2 hours from now)
		• d: days (1d = 1 day from now)
		• w: weeks (2w = 2 weeks from now)
		• M: months (1M = 1 month from now)
	- Combinations: 2d 3h 30m (2days, 3hours, 30 minutes from now)`,

		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" || description == "" {
				return fmt.Errorf("both title and description are required")
			}
			if err := validateTaskText(title, description); err != nil {
				return err
			}
			normalized, err := normalizeTags(tags)
			if err != nil {
				return err
			}

			var deadlineTime *time.Time
			if deadline != "" {
				parsed, err := ParseDeadline(deadline)
				if err != nil {
					return fmt.Errorf("invalid deadline format %q: %w", deadline, err)
				}
				deadlineTime = parsed
			}

			task := &ItemModel{
				Title:       title,
				Description: description,
				Deadline:    deadlineTime,
				Status:      StatusTodo,
				Tags:        normalized,
			}

			if err := db.CreateTask(cmd.Context(), task); err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "✔ Task created successfully!")
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Title of the task")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description of the task")
	cmd.Flags().StringVarP(&deadline, "deadline", "n", "", "Deadline for the task (formats: YYYY-MM-DD HH:MM | 2d 3h 30m | 1d)")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "Tag for the task (repeatable or comma-separated)")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("description")

	return cmd
}

// NewEditCmd changes fields of an existing task.
func NewEditCmd(db *Database) *cobra.Command {
	var (
		title         string
		description   string
		deadline      string
		clearDeadline bool
		status        string
		addTags       []string
		removeTags    []string
	)

	cmd := &cobra.Command{
		Use:   "edit [task-id]",
		Short: "Edit a task",
		Long:  "Change the title, description, deadline, status or tags of a task. Only the given fields change.",
		Example: `	munus edit 12 --title "New title"
	munus edit 12 --deadline 2d --status doing
	munus edit 12 --clear-deadline --tag work --untag home`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID, err := strconv.Atoi(args[0])
			if err != nil || taskID <= 0 {
				return fmt.Errorf("invalid task ID %q: must be a positive integer", args[0])
			}
			flags := cmd.Flags()
			changed := false
			for _, name := range []string{"title", "description", "deadline", "clear-deadline", "status", "tag", "untag"} {
				changed = changed || flags.Changed(name)
			}
			if !changed {
				return errors.New("nothing to change: pass --title, --description, --deadline, --clear-deadline, --status, --tag or --untag")
			}

			ctx := cmd.Context()
			task, err := db.GetTaskByID(ctx, taskID)
			if err != nil {
				return fmt.Errorf("failed to load task %d: %w", taskID, err)
			}

			if err := applyTaskEdits(task, taskEdits{
				title:         optional(flags.Changed("title"), title),
				description:   optional(flags.Changed("description"), description),
				deadline:      optional(flags.Changed("deadline"), deadline),
				clearDeadline: clearDeadline,
				status:        optional(flags.Changed("status"), status),
				addTags:       addTags,
				removeTags:    removeTags,
			}, time.Now()); err != nil {
				return err
			}

			if err := db.UpdateTask(ctx, task); err != nil {
				return fmt.Errorf("failed to update task %d: %w", taskID, err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✔ Task %d updated.\n", taskID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "New title")
	cmd.Flags().StringVarP(&description, "description", "d", "", "New description")
	cmd.Flags().StringVarP(&deadline, "deadline", "n", "", "New deadline (formats: YYYY-MM-DD HH:MM | 2d 3h 30m | 1d)")
	cmd.Flags().BoolVar(&clearDeadline, "clear-deadline", false, "Remove the deadline")
	cmd.Flags().StringVarP(&status, "status", "s", "", "New status: todo|doing|done")
	cmd.Flags().StringSliceVar(&addTags, "tag", nil, "Add a tag (repeatable or comma-separated)")
	cmd.Flags().StringSliceVar(&removeTags, "untag", nil, "Remove a tag (repeatable or comma-separated)")
	cmd.MarkFlagsMutuallyExclusive("deadline", "clear-deadline")

	return cmd
}

// taskEdits describes the requested changes to a task; nil fields are unchanged.
type taskEdits struct {
	title         *string
	description   *string
	deadline      *string
	clearDeadline bool
	status        *string
	addTags       []string
	removeTags    []string
}

func optional(set bool, value string) *string {
	if !set {
		return nil
	}
	return &value
}

// applyTaskEdits validates and applies edits to task, using the same rules as
// `munus add`. task is unchanged when an error is returned.
func applyTaskEdits(task *ItemModel, e taskEdits, now time.Time) error {
	next := *task
	next.Tags = slices.Clone(task.Tags)

	if e.title != nil {
		if strings.TrimSpace(*e.title) == "" {
			return errors.New("title must not be empty")
		}
		next.Title = *e.title
	}
	if e.description != nil {
		if strings.TrimSpace(*e.description) == "" {
			return errors.New("description must not be empty")
		}
		next.Description = *e.description
	}
	// Validate only the fields being changed, so tasks stored before validation
	// existed can still have their status, deadline or tags edited.
	var newTitle, newDescription string
	if e.title != nil {
		newTitle = next.Title
	}
	if e.description != nil {
		newDescription = next.Description
	}
	if err := validateTaskText(newTitle, newDescription); err != nil {
		return err
	}
	if e.deadline != nil {
		parsed, err := ParseDeadline(*e.deadline)
		if err != nil {
			return fmt.Errorf("invalid deadline format %q: %w", *e.deadline, err)
		}
		next.Deadline = parsed
	}
	if e.clearDeadline {
		next.Deadline = nil
	}
	if e.status != nil {
		status, err := parseTaskStatus(*e.status)
		if err != nil {
			return err
		}
		next.setStatus(status, now)
	}
	if len(e.addTags) > 0 || len(e.removeTags) > 0 {
		remove := make(map[string]struct{}, len(e.removeTags))
		for _, tag := range e.removeTags {
			remove[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
		}
		kept := make([]string, 0, len(next.Tags)+len(e.addTags))
		for _, tag := range append(next.Tags, e.addTags...) {
			if _, drop := remove[strings.ToLower(strings.TrimSpace(tag))]; !drop {
				kept = append(kept, tag)
			}
		}
		tags, err := normalizeTags(kept)
		if err != nil {
			return err
		}
		next.Tags = tags
	}

	*task = next
	return nil
}

func GetTaskStatus(task *ItemModel) string {
	return taskStatusLabel(task, time.Now())
}

func taskStatusLabel(task *ItemModel, now time.Time) string {
	switch {
	case task.Completed:
		return "✓ DONE"
	case isOverdueAt(task, now):
		return "⚠ OVERDUE"
	case itemStatus(task) == StatusDoing:
		return "◐ DOING"
	default:
		return "○ TODO"
	}
}

func PrintList(w io.Writer, tasks []*ItemModel) {
	for _, t := range tasks {
		_, _ = fmt.Fprintf(w, "[%s] ID: %v- %s:\n%s\n -Deadline: %v\n -Complete: %t\n -Status: %s\n", GetTaskStatus(t), t.ID,
			sanitizeForTerminal(t.Title, false), sanitizeForTerminal(t.Description, true), t.Deadline, t.Completed, itemStatus(t))
		if len(t.Tags) > 0 {
			_, _ = fmt.Fprintf(w, " -Tags: %s\n", sanitizeForTerminal(strings.Join(t.Tags, ", "), false))
		}
		_, _ = fmt.Fprintln(w)
	}
}

// NewListCmd lists tasks
func NewListCmd(db *Database) *cobra.Command {
	var (
		f      taskFilter
		status string
		tags   []string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Long:  "List tasks, optionally filtered. Filters combine: a task must match all of them.",
		Example: `	munus list
	munus list --pending
	munus list --overdue --tag work
	munus list --status doing`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if status != "" {
				s, err := parseTaskStatus(status)
				if err != nil {
					return err
				}
				f.status = s
			}
			if len(tags) > 0 {
				normalized, err := normalizeTags(tags)
				if err != nil {
					return err
				}
				f.tags = normalized
			}
			tasks, err := db.ListTasks(cmd.Context())
			if err != nil {
				return err
			}
			PrintList(cmd.OutOrStdout(), filterItems(tasks, f, time.Now()))
			return nil
		},
	}
	cmd.Flags().BoolVar(&f.pending, "pending", false, "Only tasks that are not done")
	cmd.Flags().BoolVar(&f.completed, "completed", false, "Only completed tasks")
	cmd.Flags().BoolVar(&f.overdue, "overdue", false, "Only unfinished tasks past their deadline")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Only tasks with this status: todo|doing|done")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "Only tasks with this tag (repeatable; all must match)")
	cmd.MarkFlagsMutuallyExclusive("pending", "completed")
	return cmd
}

//-------------------------------complete and delete--------------------------------------------------------//

// DeleteTaskCmd delete task
func DeleteTaskCmd(db *Database) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete [task-id]",
		Short:   "Delete a task",
		Example: `	munus delete 12`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			taskID, err := strconv.Atoi(args[0])
			if err != nil || taskID <= 0 {
				return fmt.Errorf("invalid task ID %q: must be a positive integer", args[0])
			}

			// Fail before prompting when the task does not exist.
			if _, err := db.GetTaskByID(ctx, taskID); err != nil {
				return fmt.Errorf("failed to delete task %d: %w", taskID, err)
			}

			ok, err := Confirm(cmd, fmt.Sprintf("Delete task %d? [y/N]: ", taskID))
			if err != nil {
				return err
			}
			if !ok {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Delete cancelled.")
				return nil
			}

			if err := db.DeleteTask(ctx, taskID); err != nil {
				return fmt.Errorf("failed to delete task %d: %w", taskID, err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %d deleted.\n", taskID)
			return nil
		},
	}
	return cmd
}

//-------------------------------------------------------toggle complete---------------------------------------------//

// CompleteTaskCmd mark and unmark complete
func CompleteTaskCmd(db *Database) *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:   "complete [task-id]",
		Short: "Complete task",
		Example: `	munus complete 12
	munus complete 12 --undo`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			taskID, err := strconv.Atoi(args[0])
			if err != nil || taskID <= 0 {
				return fmt.Errorf("invalid task ID %q: must be a positive integer", args[0])
			}

			task, err := db.GetTaskByID(ctx, taskID)
			if err != nil {
				return fmt.Errorf("failed to load task %d: %w", taskID, err)
			}

			// --undo only reopens completed tasks; a task that is in progress
			// keeps its status.
			if undo {
				if task.Completed {
					task.MarkIncomplete()
				}
			} else {
				task.MarkComplete()
			}

			if err := db.UpdateTask(ctx, task); err != nil {
				return fmt.Errorf("failed to update task %d: %w", taskID, err)
			}

			if undo {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %d marked incomplete.\n", taskID)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Task %d completed.\n", taskID)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&undo, "undo", "u", false, "mark incomplete")
	return cmd
}
