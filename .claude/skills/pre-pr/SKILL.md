---
name: pre-pr
description: Runs the Munus checks that the current uncommitted changes require and writes a validation report ready for the pull request template. Use before declaring a change done, before opening a pull request, or when asked whether the work is ready.
argument-hint: "[base-ref, default HEAD]"
allowed-tools: Bash(git status *) Bash(git diff *) Bash(make check) Bash(make build) Bash(make test) Bash(make race) Bash(make lint) Bash(make fuzz *) Bash(go mod tidy -diff) Bash(go version) Bash(go env *) Bash(golangci-lint --version) Bash(gofmt -s -l *)
---

# Pre-PR validation

Compare against `$ARGUMENTS` if given, otherwise against `HEAD` (uncommitted work). Do not
commit, push or open a pull request. Do not fix problems you find unless the user asks; report
them.

## 1. Scope

Run `git status --short` and `git diff --stat <base>` (include untracked files). Classify the
changed paths:

- **go**: any `*.go`
- **untrusted-input**: `pkg/ext-deadline*.go`, `pkg/ext-text*.go`, `pkg/ext-export-import*.go`
- **modules**: `go.mod`, `go.sum`
- **docker**: `Dockerfile`, `.dockerignore`
- **workflows**: `.github/workflows/*`, `.github/dependabot.yml`
- **docs**: `*.md`, `.github/*.md`, `NOTICE`
- **instructions**: `CLAUDE.md`, `.claude/**`

## 2. Environment

Record `go version`, `go env CGO_ENABLED` and `golangci-lint --version` (or "not installed"). If
Go is older than `go.mod` requires or cgo is off, stop the Go checks, report that, and continue
with the rest.

## 3. Checks (run only what applies, in this order)

| Condition | Run |
|---|---|
| go | License header present on every changed `.go` file (line 2 is `Copyright 2026 Joseph Anthony Abbott III` in the existing block) |
| go, modules | `make check` |
| untrusted-input | `make fuzz` (use `FUZZTIME=10s` only if the user asks for a quick run, and say so) |
| modules | `go mod tidy -diff`; confirm `THIRD_PARTY_LICENSES` and the `NOTICE` module list changed if modules were added or removed (`make check` runs the licenses test) |
| docker | `hadolint Dockerfile` and `docker build .` / `docker build --target static --output type=local,dest=dist .` if available; otherwise list them as not run |
| workflows | `actionlint` if available; check every new or changed `uses:` is pinned to a full SHA with a version comment and that `permissions` did not widen |
| docs | Every command, flag, path and version mentioned in the changed lines exists in the code |
| instructions | Frontmatter parses; `paths` globs match real files; commands named exist in `Makefile` |

`make lint` prints a skip message when golangci-lint is missing; treat that as **not run**, and a
version other than v2.13.2 as a caveat.

## 4. Review the diff

Read the full diff and check it against `CLAUDE.md` and the matching `.claude/rules/` files:

- New or changed behaviour has tests; tests are deterministic.
- Stored text printed only via `sanitizeForTerminal`; validation before `openForCommand`;
  `ctx` first on storage/transfer functions; multi-step writes in one transaction.
- No exported identifier in `pkg/` renamed, removed or re-signatured without approval.
- No weakened limit, permission, prompt, scan or pin.
- Documentation map in `CLAUDE.md`: every required doc update is present, or "no doc impact" is
  justified.
- No `*.db`, coverage files, binaries, `dist/`, export JSON or secrets in the change.

## 5. Report

Write the report in this shape so it can be pasted into the PR template's Validation section.
Mark a box `[x]` only when that check actually ran and passed; leave it `[ ]` otherwise. Paste
the decisive output lines.

```markdown
## Validation

- [ ] `make check` (fmt-check, vet, test, race, lint) — <result>
- [ ] golangci-lint v2.13.2 ran as part of `make check` — <version or "not installed">
- [ ] `make fuzz` — <result or "not required">
- [ ] `make licenses` / `NOTICE` / `go mod tidy -diff` — <result or "not required">
- [ ] Manual check with `make build` — <what was exercised, or "not done">

**Tested on:** <OS> / <go version>

**Not run:** <each check and why>
**Findings:** <problems found in step 4, or "none">
**Doc impact:** <files updated, or "none" with reason>
```
