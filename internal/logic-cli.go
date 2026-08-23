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
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

//--------------------------------------CORE----------------------------------------------------------------------------//

// tui main entry point
func NewRootCmd(db *Database) *cobra.Command {
	cmd := &cobra.Command{
		Use: "munus",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := tea.NewProgram(NewFormModel(db), tea.WithAltScreen())
			_, err := p.Run()
			return err
		},
	}

	cmd.AddCommand(NewAddCmd(db))
	cmd.AddCommand(NewListCmd(db))
	cmd.AddCommand(DeleteTaskCmd(db))
	cmd.AddCommand(CompleteTaskCmd(db))
	cmd.AddCommand(NewExportCmd())
	cmd.AddCommand(NewImportCmd())

	return cmd
}

// -------------------------------------- export ------------------------------------------------------------------------------------ //

func NewExportCmd() *cobra.Command {
	opts := exportOpts{}

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export tasks to JSON",
		Long:  "Export tasks to a versioned JSON file for backup/migration.",
		Example: `  munus export -f tasks.json
 					munus export --stdout > tasks.json
					munus export --include-completed
					munus export --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve default filename if not stdout
			if !opts.Stdout && opts.File == "" {
				opts.File = fmt.Sprintf("munus-export-%s.json", time.Now().Format("20060102"))
			}

			// Build filter
			filter := ExportFilter{
				IncludeCompleted: opts.IncludeCompleted,
			}

			// service/repo wiring
			svc, err := buildTaskService()
			if err != nil {
				return err
			}

			plan, err := PlanExport(svc, filter)
			if err != nil {
				return err
			}

			if opts.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(),
					"Would export %d tasks (task:%d doing:%d done:%d)\n",
					plan.Total, plan.Todo, plan.Doing, plan.Done)
				return nil
			}

			if opts.Stdout {
				payload, err := ExportToBytes(svc, filter, opts.Pretty)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(payload)
				return err
			}

			if err := ExportToFile(svc, filter, opts.File, opts.Pretty); err != nil {
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
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Show what would be exported without writing")

	return cmd
}

//---------------------------------------------import---------------------------------------------------------------------------//

func NewImportCmd() *cobra.Command {
	opts := importOpts{}

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import tasks from JSON",
		Long:  "Import tasks from a versioned JSON export.",
		Example: `  munus import -f tasks.json
					munus import -f tasks.json --mode replace --yes --backup
					munus import -f tasks.json --dry-run --strict
					munus import -f tasks.json --mode merge --on-conflict rename --id-strategy ict`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.File == "" {
				return errors.New("required flag: --file")
			}

			svc, err := buildTaskService()
			if err != nil {
				return err
			}

			cfg := ImportConfig{
				Mode:       opts.Mode,
				OnConflict: opts.OnConflict,
				IDStrategy: opts.IDStrategy,
				Strict:     opts.Strict,
				DryRun:     opts.DryRun,
				Backup:     opts.Backup,
			}

			plan, err := PlanImport(svc, opts.File, cfg)
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

			res, err := ApplyImport(svc, opts.File, cfg)
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

func Confirm(cmd *cobra.Command, prompt string) (bool, error) {
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	r := bufio.NewReader(cmd.InOrStdin())
	s, err := r.ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.TrimSpace(s)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

//------------------------------------------------------------add / list tasks----------------------------------------------------------------//

// adding tasks
func NewAddCmd(db *Database) *cobra.Command {
	var title, description, deadline string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new task",
		Example: `  munus add -t "Meeting" -d "Team sync" -n "2025-11-20 14:00
					Deadline formats:
					- Absolute: YYYY-MM-DD HH:MM (e.g., 2025-11-16 14:30)
					- Relative units:
						• m: minutes (30m = 30 minutes from now)
						• h: hours (2h = 2 hours from now)
						• d: days (1d = 1 day from now)
						• w: weeks (2w = 2 weeks from now)
						• M: months (1M = 1 month from now)
					- Combinations: 2d 3h 30m (2days, 3hours, 30 minutes from now"`,
					
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" || description == "" {
				return fmt.Errorf("both title and description are required")
			}
			if len(title) > MaxTitleLength {
				return fmt.Errorf("title exceeds maximum length of %d", MaxTitleLength)
			}
			if len(description) > MaxDescriptionLength {
				return fmt.Errorf("description exceeds maximum length of %d", MaxDescriptionLength)
			}

			var deadlineTime *time.Time
			if deadline != "" {
				parsed, err := ParseDeadline(deadline)
				if err != nil {
					return err
				}
				deadlineTime = parsed
			}

			task := &ItemModel{
				Title:       title,
				Description: description,
				Deadline:    deadlineTime,
				Completed:   false,
			}

			if err := db.CreateTask(task); err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "✔ Task created successfully!")
			return nil
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Title of the task")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description of the task")
	cmd.Flags().StringVarP(&deadline, "deadline", "n", "", "Deadline for the task")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("description")

	return cmd
}

func PrintList(w io.Writer, tasks []*ItemModel) {
	for _, t := range tasks {
		fmt.Fprintf(w, " ID: %v- %s:\n%s\n -Deadline: %t\n -Complete: %t\n", t.ID, t.Title, t.Description, t.Deadline, t.Completed)
	}
}

// lists tasks
func NewListCmd(db *Database) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			tasks, err := db.ListTasks()
			if err != nil {
				return err
			}
			PrintList(cmd.OutOrStdout(), tasks)
			return nil
		},
	}
	return cmd
}

//-------------------------------complete and delete--------------------------------------------------------//

//delete task
func DeleteTaskCmd(db *Database) *cobra.Command {
	cmd := &cobra.Command{
		Use:	"delete [task-id]",
		Short:	"Delete a task",
		Example: `munus delete 12`,
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID, err := strconv.Atoi(args[0])
			if err != nil || taskID <= 0 {
				return fmt.Errorf("invalid task ID %q: must be a positive integer", args[0])
			}
			
			// confirmation
			var confirm string
			fmt.Fprintf(cmd.OutOrStdout(), "Delete task %d? [y/N]: ", taskID)
			_, _ = fmt.Fscanln(cmd.InOrStdin(), &confirm)
			if confirm != "y" && confirm != "Y" && confirm != "yes" && confirm != "YES" {
				fmt.Fprintln(cmd.OutOrStdout(), "Delete cancelled.")
				return nil
			}

			// database check
			if err := db.DeleteTask(taskID); err != nil {
				return fmt.Errorf("failed to delete task %d: %w", taskID, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Task %d deleted.\n", taskID)
			return nil
		},
	}
	return cmd
}

//-------------------------------------------------------toggle complete---------------------------------------------//

//mark and unmark complete
func CompleteTaskCmd(t *ItemModel) *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:	"complete [task-id]",
		Short:	"Complete task",
		Example: `munus complete 12,
				  munus complete 12 --undo`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID, err := strconv.Atoi(args[0])
			if err != nil || taskID <= 0 {
				return fmt.Errorf("invalid task ID %q: must be a positive integer", args[0])
			}

			// 1) Load task from DB by ID
			task, err := db.GetTaskByID(taskID)
			if err != nil {
				return fmt.Errorf("failed to load task %d: %w", taskID, err)
			}
			if task == nil {
				return fmt.Errorf("task %d not found", taskID)
			}

			// 2) Update fields in memory
			now := time.Now()
			if undo {
				task.Completed = false
				task.CompletedAt = nil
			} else {
				task.Completed = true
				task.CompletedAt = &now
			}
			task.UpdatedAt = now

			// 3) Persist update
			if err := db.UpdateTask(task); err != nil {
				return fmt.Errorf("failed to update task %d: %w", taskID, err)
			}

			if undo {
				fmt.Fprintf(cmd.OutOrStdout(), "Task %d marked incomplete.\n", taskID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Task %d completed.\n", taskID)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&undo, "undo", "u", false, "mark incomplete")
	return cmd
}
