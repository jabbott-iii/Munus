# Active Plans & Follow-on Work

IDs reference `notes.md` (N-) and `cybersec.md` (SEC-). Per `CONTRIBUTING.md`, each change needs
an issue before a PR. Status as of 2026-09-23: implemented in the working tree, **uncommitted**.

Maintainer decisions (2026-09-23): native per-OS CGO release builds (not a pure-Go driver); keep
`./munus.db` as the default DB location; export refuses only the active DB (other files may be
overwritten); leave `CONTRIBUTING.md`/`NOTICE` untouched.

| ID | Priority | Item | Ref | Status |
|---|---|---|---|---|
| P-001 | High | Working release binaries: per-OS CGO builds, smoke runs, CI runs a built binary | N-002 | Done — awaiting first GitHub run/tag |
| P-002 | High | Fix absolute deadline layout (`15:04`) in code and test | N-001 | Done |
| P-003 | High | Import integrity: stable IDs, `CompletedAt`, `--id-strategy`, deadline compare, option validation before backup, plan == apply | N-003, N-004, N-014, N-015 | Done |
| P-004 | Medium | TUI list fixes | N-008–N-011, N-021, N-022 | Done |
| P-005 | Medium | Don't create DB for `--help`; document `MUNUS_DB_PATH` (default location kept) | N-005 | Done |
| P-006 | Medium | Import/export hardening: control chars, limits, safe temp file, refuse active DB | SEC-005–007 | Done |
| P-007 | Medium | README corrections | notes | Done |
| P-008 | Medium | CI/CD security: pin actions to SHAs, upload gosec SARIF | SEC-002, SEC-003 | Done — awaiting GitHub run |
| P-009 | Low | Deadline parser: multi-month values, bounds/overflow | N-012, N-013, SEC-008 | Done |
| P-010 | Low | CLI polish: `delete` missing IDs, consistent prompt, silent GORM logger, `--version` | N-016, N-017, N-006 | Done |
| P-011 | Low | Dockerfile: non-root, `.dockerignore`, drop `sqlite-libs`, no hard-coded `GOARCH` | SEC-004 | Done — image build/run not yet verified |
| P-012 | Low | File permissions 0600/0700; backups via `os.UserHomeDir()` with unique names | SEC-001, N-019 | Done |
| P-013 | Low | Test hygiene | notes "Info" | Done |
| P-014 | Low | Remove `cryptare-*` builds in CI; bump codecov action | notes | Done — set `CODECOV_TOKEN` secret |

## Follow-on candidates (not yet planned)
- N-018 default export omits completed tasks (consider `--include-completed` default for backups).
- N-020 overdue-by-hours shown as "Due today!"; N-023 Enter-Enter replace confirm; N-024 window size
  after switching from the form; Info items in `notes.md`.
- Restore/update `CONTRIBUTING.md` (CGO prerequisite) once the maintainer decides on its content.
- Consider pinning `apk` package versions (hadolint DL3018) and a windows/arm64 build via llvm-mingw.
