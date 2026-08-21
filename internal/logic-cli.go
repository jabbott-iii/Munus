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
	"time"

	"github.com/spf13/cobra"
	tea "github.com/charmbracelet/bubbletea"
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
	return s == "y\n" || s == "Y\n" || s == "yes\n" || s == "YES\n", nil
}

//------------------------------------------------------------add / list tasks----------------------------------------------------------------//

// adding tasks
func NewAddCmd(db *Database) *cobra.Command {
	var title, description, deadline string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new task",
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
			PrintList(tasks)
			return nil
		},
	}
	return cmd
}
