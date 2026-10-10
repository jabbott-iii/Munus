# Contributing to Munus

## Workflow
- Create an issue to pitch an addition or change; pull requests with no corresponding issue will be denied.
- If the issue already exists, comment on it before making a pull request that addresses it.
- Confirmation of a pitched concept is required on the applicable issue before making a pull request that modifies the code base.
- Report security vulnerabilities privately as described in `SECURITY.md`, not in an issue or pull request.

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
  golangci-lint when installed). After changing deadline parsing, text handling, import or export, also
  run `make fuzz`.
- After adding, removing or updating a Go module, run `make licenses` (regenerates
  `THIRD_PARTY_LICENSES`; never edit it by hand) and keep the module list in `NOTICE` in step;
  `go test ./...` fails until both match the modules compiled into the release binaries.
- CI runs on Linux, macOS and Windows: a `go mod tidy` drift check, `go vet`, golangci-lint v2.13.2,
  tests with coverage, and a native build that is run as a smoke test, using the latest Go 1.26
  patch release. CodeQL, gosec and govulncheck run on every push, and the Docker image and the static
  Linux release binary (amd64 and arm64) are built and smoke-tested on `main` and for pull requests
  to `main`.
- Dependabot opens weekly, grouped update pull requests for GitHub Actions, Go modules and Docker
  images; they follow the same review and checks (Go module updates may need `make licenses`).

## Coding expectations
- Follow `AGENTS.md`, `intel/golang.md` (Go rules) and `intel/maint.md` (architecture).
- Keep changes focused on the issue; avoid unrelated refactors and new dependencies.
- Add or update tests for every behaviour change; tests must be deterministic (no sleeps, no
  dependence on the machine's time zone) and use `t.TempDir()` for files.
- Pass `context.Context` to storage and import/export operations.
- Validate user input with the existing helpers (task text, tags, deadlines) and never print
  stored text without `sanitizeForTerminal`.

## Pull request expectations
- Fill in the pull request template (`.github/pull_request_template.md`), which GitHub
  loads when you open a pull request.
- Reference the issue and describe what changed and why.
- List the checks you ran and their results.
- Update `README.md` for user-visible changes and the `intel/` documents when architecture,
  security items or plans change (`intel/history.md` is append-only).
- Do not commit local databases (`*.db`), coverage files or built binaries.

## Releases
Maintainers release by tagging: `make release VERSION=vX.Y.Z`. The CD workflow builds, smoke-tests
and publishes the binaries (each archive with `LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES`) and
attests their build provenance. It also builds and smoke-tests the container image for linux/amd64
and linux/arm64 and, after the GitHub Release, publishes it to GitHub Packages
(`ghcr.io/jabbott-iii/munus`) and attests it. Image tags drop the `v`: `X.Y.Z`, plus `X.Y` and
`latest` for stable releases. A tag that is not `vMAJOR.MINOR.PATCH[-PRERELEASE]` fails the CD run
before a release is created.
