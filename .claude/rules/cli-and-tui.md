---
paths:
  - "main.go"
  - "pkg/logic-cli.go"
  - "pkg/logic-cli_test.go"
  - "pkg/logic-tui.go"
  - "pkg/logic-tui_test.go"
  - "pkg/ui-*.go"
---

# CLI commands and the TUI

## CLI (`pkg/logic-cli.go`)

- Each command is built by a constructor that takes `db *Database` (`New…Cmd`; `delete` and
  `complete` use the exported `DeleteTaskCmd` and `CompleteTaskCmd`) and is registered on
  `NewRootCmd`.
- Validation order: cobra `Args` validators and required/grouped flags, then value checks at the
  top of `RunE`, then `openForCommand`. `openForCommand` marks validation finished (cobra stops
  printing usage), writes the legacy-location notice once to stderr and opens the database. So an
  argument or flag error prints usage and never creates or migrates the database; later errors
  print only the error line. New commands and flags must follow the same order.
- I/O goes through `cmd.OutOrStdout()`, `cmd.ErrOrStderr()` and `cmd.InOrStdin()` so tests can
  capture it. Standard output carries only the command's result (`export --stdout` must stay valid
  JSON); notices and diagnostics go to stderr.
- Destructive actions keep their confirmations: `delete` asks `[y/N]`, `import --mode replace`
  asks unless `--yes`, and with `--file -` requires `--yes` (except `--dry-run`). Prompts use
  `Confirm` (whole line, `y`/`yes` in any case, EOF means no).
- Task IDs are positive integers; unknown IDs are runtime errors, not usage errors.
- `main.version` is stamped by `-ldflags` at build time; do not hard-code a version.
- Do not trap Ctrl+C in CLI commands.
- Keep flag help text and README "Core CLI capabilities" in step; `--help` output is part of the
  documentation.

## TUI (`pkg/ui-form.go`, `pkg/ui-list.go`, `pkg/logic-tui.go`)

- `FormModel` creates and edits tasks; `ListModel` is the list/dashboard (filters, help panel,
  import/export dialog, delete prompt). Pure logic (status transitions, deadline labels, filters)
  belongs in `logic-tui.go` and takes `now` as a parameter.
- No blocking I/O in `Update`. Export, import preview and import run as Bubble Tea commands
  started by `startTransfer`: each step gets its own cancellable context (`transferOp` keeps only
  the cancel function and identifies the step by pointer, so a late result never matches a newer
  step), touches only values captured at start, reads files through `ListModel.readImport`, and
  reports a `transferResultMsg`. While a step runs, only `esc` (cancel) and `ctrl+c` (cancel and
  quit) are handled.
- `c` and `s` change status through `UpdateTaskStatus` with `completedStatus`,
  `reopenedStatus` or `nextStatus`, deciding on the stored status, not the copy on screen.
- On `ErrTaskNotFound` reload the list instead of failing.
- Everything rendered is sanitised: task text, tags, error lines, the location notice, the path
  prompt.
- Two keymaps exist: the default and `--vim` (list opens first; forms start in insert mode). A key
  change must work in both or be explicitly mode-specific, and must be reflected in
  `helpText()` and README "Interactive TUI". A visible TUI change may need the demo GIF
  re-recorded (`demo/TUI-demo.tape`); say so rather than editing the GIF.
- Test helpers: `loadList`, `finishTransfer`, `pressAndFinish` in `ui-list_test.go`.
