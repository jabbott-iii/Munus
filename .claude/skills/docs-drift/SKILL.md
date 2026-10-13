---
name: docs-drift
description: Audits Munus documentation (README, CONTRIBUTING, SECURITY, PR template, NOTICE, demo tapes, CLAUDE.md and .claude/rules) against the code, config and workflows, and reports every mismatch with a proposed fix. Use when asked to check, sync or refresh the docs, after a feature lands, or before a release.
argument-hint: "[all|readme|contributing|security|licenses|instructions]"
allowed-tools: Bash(git ls-files *) Bash(git grep *) Bash(git for-each-ref *) Bash(git log *) Bash(make build) Bash(go run ./tools/licenses -check)
---

# Documentation drift audit

Scope: `$ARGUMENTS` (default `all`). This is an audit first: read, compare, report. Edit files only
after the user approves the findings, or if they asked you to fix drift directly. Never commit.

The code is the source of truth. When doc and code disagree, the fix is usually the doc; if the
code looks wrong instead, report it as a possible bug and do not change behaviour.

## Checks

Work through each area in scope. For each item, cite the doc location and the code location.

**CLI (readme)** — Build with `make build` if the toolchain allows, then run
`MUNUS_DB_PATH="$(mktemp -d)/drift.db" ./munus <command> --help` for `add`, `edit`, `list`,
`complete`, `delete`, `export`, `import` (help never opens the database). If the build is not
possible, read the `Flags()` calls in `pkg/logic-cli.go` instead and say so. Compare every flag,
short form, default and accepted value with README "Core CLI capabilities". Flag both directions:
documented-but-missing and existing-but-undocumented.

**Limits and formats (readme, security)** — Title/description limits (`pkg/ui-form.go`), tag rules
(`pkg/ext-text.go`), import size, task count and ID cap, export schema version
(`pkg/ext-export-import.go`, `pkg/database.go`), deadline units (`pkg/ext-deadline.go`), busy
timeout (`pkg/database.go`), database locations (`pkg/database_path.go`).

**TUI (readme)** — Key handling in `pkg/ui-list.go` and `pkg/ui-form.go`, `helpText()`, and README
"Interactive TUI", for both the default and `--vim` keymaps.

**Build and tooling (readme, contributing)** — Go version (`go.mod`, workflows, `Dockerfile`),
golangci-lint version (`ci.yml`, `Makefile`, README, `CONTRIBUTING.md`, PR template), make
targets (`Makefile` help vs README "Testing and quality checks"), what CI runs (`ci.yml`, `docker.yml`,
`security.yml`) vs README and CONTRIBUTING descriptions.

**Docker and release (readme, contributing, security)** — Image paths, user, env, volume and
license location (`Dockerfile`) vs README "Docker"; release artifacts, archive names, image tags
and attestation (`cd.yml`) vs README "Install"/"Docker", CONTRIBUTING "Releases" and SECURITY
"Verifying releases". Supported-versions table in `SECURITY.md` vs the release tags
(`git for-each-ref --sort=-v:refname --format='%(refname:short)' refs/tags`; local tags may be
incomplete, so say so, and do not change the table without confirmation).

**Structure and references (all)** — README "Project structure" vs `git ls-files`. Every path a
doc or config comment mentions must exist: `git grep -nE 'intel/|AGENTS\.md'` and any other
referenced file. References to removed `intel/` files and legacy IDs (`P-…`, `SEC-…`) in *code
comments* are expected history (see `CLAUDE.md` "Legacy references"); report only those in docs
or that point readers to a procedure that has moved.

**Contributor docs (contributing)** — `CONTRIBUTING.md` and `.github/pull_request_template.md`
describe the same checks and expectations.

**Licenses (licenses)** — `go run ./tools/licenses -check` (needs the Go toolchain and modules).

**Demo tapes (readme)** — Paths and commands in `demo/*.tape` headers match the repository
layout and the current CLI.

**Instructions (instructions)** — `CLAUDE.md` and `.claude/rules/*.md`: every identifier, path,
command and version they name still exists (`git grep -nw <identifier>`), and `paths` globs match
files.

## Report

A table, most severe first:

| # | Severity | Doc location | Says | Code/config says (location) | Proposed fix |
|---|---|---|---|---|---|

Severity: **wrong** (a user following the doc fails or is misled), **missing** (behaviour exists but
is undocumented), **stale** (outdated reference or wording, harmless), **code?** (the code may be
the problem). Then list what could not be checked and why. Offer to apply the doc fixes; apply
them as minimal edits in the existing style once approved.
