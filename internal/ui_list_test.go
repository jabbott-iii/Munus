package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestListModelExportFlowIncludesCompletedWhenToggled(t *testing.T) {
	db := newTestDatabase(t)
	createTestTask(t, db, "open task", false)
	createTestTask(t, db, "done task", true)

	model := loadTestListModel(t, db)
	model, _ = updateListModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})

	exportPath := filepath.Join(t.TempDir(), "tasks.json")
	model.transfer.path = exportPath
	model.transfer.cursor = len(exportPath)

	model, _ = updateListModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	model, _ = updateListModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if model.transfer != nil {
		t.Fatalf("expected export flow to close after export")
	}
	if !strings.Contains(model.statusMessage, "Exported 2 tasks") {
		t.Fatalf("expected export success message, got %q", model.statusMessage)
	}

	payload, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}

	var bundle ExportBundle
	if err := json.Unmarshal(payload, &bundle); err != nil {
		t.Fatalf("unmarshal export file: %v", err)
	}
	if got := len(bundle.Tasks); got != 2 {
		t.Fatalf("expected 2 exported tasks, got %d", got)
	}
}

func TestListModelImportReplaceRequiresConfirmation(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	db := newTestDatabase(t)
	createTestTask(t, db, "existing task", false)

	importPath := filepath.Join(t.TempDir(), "import.json")
	writeImportBundle(t, importPath, []TaskDTO{{
		ID:          "task-1",
		Title:       "imported task",
		Description: "from file",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}})

	model := loadTestListModel(t, db)
	model, _ = updateListModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})

	model.transfer.path = importPath
	model.transfer.cursor = len(importPath)

	model, _ = updateListModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	if model.transfer.importMode != "replace" {
		t.Fatalf("expected replace mode, got %q", model.transfer.importMode)
	}

	model, _ = updateListModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.transfer == nil || model.transfer.stage != transferStageConfirm {
		t.Fatalf("expected import flow confirmation stage")
	}

	tasksBeforeConfirm, err := db.ListTasks()
	if err != nil {
		t.Fatalf("list tasks before confirmation: %v", err)
	}
	if got := tasksBeforeConfirm[0].Title; got != "existing task" {
		t.Fatalf("expected existing task before confirmation, got %q", got)
	}

	model, cmd := updateListModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd != nil {
		msg := cmd()
		model, _ = updateListModel(t, model, msg)
	}

	if model.transfer != nil {
		t.Fatalf("expected import flow to close after confirmation")
	}
	if !strings.Contains(model.statusMessage, "Import complete") {
		t.Fatalf("expected import success message, got %q", model.statusMessage)
	}
	if !strings.Contains(model.statusMessage, "backup=") {
		t.Fatalf("expected backup path in success message, got %q", model.statusMessage)
	}

	tasksAfterConfirm, err := db.ListTasks()
	if err != nil {
		t.Fatalf("list tasks after confirmation: %v", err)
	}
	if len(tasksAfterConfirm) != 1 || tasksAfterConfirm[0].Title != "imported task" {
		t.Fatalf("expected imported task after confirmation, got %+v", tasksAfterConfirm)
	}

	backupFiles, err := filepath.Glob(filepath.Join(homeDir, ".munus", "backups", "*.json"))
	if err != nil {
		t.Fatalf("glob backup files: %v", err)
	}
	if len(backupFiles) == 0 {
		t.Fatalf("expected backup file to be created")
	}
}

func loadTestListModel(t *testing.T, db *Database) *ListModel {
	t.Helper()

	model := NewListModel(db)
	msg := model.Init()()
	loadedModel, _ := updateListModel(t, model, msg)
	return loadedModel
}

func updateListModel(t *testing.T, model *ListModel, msg tea.Msg) (*ListModel, tea.Cmd) {
	t.Helper()

	updated, cmd := model.Update(msg)
	listModel, ok := updated.(*ListModel)
	if !ok {
		t.Fatalf("expected *ListModel, got %T", updated)
	}
	return listModel, cmd
}

func newTestDatabase(t *testing.T) *Database {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "munus.db")
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("new database: %v", err)
	}
	return db
}

func createTestTask(t *testing.T, db *Database, title string, completed bool) {
	t.Helper()

	task := &ItemModel{
		Title:       title,
		Description: title + " description",
		Completed:   completed,
	}
	if err := db.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}
}

func writeImportBundle(t *testing.T, path string, tasks []TaskDTO) {
	t.Helper()

	payload, err := json.Marshal(ExportBundle{
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Tasks:      tasks,
	})
	if err != nil {
		t.Fatalf("marshal import bundle: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write import bundle: %v", err)
	}
}
