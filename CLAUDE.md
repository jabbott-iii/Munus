# Munus — Claude Code instructions

Munus is a terminal task manager: a scriptable CLI (cobra) and an interactive TUI (Bubble Tea)
over a local SQLite database (GORM + `mattn/go-sqlite3`). One Go module,
`github.com/jabbott-iii/Munus`, `go 1.26.0`. All application code is package `pkg` in `pkg/`;
`main.go` only wires it up. Single maintainer (`CODEOWNERS`).

Path-scoped rules in `.claude/rules/` load when you touch matching files and hold the detailed
invariants for storage, input/transfer, CLI/TUI, build/release and docs. Read the relevant one
before changing an area even if it has not loaded yet.

## Ground rules

- Do not commit, amend, tag, push, rebase, merge or rewrite history, and do not run `make tag`,
  `make push-tag` or `make release`. Leave every change uncommitted for the maintainer to review.
- Make the smallest complete change. No unrelated refactors, reformatting, renames, dependency
  changes or new abstractions unless asked.
- Exported identifiers in `pkg/` are a public Go API: the package was moved out of `internal/` so
  other modules can import it (commits `3e0fecd`, `a5d8bcf`). Do not rename, remove or change the
  signature of an exported identifier without explicit approval.
- Breaking changes need explicit approval: CLI commands or flags, TUI keys, output relied on by
  scripts, the export format, the database schema, file locations.
- Do not weaken input validation, text sanitising, import limits, file permissions, confirmation
  prompts, CI security scans or SHA pins to make something pass. If a request conflicts with
  these, say so and propose a compliant alternative.
- Never put vulnerability details (payloads, crafted inputs, reproduction steps) in public files,
  commit messages, issues or PR text. Security reports follow `SECURITY.md` (private advisory).
- Do not invent commands, flags, env vars or features. Verify against the code before writing
  about it. Never claim a check ran or passed unless you ran it and saw the result.

## Build and verify

The SQLite driver needs cgo and a C compiler. The Makefile exports `CGO_ENABLED=1`; plain `go`
uses cgo by default when a C compiler is on `PATH` (check `go env CGO_ENABLED`). A
`CGO_ENABLED=0` binary compiles but cannot open a database.

| Command | Purpose |
|---|---|
| `make build` | Build `./munus` |
| `make check` | `fmt-check`, `vet`, `test`, `race`, `lint` — the default gate for any Go change |
| `make test` / `make race` / `make cover` | Tests, race detector, coverage summary |
| `make fmt` | `gofmt -s -w .` |
| `make lint` | golangci-lint; **skipped without failing if not installed** (CI uses v2.13.2) |
| `make fuzz` | Each fuzz target for `FUZZTIME` (default 30s); `make fuzz FUZZTIME=10s` |
| `make licenses` | Regenerate `THIRD_PARTY_LICENSES` |
| `go mod tidy -diff` | CI's tidy drift check (CI runs `go mod tidy && git diff --exit-code`); no make target |
| `go test ./pkg -run '^TestName$' -count=1` | One test |

What to run:

- Any Go change: `make check`. Report whether lint actually ran and which golangci-lint version.
- Deadline parsing, text handling, import or export (`pkg/ext-*.go`): also `make fuzz`.
- `go.mod`/`go.sum` change: `go mod tidy -diff`, `make licenses`, update the `NOTICE` module list,
  `make check` (`go test ./tools/licenses` fails until both match the compiled modules).
- `Dockerfile`: `docker build .` and `docker build --target static --output type=local,dest=dist .`
  if Docker is available; `hadolint Dockerfile` if installed.
- Workflows: `actionlint` if installed. CI behaviour itself is only proven by a GitHub run.
- If the toolchain cannot run (Go older than `go.mod`, no C compiler, no network for modules),
  say exactly what was not run and why. Never substitute a guess.

`/pre-pr` runs this selection for the current diff and writes the validation report.

## Layout

| Path | Contents |
|---|---|
| `main.go` | Resolves the DB location, creates a deferred DB, stamps `main.version`, runs the root command |
| `pkg/database.go` | `Storage` interface, GORM `Database`, migration, tags, triggers, all shared types |
| `pkg/database_path.go` | `LocateDatabase`: `MUNUS_DB_PATH` or the per-user data directory, legacy `./munus.db` notice |
| `pkg/logic-cli.go` | Cobra commands: root (TUI), `add`, `edit`, `list`, `complete`, `delete`, `export`, `import` |
| `pkg/logic-tui.go` | Status transitions, deadline labels, filters (pure helpers, clock passed in) |
| `pkg/ui-form.go`, `pkg/ui-list.go` | Bubble Tea form and list/dashboard models; text limits live in `ui-form.go` |
| `pkg/ext-text.go` | Text and tag validation, control-character handling, `sanitizeForTerminal` |
| `pkg/ext-deadline.go` | Absolute and bounded relative deadline parsing |
| `pkg/ext-export-import.go` | Versioned JSON export, import plan/apply, backups |
| `pkg/*_test.go` | Tests and fuzz targets, next to the code |
| `tools/licenses/` | Stdlib-only generator/check for `THIRD_PARTY_LICENSES` and the `NOTICE` module list |
| `demo/` | VHS tapes and the GIFs they record |
| `.github/workflows/` | `ci.yml`, `cd.yml` (release), `docker.yml`, `security.yml` |
| `Dockerfile` | Runtime image and the `static` stage that builds the Linux release binaries |

## Invariants that apply everywhere

- Commands validate arguments and flags first, then call `openForCommand`. Help, version and
  argument errors never create or migrate a database.
- Stored text reaches the terminal only through `sanitizeForTerminal`; the root command's errors
  go through `terminalSafeWriter`. Statuses shown or exported are only `todo`/`doing`/`done`.
- Storage and import/export functions take `ctx context.Context` first. No contexts in structs,
  no package-level mutable state; inject clocks, env lookups and writers.
- Every multi-step write is one storage transaction. Ctrl+C keeps Go's default behaviour; do not
  add signal handling.
- Task IDs are stable; imports never renumber existing tasks.
- Schema changes are additive and idempotent; v1 and v2 export files must keep importing.
- New database, export and backup files are `0600`; data and backup directories `0700`.
- Every `.go` file starts with the Apache-2.0 header block used in `main.go`.

## Tests

- Every behaviour change gets a test; every bug fix gets a regression test.
- Deterministic only: no sleeps, no dependence on the machine's time zone or clock (pass `now`,
  use fixed locations), `t.TempDir()` for files, `setTestHome(t)` for anything that writes
  backups, `t.Fatalf` (never `return`) on setup errors.
- Reuse existing helpers before writing new ones: `setupTestDB`, `newFileTestDB`, `runCmd`,
  `seedTask`, `setTestHome`, `loadList`, `finishTransfer`, `pressAndFinish`.
- Do not weaken or delete assertions to make tests pass.

## Documentation map

Update docs in the same change as the code. README content must be verifiable from the code.

| When you change | Update |
|---|---|
| A CLI command, flag, validation rule or output | README "Core CLI capabilities" (and examples/use cases if affected); the flag's help text |
| A TUI key or behaviour | README "Interactive TUI" and `helpText()` in `pkg/ui-list.go` |
| Env vars, DB location, file permissions | README "Configuration" / "Database location" |
| Export format or import limits/options | README export/import sections; `SECURITY.md` scope if limits change |
| Go, golangci-lint or tool versions, make targets | README "Build from source"/"Testing", `CONTRIBUTING.md`, PR template, `Makefile` help, `ci.yml` — all must agree |
| `Dockerfile` or image behaviour | README "Docker" |
| Release pipeline (`cd.yml`) | README "Install"/"Docker", `CONTRIBUTING.md` "Releases", `SECURITY.md` "Verifying releases" |
| Go modules | `make licenses` (never hand-edit `THIRD_PARTY_LICENSES`) and the `NOTICE` module list |
| Contributor workflow or required checks | `CONTRIBUTING.md` and `.github/pull_request_template.md` together |
| Architecture, conventions or invariants | This file or the matching `.claude/rules/` file |

If a change has no doc impact, say so in the completion report. `/docs-drift` audits all of the
above against the code.

## Legacy references

Code and config comments cite IDs such as `P-052`, `D-14`, `N-026`, `SEC-013` and files under
`intel/`. Those documents (and the old `AGENTS.md`) were removed in `102b5ce`. Read them from
history when a comment's reasoning matters (needs full history, not a shallow clone):

- `SEC-*`, current architecture: `git show 102b5ce^:intel/cybersec.md`, `git show 102b5ce^:intel/maint.md`
- `P-001`–`P-060`, `D-*`, `N-001`–`N-041`: `git show 442e63c:intel/plan.md`, `git show 442e63c:intel/notes.md`
- `P-061`+, `N-042`+: `git show 102b5ce^:intel/plan.md`, `git show 102b5ce^:intel/notes.md`

Do not add new IDs of this kind, and do not edit existing ones unless asked.

## Project skills

- `/pre-pr` — run the checks the diff needs and produce the validation report.
- `/docs-drift [scope]` — audit README, CONTRIBUTING, SECURITY, PR template, NOTICE, demo tapes and
  these instructions against the code.
- `/dependency-update [kind]` — Go modules, Actions, Docker/Alpine pins, Go minor bumps.
- `/release-prep vX.Y.Z` — readiness checks and doc updates before a tag; prints the release command.

## Completion report

End every task with: summary; files changed and why; commands run with results; checks not run
and why; assumptions; doc impact (updated, or "none"); unresolved issues and residual risks.
