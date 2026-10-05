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

package pkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database owns the gorm connection for internal data access.
type Database struct {
	conn  *gorm.DB
	sqlDB *sql.DB
	path  string
	file  string // file sqlite actually opened ("" for in-memory databases)

	// Set only for a deferred database that is not open yet (see
	// NewDeferredDatabaseAt); opening replaces the whole struct.
	dir         string // created owner-only before the first open
	notice      string // written once by openForCommand
	locationErr error  // returned by open instead of opening anything
}

// DatabaseLocation describes where a deferred Database lives and what to do
// before it is first opened.
type DatabaseLocation struct {
	// Path is the database file path or sqlite DSN.
	Path string
	// Dir, when set, is created owner-only (0700) before the database is first
	// opened; it is the directory that holds Path.
	Dir string
	// Notice, when set, is written to the command's error output when a
	// command opens the database.
	Notice string
	// Err, when set, is returned when a command opens the database (for
	// example when no default location could be determined), so that help and
	// version output still work.
	Err error
}

var (
	ErrTaskNotFound    = errors.New("task not found")
	errDatabaseNotOpen = errors.New("database is not open")
)

//-----------------------------------------------------------------------------------Models-------------------------------------------------------------------//

// TaskStatus is the workflow state of a task.
type TaskStatus string

// Task statuses. StatusDone is the only status for which ItemModel.Completed is true.
const (
	StatusTodo  TaskStatus = "todo"
	StatusDoing TaskStatus = "doing"
	StatusDone  TaskStatus = "done"
)

// parseTaskStatus converts user input (any case) into a TaskStatus.
func parseTaskStatus(s string) (TaskStatus, error) {
	switch TaskStatus(strings.ToLower(strings.TrimSpace(s))) {
	case StatusTodo:
		return StatusTodo, nil
	case StatusDoing:
		return StatusDoing, nil
	case StatusDone:
		return StatusDone, nil
	}
	return "", fmt.Errorf("invalid status %q (use todo, doing or done)", s)
}

// ItemModel Represents an item
type ItemModel struct {
	ID          int        `gorm:"primaryKey"`
	Title       string     `gorm:"size:255;not null"`
	Description string     `gorm:"type:text"`
	Deadline    *time.Time `gorm:"column:deadline"`
	Status      TaskStatus `gorm:"size:16;not null;default:'todo'"`
	Completed   bool       `gorm:"default:false;not null"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	Tags        []string   `gorm:"-"` // stored in the tags/task_tags tables
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

// BeforeSave keeps Status, Completed and CompletedAt consistent and normalises
// tags on every write. Completed decides done versus not done when the two
// disagree, so code that only sets Completed (as before statuses existed)
// keeps working; setStatus changes both together.
func (t *ItemModel) BeforeSave(*gorm.DB) error {
	status := StatusTodo
	if t.Status != "" {
		parsed, err := parseTaskStatus(string(t.Status))
		if err != nil {
			return err
		}
		status = parsed
	}
	switch {
	case t.Completed:
		status = StatusDone
	case status == StatusDone:
		status = StatusTodo
	}
	t.Status = status
	switch {
	case !t.Completed:
		t.CompletedAt = nil
	case t.CompletedAt == nil:
		now := time.Now()
		t.CompletedAt = &now
	}
	tags, err := normalizeTags(t.Tags)
	if err != nil {
		return err
	}
	t.Tags = tags
	return nil
}

// tagModel is a unique tag name.
type tagModel struct {
	ID   int    `gorm:"primaryKey"`
	Name string `gorm:"size:32;not null;uniqueIndex"`
}

func (tagModel) TableName() string { return "tags" }

// taskTagModel links a task to a tag.
type taskTagModel struct {
	TaskID int `gorm:"primaryKey;autoIncrement:false"`
	TagID  int `gorm:"primaryKey;autoIncrement:false;index"`
}

func (taskTagModel) TableName() string { return "task_tags" }

// listFilter selects which tasks the TUI list shows.
type listFilter int

const (
	filterAll listFilter = iota
	filterPending
	filterDoing
	filterOverdue
	filterDone
)

// ListModel represents the list view model
type ListModel struct {
	storage          Storage
	allTasks         []*ItemModel // everything loaded from storage
	tasks            []*ItemModel // allTasks after the active filter
	topUpcoming      []*ItemModel
	tasksNoDeadline  []*ItemModel
	cursor           int
	expanded         map[int]bool
	currentPage      int
	showHelp         bool
	err              error
	loading          bool
	confirmingDelete bool
	deletePrimed     bool
	taskToDelete     *ItemModel
	viewportWidth    int
	viewportHeight   int
	statusMessage    string
	transfer         *transferState
	vimEnabled       bool
	notice           string // shown above the list (see tuiOptions)
	filter           listFilter
	tagFilter        string
	now              func() time.Time // clock used for deadline labels and filters
	// readImport reads an import file; it runs inside a Bubble Tea command,
	// off the event loop (tests replace it with a blocking reader).
	readImport func(path string) ([]byte, error)
}

type formInputMode int

const (
	formModeInsert formInputMode = iota
	formModeNormal
)

// FormModel represents the form input model
type FormModel struct {
	storage          Storage
	fields           []string
	currentField     formField
	cursor           int
	done             bool
	err              error
	submitted        bool
	formMode         formInputMode
	vimEnabled       bool
	notice           string // shown above the form (see tuiOptions)
	editingID        int    // 0 when creating a task, otherwise the task being edited
	originalDeadline string // deadline text shown when editing started
	viewportWidth    int
	viewportHeight   int
	listFilter       listFilter // list filters restored when returning to the list
	listTagFilter    string
	pendingStatus    string // outcome of an export or import that finished while the form was open
}

type tuiOptions struct {
	vimEnabled bool
	// notice is shown at the top of every TUI screen, because the alternate
	// screen hides what was written to the terminal before the TUI started
	// (for example the note about an old ./munus.db).
	notice string
}

// DataLoadedMsg is emitted when tasks are loaded from storage.
type DataLoadedMsg struct {
	tasks []*ItemModel
}

type ErrMsg struct {
	err error
}

// NewFormModel creates a new form model
func NewFormModel(storage Storage) *FormModel {
	return NewFormModelWithOptions(storage, tuiOptions{})
}

func NewFormModelWithOptions(storage Storage, opts tuiOptions) *FormModel {
	return &FormModel{
		storage:      storage,
		fields:       make([]string, 3),
		currentField: titleField,
		formMode:     formModeInsert,
		vimEnabled:   opts.vimEnabled,
		notice:       opts.notice,
	}
}

// NewListModel creates a new list model
func NewListModel(storage Storage) *ListModel {
	return NewListModelWithOptions(storage, tuiOptions{})
}

func NewListModelWithOptions(storage Storage, opts tuiOptions) *ListModel {
	m := &ListModel{
		storage:          storage,
		expanded:         make(map[int]bool),
		loading:          true,
		confirmingDelete: false,
		taskToDelete:     nil,
		vimEnabled:       opts.vimEnabled,
		notice:           opts.notice,
		now:              time.Now,
		readImport:       readImportFile,
	}
	return m
}

//-------------------------------------------------------------------------------export/import------------------------------------------------------//

// exportSchemaVersion is written by exports and backups; imports also accept
// version 1 files (no status or tags).
const exportSchemaVersion = 2

type ExportBundle struct {
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exported_at"`
	Tasks      []TaskDTO `json:"tasks"`
}

// TaskDTO is the export/import wire format. CompletedAt, Status and Tags are
// optional so version 1 files and files written before completed_at existed
// still import.
type TaskDTO struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Completed   bool       `json:"completed"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Status      TaskStatus `json:"status,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Task struct {
	ID          string
	Title       string
	Description string
	Completed   bool
	Deadline    *time.Time
	CompletedAt *time.Time
	Status      TaskStatus
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ExportFilter struct {
	IncludeCompleted bool
}

type ExportPlan struct {
	Total int
	Todo  int
	Doing int
	Done  int
}

type ImportConfig struct {
	Mode         string
	OnConflict   string
	SkipExisting bool
	IDStrategy   string
	Strict       bool
	DryRun       bool
	Backup       bool
}

type ImportPlan struct {
	SchemaVersion int
	Incoming      int
	Current       int
	ToCreate      int
	ToUpdate      int
	Unchanged     int
	Conflicts     int
	ConflictIDs   []string
	Shortened     int // tasks in the file whose over-length text was shortened (default import)
}

type ImportResult struct {
	Created     int
	Updated     int
	Unchanged   int
	Skipped     int
	Conflicted  int
	ConflictIDs []string
	SkippedIDs  []string
	BackupPath  string
	Shortened   int // tasks in the file whose over-length text was shortened (default import)
}

type exportOpts struct {
	File             string
	Pretty           bool
	Stdout           bool
	IncludeCompleted bool // deprecated: completed tasks are exported by default
	PendingOnly      bool
	Tags             []string
	Status           []string
	DryRun           bool
}

type importOpts struct {
	File         string
	Mode         string // merge|replace
	OnConflict   string // skip|overwrite|rename
	SkipExisting bool
	IDStrategy   string // preserve|regenerate
	DryRun       bool
	Yes          bool
	Strict       bool
	Backup       bool
}

type TaskServiceAdapter struct {
	storage Storage
}

type transferAction string

const (
	transferActionExport transferAction = "export"
	transferActionImport transferAction = "import"
)

type transferStage string

const (
	transferStageInput   transferStage = "input"
	transferStageConfirm transferStage = "confirm"
)

type transferState struct {
	action           transferAction
	stage            transferStage
	path             string
	cursor           int
	includeCompleted bool
	importMode       string
	skipExisting     bool
	backup           bool
	strict           bool
	plan             *ImportPlan
	data             []byte // import file contents read for the preview and applied as previewed
	operationError   error
	// A step (export, preview, import) runs as a Bubble Tea command, so a
	// slow file never freezes the interface (plan 4, P-054). While it runs,
	// working describes it and op identifies it; esc cancels it and stops
	// waiting for it.
	working string
	op      *transferOp
}

// transferOp identifies one background transfer step; results are matched by
// pointer, so a late result never matches a newer step, even one started in
// another list model.
type transferOp struct {
	cancel context.CancelFunc // cancels the step's context
}

// transferStep names the background step a transferResultMsg reports.
type transferStep int

const (
	transferStepExport transferStep = iota + 1
	transferStepPlan
	transferStepApply
)

// transferResultMsg reports the end of an import or export step that ran as
// a Bubble Tea command, off the event loop.
type transferResultMsg struct {
	op        *transferOp
	step      transferStep
	path      string
	plan      *ImportPlan  // transferStepPlan
	data      []byte       // transferStepPlan: the bytes that were previewed
	result    ImportResult // transferStepApply
	exported  int          // transferStepExport: number of tasks written
	err       error
	cancelled bool // the step failed after its context was cancelled
}

//-----------------------------------interface tasks------------------------------------------------------------------------------------------------------//

// Storage database sql interface
type Storage interface {
	CreateTask(ctx context.Context, task *ItemModel) error
	GetTaskByID(ctx context.Context, id int) (*ItemModel, error)
	ListTasks(ctx context.Context) ([]*ItemModel, error)
	UpdateTask(ctx context.Context, task *ItemModel) error
	DeleteTask(ctx context.Context, id int) error
	ReplaceAllTasks(ctx context.Context, tasks []*ItemModel) error
	// ReplaceAllTasksFunc reads the current tasks and replaces them with the
	// result of fn as one atomic operation (a single transaction for Database).
	ReplaceAllTasksFunc(ctx context.Context, fn func(current []*ItemModel) ([]*ItemModel, error)) error
	// UpdateTaskStatus sets the status of task id to next(current), where
	// current is the status stored when the change is written (read in the
	// same transaction), so the decision never rests on a stale copy. Only the
	// status (with completed, completed_at and updated_at) is written, so
	// concurrent edits to the other fields are kept. It returns the resulting
	// status, and ErrTaskNotFound when the task does not exist. next runs
	// inside the transaction and must not block.
	UpdateTaskStatus(ctx context.Context, id int, next func(current TaskStatus) TaskStatus, now time.Time) (TaskStatus, error)
}

// NewDatabase opens (or creates) the sqlite file and runs migrations.
func NewDatabase(path string) (*Database, error) {
	return openDatabase(path, io.Discard)
}

// openDatabase is NewDatabase with an explicit destination for GORM's own log
// output. Errors are returned to callers, so production code discards it
// (stdout output would corrupt `export --stdout` and the TUI).
func openDatabase(path string, logWriter io.Writer) (*Database, error) {
	if path == "" {
		path = "munus.db"
	}

	created := createPrivateDatabaseFile(path)

	conn, err := gorm.Open(sqlite.Open(withBusyTimeout(withImmediateTransactions(path))), &gorm.Config{
		Logger: logger.New(log.New(logWriter, "", 0), logger.Config{LogLevel: logger.Silent}),
	})
	if err != nil {
		removeUnusedDatabaseFile(created)
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := migrateSchema(conn); err != nil {
		closeConnection(conn)
		return nil, err
	}

	sqlDB, err := conn.DB()
	if err != nil {
		return nil, fmt.Errorf("access sql database: %w", err)
	}

	// Record the file sqlite really opened, which differs from path for URI
	// or parameterised DSNs (e.g. "file:x.db" or "x.db?_busy_timeout=5000").
	// An in-memory database has no file, so a lookup failure leaves it empty.
	var file string
	if err := sqlDB.QueryRow("SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&file); err != nil {
		file = ""
	}
	return &Database{conn: conn, sqlDB: sqlDB, path: path, file: file}, nil
}

// withImmediateTransactions makes go-sqlite3 start every transaction with
// BEGIN IMMEDIATE (the _txlock DSN parameter), so a transaction takes the
// write lock when it begins and concurrent writers wait (busy timeout) instead
// of failing when a read-then-write transaction tries to upgrade its lock. A
// DSN that already sets _txlock is kept; a DSN starting with '?' is left alone
// because go-sqlite3 reads no parameters from it.
func withImmediateTransactions(dsn string) string {
	pos := strings.IndexByte(dsn, '?')
	switch {
	case pos == 0:
		return dsn
	case pos < 0:
		return dsn + "?_txlock=immediate"
	case strings.Contains(dsn[pos+1:], "_txlock="):
		return dsn
	default:
		return dsn + "&_txlock=immediate"
	}
}

// defaultBusyTimeout is how long a connection waits for another process's
// lock before failing with "database is locked" (plan 4, D-14); go-sqlite3's
// own default is 5 s, which heavy contention between Munus processes can
// exceed because SQLite does not queue waiting writers fairly.
const defaultBusyTimeout = 15 * time.Second

// withBusyTimeout sets go-sqlite3's busy timeout to defaultBusyTimeout unless
// the DSN sets one itself (_busy_timeout or its alias _timeout), so a user's
// value always wins. Like withImmediateTransactions it leaves a DSN starting
// with '?' alone, as well as a DSN whose parameters cannot be parsed, which
// go-sqlite3 then reports itself.
func withBusyTimeout(dsn string) string {
	param := "_busy_timeout=" + strconv.FormatInt(defaultBusyTimeout.Milliseconds(), 10)
	pos := strings.IndexByte(dsn, '?')
	switch {
	case pos == 0:
		return dsn
	case pos < 0:
		return dsn + "?" + param
	}
	params, err := url.ParseQuery(dsn[pos+1:])
	if err != nil {
		return dsn
	}
	if _, ok := params["_busy_timeout"]; ok {
		return dsn
	}
	if _, ok := params["_timeout"]; ok {
		return dsn
	}
	return dsn + "&" + param
}

// migrateSchema brings the schema and data up to date. The first attempt
// runs without a transaction, so an up-to-date database is only read (and can
// be opened read-only). When it fails, for example because another process
// created a table between GORM's existence check and its CREATE, the
// migration runs again inside a transaction, which (see
// withImmediateTransactions) holds the write lock from the start: concurrent
// first opens then run one after another and each re-checks the schema
// under the lock. Every step is idempotent, so a partly applied first
// attempt is completed by the second.
func migrateSchema(conn *gorm.DB) error {
	first := migrateSchemaOnce(conn)
	if first == nil {
		return nil
	}
	retry := conn.Transaction(migrateSchemaOnce)
	if retry == nil || retry.Error() == first.Error() {
		return retry
	}
	// Report both: the retry may fail for a different reason (for example a
	// lock timeout) that would otherwise hide the original cause.
	return errors.Join(retry, fmt.Errorf("first attempt: %w", first))
}

func migrateSchemaOnce(tx *gorm.DB) error {
	if err := tx.AutoMigrate(&ItemModel{}, &tagModel{}, &taskTagModel{}); err != nil {
		return fmt.Errorf("auto-migrate schema: %w", err)
	}
	if err := migrateStatusAndTags(tx); err != nil {
		return fmt.Errorf("migrate task status and tags: %w", err)
	}
	return nil
}

// closeConnection closes the connection pool of a database that could not be
// set up; the setup error is what gets reported.
func closeConnection(conn *gorm.DB) {
	if sqlDB, err := conn.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// consistencyTriggers keep status and tag links correct even when an older
// Munus binary (v2.1.1 or earlier, which knows nothing about them) writes to
// the database: its inserts and updates only touch "completed", and its
// deletes leave task_tags rows behind.
var consistencyTriggers = map[string]string{
	"munus_status_after_insert": `CREATE TRIGGER IF NOT EXISTS munus_status_after_insert AFTER INSERT ON item_models
WHEN (NEW.completed = 1) <> (NEW.status = 'done')
BEGIN UPDATE item_models SET status = CASE WHEN NEW.completed = 1 THEN 'done' ELSE 'todo' END WHERE id = NEW.id; END`,
	"munus_status_after_update": `CREATE TRIGGER IF NOT EXISTS munus_status_after_update AFTER UPDATE OF completed ON item_models
WHEN (NEW.completed = 1) <> (NEW.status = 'done')
BEGIN UPDATE item_models SET status = CASE WHEN NEW.completed = 1 THEN 'done' ELSE 'todo' END WHERE id = NEW.id; END`,
	"munus_tags_after_delete": `CREATE TRIGGER IF NOT EXISTS munus_tags_after_delete AFTER DELETE ON item_models
BEGIN DELETE FROM task_tags WHERE task_id = OLD.id; END`,
}

// statusNeedsRepair selects rows whose status is unknown (for example written
// by another tool) or contradicts completed (written by an older binary).
const statusNeedsRepair = "status IS NULL OR status NOT IN ('todo', 'doing', 'done') OR (completed = 1) <> (status = 'done')"

// migrateStatusAndTags repairs rows written before statuses existed, by an
// older binary or by another tool, and installs consistencyTriggers. It only
// writes when something needs changing, so an up-to-date database can be
// opened read-only.
func migrateStatusAndTags(conn *gorm.DB) error {
	var n int64
	if err := conn.Raw("SELECT COUNT(*) FROM item_models WHERE " + statusNeedsRepair).Scan(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		// completed decides done versus not done; a "doing" written in another
		// case or with spaces is kept as doing.
		if err := conn.Exec("UPDATE item_models SET status = CASE WHEN completed = 1 THEN 'done' WHEN lower(trim(status)) = 'doing' THEN 'doing' ELSE 'todo' END WHERE " + statusNeedsRepair).Error; err != nil {
			return err
		}
	}

	if err := conn.Raw("SELECT COUNT(*) FROM task_tags WHERE task_id NOT IN (SELECT id FROM item_models)").Scan(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		if err := conn.Exec("DELETE FROM task_tags WHERE task_id NOT IN (SELECT id FROM item_models)").Error; err != nil {
			return err
		}
		if err := pruneTags(conn); err != nil {
			return err
		}
	}

	for name, ddl := range consistencyTriggers {
		if err := conn.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name = ?", name).Scan(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			if err := conn.Exec(ddl).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// NewDeferredDatabase returns a Database for path that is opened on the first
// call to open, so commands such as --help never create a database file.
func NewDeferredDatabase(path string) *Database {
	return NewDeferredDatabaseAt(DatabaseLocation{Path: path})
}

// NewDeferredDatabaseAt is NewDeferredDatabase for a location that may also
// need its directory created, carry a notice for the user, or have failed to
// resolve. Nothing is created before the first open.
func NewDeferredDatabaseAt(loc DatabaseLocation) *Database {
	path := loc.Path
	if path == "" {
		path = "munus.db"
	}
	return &Database{path: path, dir: loc.Dir, notice: loc.Notice, locationErr: loc.Err}
}

// open opens the database if it is not open yet. It is safe to call repeatedly.
func (d *Database) open() error {
	if d == nil {
		return errors.New("database is not initialized")
	}
	if d.conn != nil {
		return nil
	}
	if d.locationErr != nil {
		return d.locationErr
	}
	if d.dir != "" {
		if err := ensurePrivateDir(d.dir); err != nil {
			return fmt.Errorf("create data directory: %w", err)
		}
	}
	opened, err := NewDatabase(d.path)
	if err != nil {
		return err
	}
	*d = *opened
	return nil
}

// takeNotice returns the notice of a database that has not been opened yet
// and clears it, so it is shown at most once.
func (d *Database) takeNotice() string {
	if d == nil || d.conn != nil {
		return ""
	}
	notice := d.notice
	d.notice = ""
	return notice
}

// ensurePrivateDir creates dir (and missing parents) owner-only and tightens
// an existing dir that others can access, like the backup directory.
func ensurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// MkdirAll fails when dir exists but is not a directory.
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		// A directory needs its execute bit to be entered, so 0700 (not 0600)
		// is its owner-only mode; gosec's G302 assumes a regular file.
		if err := os.Chmod(dir, 0o700); err != nil { // #nosec G302 -- owner-only directory mode, see above
			return fmt.Errorf("restrict directory permissions: %w", err)
		}
	}
	return nil
}

// createPrivateDatabaseFile creates a new database file readable only by the
// owner, for plain paths as well as DSNs with parameters and "file:" URIs, so
// sqlite never creates it with the process umask. It returns the path of the
// file it created ("" when it created none). Existing files are left
// untouched; any failure is ignored so that sqlite reports the underlying
// problem when it opens the DSN.
func createPrivateDatabaseFile(dsn string) string {
	path, ok := databaseFilePath(dsn)
	if !ok {
		return ""
	}
	// The path is the user's own setting (MUNUS_DB_PATH or the default in the
	// user's data directory), so there is no privilege boundary for G304 to
	// protect; O_EXCL only ever creates a new file and never opens an existing
	// one.
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- user-configured database path, see above
	if err != nil {
		return ""
	}
	// Closing an empty, just-created file cannot lose data; sqlite reopens it.
	_ = f.Close()
	return path
}

// removeUnusedDatabaseFile deletes a file created by createPrivateDatabaseFile
// when sqlite then failed to open the DSN (for example an invalid driver
// parameter), so no stray empty file is left behind. A file that has been
// written to is kept.
func removeUnusedDatabaseFile(path string) {
	if path == "" {
		return
	}
	if info, err := os.Stat(path); err == nil && info.Size() == 0 {
		// Best effort: the open error is what gets reported to the user.
		_ = os.Remove(path)
	}
}

// databaseFilePath returns the file sqlite creates for dsn on this system when
// it does not exist yet; see databaseFilePathFor.
func databaseFilePath(dsn string) (string, bool) {
	return databaseFilePathFor(dsn, runtime.GOOS)
}

// databaseFilePathFor maps dsn to the file sqlite creates for it on goos,
// following mattn/go-sqlite3 and sqlite: a plain DSN is cut at its first '?'
// unless the DSN starts with it; a "file:" URI is read with sqlite's URI rules
// (an empty or "localhost" authority, a path ending at '?' or '#', %HH escapes
// decoded, an encoded NUL ending the path or parameter, the last "mode"
// parameter winning). ok is false for in-memory databases, for URIs that
// sqlite does not create a file for (a mode other than rwc) and for URIs that
// cannot be mapped to a file safely (another authority, a vfs that is not one
// of sqlite's unix or win32 file vfs).
func databaseFilePathFor(dsn, goos string) (path string, ok bool) {
	if !strings.HasPrefix(dsn, "file:") {
		if pos := strings.IndexByte(dsn, '?'); pos >= 1 {
			dsn = dsn[:pos]
		}
		if dsn == "" || dsn == ":memory:" {
			return "", false
		}
		return dsn, true
	}

	rest := strings.TrimPrefix(dsn, "file:")
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest = rest[:i]
	}
	query := ""
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		rest, query = rest[:i], rest[i+1:]
	}
	mode, hasMode, vfs := "", false, ""
	for _, param := range strings.Split(query, "&") {
		key, value, _ := strings.Cut(param, "=")
		switch sqliteURIUnescape(key) {
		case "mode":
			mode, hasMode = sqliteURIUnescape(value), true
		case "vfs":
			vfs = sqliteURIUnescape(value)
		}
	}
	if hasMode && mode != "rwc" {
		return "", false
	}
	if vfs != "" && !strings.HasPrefix(vfs, "unix") && !strings.HasPrefix(vfs, "win32") {
		return "", false
	}
	if strings.HasPrefix(rest, "//") {
		authority, remainder, _ := strings.Cut(rest[2:], "/")
		if authority != "" && authority != "localhost" {
			return "", false
		}
		rest = "/" + remainder
	}
	path = sqliteURIUnescape(rest)
	if path == "" || path == ":memory:" {
		return "", false
	}
	// sqlite drops the slash before a Windows drive letter ("/C:/x.db").
	if goos == "windows" && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return path, true
}

// sqliteURIUnescape decodes %HH escapes the way sqlite does in URI filenames:
// a '%' not followed by two hex digits is kept as is, and an encoded NUL ends
// the text.
func sqliteURIUnescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHexDigit(s[i+1]) && isHexDigit(s[i+2]) {
			c := hexValue(s[i+1])<<4 | hexValue(s[i+2])
			if c == 0 {
				break
			}
			b.WriteByte(c)
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isHexDigit(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func hexValue(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	default:
		return c - '0'
	}
}

func (d *Database) ready() error {
	if d == nil || d.conn == nil {
		return errDatabaseNotOpen
	}
	return nil
}

// databaseFile returns the file sqlite opened, falling back to the configured
// path when it is unknown.
func (d *Database) databaseFile() string {
	if d == nil {
		return ""
	}
	if d.file != "" {
		return d.file
	}
	if d.path == ":memory:" || strings.HasPrefix(d.path, "file:") || strings.Contains(d.path, "?") {
		return ""
	}
	return d.path
}

func (d *Database) Close() error {
	if d == nil {
		return nil
	}
	if d.sqlDB != nil {
		return d.sqlDB.Close()
	}
	return nil
}

// Conn exposes the raw gorm handle for advanced queries/transactions.
func (d *Database) Conn() *gorm.DB {
	return d.conn
}

// CreateTask persists a new task and its tags.
func (d *Database) CreateTask(ctx context.Context, task *ItemModel) error {
	if task == nil {
		return errors.New("task is nil")
	}
	if err := d.ready(); err != nil {
		return err
	}
	return d.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		return setTaskTags(tx, task.ID, task.Tags)
	})
}

// ListTasks returns all tasks with their tags, newest first.
func (d *Database) ListTasks(ctx context.Context) ([]*ItemModel, error) {
	if err := d.ready(); err != nil {
		return nil, err
	}
	return listTasks(d.conn.WithContext(ctx))
}

func listTasks(tx *gorm.DB) ([]*ItemModel, error) {
	var tasks []*ItemModel
	if err := tx.Order("id DESC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	if err := loadTags(tx, tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// ReplaceAllTasks replaces every stored task with tasks in one transaction.
func (d *Database) ReplaceAllTasks(ctx context.Context, tasks []*ItemModel) error {
	if d == nil || d.conn == nil {
		return errors.New("database is not initialized")
	}
	return d.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return replaceAllTasks(tx, tasks)
	})
}

// ReplaceAllTasksFunc reads the current tasks and replaces them with the
// result of fn inside one transaction, so no write can slip in between.
func (d *Database) ReplaceAllTasksFunc(ctx context.Context, fn func(current []*ItemModel) ([]*ItemModel, error)) error {
	if d == nil || d.conn == nil {
		return errors.New("database is not initialized")
	}
	return d.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := listTasks(tx)
		if err != nil {
			return err
		}
		next, err := fn(current)
		if err != nil {
			return err
		}
		return replaceAllTasks(tx, next)
	})
}

func replaceAllTasks(tx *gorm.DB, tasks []*ItemModel) error {
	all := tx.Session(&gorm.Session{AllowGlobalUpdate: true})
	if err := all.Delete(&taskTagModel{}).Error; err != nil {
		return err
	}
	if err := all.Delete(&ItemModel{}).Error; err != nil {
		return err
	}

	// Insert rows that carry an explicit ID first so auto-assigned IDs can
	// never collide with an ID that is still waiting to be restored.
	for _, explicit := range []bool{true, false} {
		for _, task := range tasks {
			if task == nil || (task.ID != 0) != explicit {
				continue
			}
			if err := tx.Create(task).Error; err != nil {
				return err
			}
			if err := setTaskTags(tx, task.ID, task.Tags); err != nil {
				return err
			}
		}
	}
	return pruneTags(tx)
}

// GetTaskByID fetches a task and its tags by primary key.
func (d *Database) GetTaskByID(ctx context.Context, id int) (*ItemModel, error) {
	if err := d.ready(); err != nil {
		return nil, err
	}
	tx := d.conn.WithContext(ctx)
	var task ItemModel
	if err := tx.First(&task, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: %d", ErrTaskNotFound, id)
		}
		return nil, err
	}
	if err := loadTags(tx, []*ItemModel{&task}); err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTask saves changes to an existing task, including its tags. It
// returns ErrTaskNotFound when the task no longer exists instead of
// re-creating it.
func (d *Database) UpdateTask(ctx context.Context, task *ItemModel) error {
	if task == nil {
		return errors.New("task is nil")
	}
	if task.ID == 0 {
		return errors.New("task id is required")
	}
	if err := d.ready(); err != nil {
		return err
	}
	return d.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&ItemModel{}).Where("id = ?", task.ID).Count(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			return fmt.Errorf("%w: %d", ErrTaskNotFound, task.ID)
		}
		if err := tx.Save(task).Error; err != nil {
			return err
		}
		if err := setTaskTags(tx, task.ID, task.Tags); err != nil {
			return err
		}
		return pruneTags(tx)
	})
}

// SetTaskStatus sets the status of task id to status; see UpdateTaskStatus.
func (d *Database) SetTaskStatus(ctx context.Context, id int, status TaskStatus, now time.Time) error {
	status, err := parseTaskStatus(string(status))
	if err != nil {
		return err
	}
	_, err = d.UpdateTaskStatus(ctx, id, func(TaskStatus) TaskStatus { return status }, now)
	return err
}

// UpdateTaskStatus implements Storage. The task is read and written in one
// transaction, which holds the write lock from its start (see
// withImmediateTransactions), so no other process can change the status in
// between. The new status keeps completed and completed_at consistent (see
// setStatus); only those columns and updated_at are written, and nothing is
// written when the status does not change.
func (d *Database) UpdateTaskStatus(ctx context.Context, id int, next func(current TaskStatus) TaskStatus, now time.Time) (TaskStatus, error) {
	if id == 0 {
		return "", errors.New("task id is required")
	}
	if next == nil {
		return "", errors.New("status function is required")
	}
	if err := d.ready(); err != nil {
		return "", err
	}
	var result TaskStatus
	err := d.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task ItemModel
		if err := tx.First(&task, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: %d", ErrTaskNotFound, id)
			}
			return err
		}
		status, err := parseTaskStatus(string(next(itemStatus(&task))))
		if err != nil {
			return err
		}
		result = status
		before := task
		task.setStatus(status, now)
		if task.Status == before.Status && task.Completed == before.Completed {
			return nil
		}
		// UpdateColumns skips the save hooks, which would otherwise re-derive
		// and rewrite the untouched fields; the values set here are complete.
		return tx.Model(&ItemModel{}).Where("id = ?", id).UpdateColumns(map[string]any{
			"status":       task.Status,
			"completed":    task.Completed,
			"completed_at": task.CompletedAt,
			"updated_at":   task.UpdatedAt,
		}).Error
	})
	if err != nil {
		return "", err
	}
	return result, nil
}

// DeleteTask deletes a task by id. It returns ErrTaskNotFound when no task
// with that id exists.
func (d *Database) DeleteTask(ctx context.Context, id int) error {
	if id == 0 {
		return errors.New("task id is required")
	}
	if err := d.ready(); err != nil {
		return err
	}
	return d.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Delete(&ItemModel{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("%w: %d", ErrTaskNotFound, id)
		}
		if err := tx.Where("task_id = ?", id).Delete(&taskTagModel{}).Error; err != nil {
			return err
		}
		return pruneTags(tx)
	})
}

// setTaskTags replaces the tag links of taskID with names (already normalised).
func setTaskTags(tx *gorm.DB, taskID int, names []string) error {
	if err := tx.Where("task_id = ?", taskID).Delete(&taskTagModel{}).Error; err != nil {
		return err
	}
	for _, name := range names {
		tag := tagModel{Name: name}
		if err := tx.Where(tagModel{Name: name}).FirstOrCreate(&tag).Error; err != nil {
			return err
		}
		if err := tx.Create(&taskTagModel{TaskID: taskID, TagID: tag.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// pruneTags removes tags that no task uses any more.
func pruneTags(tx *gorm.DB) error {
	return tx.Exec("DELETE FROM tags WHERE id NOT IN (SELECT tag_id FROM task_tags)").Error
}

// loadTags fills the Tags field of tasks, sorted by name.
func loadTags(tx *gorm.DB, tasks []*ItemModel) error {
	if len(tasks) == 0 {
		return nil
	}
	byID := make(map[int]*ItemModel, len(tasks))
	ids := make([]int, 0, len(tasks))
	for _, t := range tasks {
		t.Tags = nil
		byID[t.ID] = t
		ids = append(ids, t.ID)
	}

	var rows []struct {
		TaskID int
		Name   string
	}
	q := tx.Table("task_tags").
		Select("task_tags.task_id AS task_id, tags.name AS name").
		Joins("JOIN tags ON tags.id = task_tags.tag_id")
	if len(ids) == 1 {
		q = q.Where("task_tags.task_id = ?", ids[0])
	}
	if err := q.Scan(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		if t, ok := byID[r.TaskID]; ok {
			t.Tags = append(t.Tags, r.Name)
		}
	}
	for _, t := range tasks {
		sort.Strings(t.Tags)
	}
	return nil
}
