---
paths:
  - "*.md"
  - ".github/*.md"
  - "demo/**"
---

# Documentation

Munus keeps its documentation in `README.md` (users), `CONTRIBUTING.md` (contributors),
`SECURITY.md` (policy), `.github/pull_request_template.md`, `NOTICE`/`THIRD_PARTY_LICENSES`
(licensing) and the demo tapes. There is no separate architecture or history document; durable
engineering rules live in `CLAUDE.md` and `.claude/rules/`.

## Grounding

- Every statement must be verifiable in the repository: flags and help text in
  `pkg/logic-cli.go`, keys in `pkg/ui-list.go`/`pkg/ui-form.go` and `helpText()`, limits in
  `pkg/ui-form.go`, `pkg/ext-text.go` and `pkg/ext-export-import.go`, versions in `go.mod`,
  `.github/workflows/*.yml` and `Dockerfile`, targets in `Makefile`.
- Do not document planned, partial or inferred behaviour as done. Do not invent commands, env
  vars, platforms or integrations. If something cannot be verified, leave it out and say so in
  the report; add a `TODO` only when the maintainer must supply the information.
- Never write that a command was run or passed in documentation.
- Version-specific notes (for example "Munus v3.0.0 and older", "Releases after `v3.0.1`") are
  historical facts; keep them unless they become wrong.

## README

- Keep the existing section order and heading text: Features, Use cases, Install, Build from
  source, Core CLI capabilities (one subsection per command, then Interactive TUI), Configuration,
  Database location, Docker, Testing and quality checks, Project structure. `SECURITY.md` and the
  README itself link to `#install`, `#docker`, `#build-from-source` and `#database-location`;
  renaming those headings breaks links.
- Command examples are copyable from the repository root and use real flags. Keep the existing
  style (short and long flag forms as already used).
- The two demo images at the top are GitHub-hosted uploads, not the files in `demo/`. Changing
  them means re-recording with VHS (`demo/*.tape`) and a manual upload by the maintainer; say so
  instead of editing the image links.
- No new badges, screenshots, diagrams or external links unless asked.
- Update "Project structure" when top-level files or directories change.

## CONTRIBUTING.md and the PR template

- They describe the same workflow and checks; change them together. The PR template's checklist
  mirrors CONTRIBUTING "Before opening a pull request" and "Coding expectations".
- Tool versions and commands in them must match `Makefile` and `ci.yml`.

## SECURITY.md

- Public. Never add exploit details. The supported-versions table changes when a new minor or
  major release line ships (`/release-prep`), and only with the maintainer's confirmation.
- Scope bullets track real attack surface: update them when import, export, terminal output, file
  handling, the image or the release pipeline change in a way that adds or removes surface.

## Files not to edit

- `LICENSE`, `CODE_OF_CONDUCT.md`: never.
- `THIRD_PARTY_LICENSES`: generated (`make licenses`).
- `NOTICE`: only its module list, alongside dependency changes.
- `demo/*.gif`: produced by VHS from the tapes.

## Style

Plain, precise, present tense; short sentences; backticks for commands, flags, paths and
identifiers. Prefer one accurate sentence to a vague paragraph. Never put secrets, real task data
or personal paths in examples.
