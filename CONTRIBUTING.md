# Contributing to Munus

## Workflow
- Create an issue to pitch an addition or change; pull requests with no corresponding issue will be denied.
- If the issue already exists, comment on it before making a pull request that addresses it.
- Confirmation of a pitched concept is required on the applicable issue before making a pull request that modifies the code base.

## Prerequisites
- Go 1.26 or newer (see `go.mod`).
- A C compiler (gcc or clang; on Windows, MinGW-w64). The SQLite driver (`mattn/go-sqlite3`)
  requires cgo, so builds and tests need `CGO_ENABLED=1`. The Makefile sets it for you.
- Optional: [golangci-lint](https://golangci-lint.run) v2.13.2, the version CI uses.

## Setup
```bash
git clone https://github.com/jabbott-iii/Munus.git
cd Munus
make build        # produces ./munus
make test
```

## Before opening a pull request
- The license header must be maintained on every source file.
- Format the code: `gofmt -s -w .` (or `make fmt`).
- Run the local checks: `make check` (formatting check, `go vet`, tests, race-detector tests and
  golangci-lint when installed).
- CI runs on Linux, macOS and Windows: a `go mod tidy` drift check, `go vet`, golangci-lint v2.13.2,
  tests with coverage, and a native build that is run as a smoke test. CodeQL and gosec run on every
  push, and the Docker image is built and smoke-tested on `main`.

## Coding expectations
- Follow `AGENTS.md`, `intel/golang.md` (Go rules) and `intel/maint.md` (architecture).
- Keep changes focused on the issue; avoid unrelated refactors and new dependencies.
- Add or update tests for every behaviour change; tests must be deterministic (no sleeps, no
  dependence on the machine's time zone) and use `t.TempDir()` for files.
- Pass `context.Context` to storage and import/export operations.
- Validate user input with the existing helpers (task text, tags, deadlines) and never print
  stored text without `sanitizeForTerminal`.

## Pull request expectations
- Reference the issue and describe what changed and why.
- List the checks you ran and their results.
- Update `README.md` for user-visible changes and the `intel/` documents when architecture,
  security items or plans change (`intel/history.md` is append-only).
- Do not commit local databases (`*.db`), coverage files or built binaries.

## Releases
Maintainers release by tagging: `make release VERSION=vX.Y.Z`. The CD workflow builds, smoke-tests
and publishes the binaries.
