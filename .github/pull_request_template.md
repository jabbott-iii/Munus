<!--
Thank you for contributing to Munus. Read CONTRIBUTING.md before opening this pull request.
Pull requests without a linked issue on which a maintainer has confirmed the concept will be denied.
Replace the comments below and remove sections that do not apply.
-->

## Linked issue

Closes #

- [ ] A maintainer confirmed the concept on the linked issue.

## Summary

<!-- What changed and why. Keep the change focused on the linked issue. -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change (CLI commands or flags, TUI keys, export format, database schema, or other existing behaviour)
- [ ] Documentation
- [ ] Build, CI or dependency update
- [ ] Refactor or tests only (no behaviour change)

## Validation

<!-- Tick only what you actually ran, and paste the relevant output below. -->

- [ ] `make check` (fmt-check, vet, test, race, lint)
- [ ] golangci-lint v2.13.2 ran as part of `make check` (not skipped as "not installed")
- [ ] `make fuzz` (required after changing deadline parsing, text handling or import/export)
- [ ] `make licenses` run, `NOTICE` module list updated and `go mod tidy` leaves no diff (required after adding, removing or updating a Go module)
- [ ] Manual check with a binary built by `make build` (describe below)

**Tested on:** <!-- OS (Linux / macOS / Windows) and `go version` output -->

<details>
<summary>Command output</summary>

```text
paste output here
```

</details>

## Checklist

- [ ] Tests added or updated for every behaviour change; they are deterministic (no sleeps, no dependence on the machine's time zone) and use `t.TempDir()` for files
- [ ] The Apache-2.0 license header is kept on every source file
- [ ] Storage and import/export operations take a `context.Context`
- [ ] User input goes through the existing validation helpers (task text, tags, deadlines), and stored text is printed only through `sanitizeForTerminal`
- [ ] No new dependency, or the reason is explained in the summary
- [ ] `README.md` updated for user-visible changes
- [ ] `intel/` documents updated where architecture, security items or plans changed (`intel/history.md` is append-only)
- [ ] No local databases (`*.db`), coverage files, built binaries or secrets are included

## Security and compatibility

<!--
Security: does this touch input validation, file permissions, import limits, terminal output,
CI/CD or dependencies? Reference any affected item in intel/cybersec.md (for example SEC-0XX).
Compatibility: does this change existing CLI or TUI behaviour, the export format or the database
schema? Schema changes must stay additive, and v1/v2 export files must still import.
Write "None" if neither applies.
-->

## Screenshots or recordings

<!-- For TUI changes, add a screenshot or terminal recording. Remove this section otherwise. -->

## Notes for reviewers

<!-- Areas to focus on, known limitations or follow-up work. -->
