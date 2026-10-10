# History (append-only)

Do not edit or remove existing entries. Append new entries at the bottom.

This file was recreated on 2026-09-27. The previous history file (entries 2026-09-23 to 2026-09-24,
covering the intel baseline, plan 1, release `v2.1.1` and the plan 2 implementation) was removed in
`48b4da2` and is still available with `git show 48b4da2^:intel/history.md`. The first entries below are
backfilled from `git log` for the changes made after that file's last entry.

## 2026-09-24 — Plan 2 committed and pushed (backfilled from git)
- `68d60fa`: plan 2 (P-015…P-030) committed: status + tags model with migration and triggers,
  `munus edit`, list filters, export schema v2, stdin import, TUI edit/status/help, context propagation,
  `golang.md` conformance, Makefile dev targets, `CONTRIBUTING.md`, windows/arm64 release job, pinned
  Alpine packages.
- `2e79130`: Windows test failure fixed (`internal/database_test.go`); `57e42fb`: README image removed;
  `61ea890`: `NOTICE` updated with third-party license notices.
- `b2e354a`, `44c6452`: in-source gosec suppressions with documented reasons for G302 (SEC-011,
  Code Scanning alert #6) and G304 (SEC-012, alerts #4 and #5). `main` on GitHub at `44c6452`.

## 2026-09-26 — Previous history file removed (backfilled from git)
- `48b4da2` deleted `intel/history.md` (local commit, not yet pushed at the 2026-09-27 review).

## 2026-09-27 — Plan 2 and SEC-012 closed
- Maintainer marked plan 2 phases A, B and C done; `plan.md` statuses updated (P-029 deviation
  recorded as accepted; the first windows/arm64 zip for P-027 is produced by the `v2.2.0` tag).
- SEC-012 closed after the maintainer confirmed successful GitHub results.

## 2026-09-27 — Test and security review of `48b4da2`
- Validation with Go 1.26.8 and `go.sum`-verified modules: gofmt, `go vet` (linux, windows, darwin),
  `go mod tidy`, tests (87.7% coverage), race ×3 shuffled, six time zones, staticcheck and govulncheck
  clean; fuzzing of deadline parsing, terminal sanitising and import.
- An independent blind review plus reproduction with the HEAD binary recorded SEC-013…SEC-018 in
  `cybersec.md` and N-026…N-036 in `notes.md` (all Open). No source code changed.

## 2026-09-27 — History file recreated and plan 3 drafted
- New `intel/history.md` (this file) at the maintainer's request, resolving the `AGENTS.md` drift noted
  in `notes.md`.
- Plan 3 (P-031…P-045, proposed target `v2.2.1`) drafted in `plan.md` from the review findings, with
  eight maintainer decisions (D-1…D-8) open.

## 2026-09-27 — Plan 3 decided and phase A implemented (uncommitted)
- Maintainer decisions D-1…D-8 recorded in `plan.md`: `v2.2.0` waits for plan 3 phase A; 50,000-task
  import cap; blank titles import as `(untitled)`; existing IDs always kept and the file-ID cap lowered
  to 1,000,000,000; CI toolchain via setup-go `1.26.x` + `check-latest`; calendar `d`/`w`; bidi controls
  treated as control characters; usage only for argument/flag errors.
- Phase A (P-031…P-036) and P-043 implemented with regression tests: stable IDs above the cap,
  non-empty titles, `Storage.SetTaskStatus` for status-only changes, sanitised status and error
  output, streaming import decode with the task cap and trailing-data rejection, owner-only DSN
  database files, three fuzz targets and `make fuzz`. SEC-013…SEC-017 closed pending maintainer
  review and CI; N-026, N-028 and N-029 fixed.
- An independent review of the change set found no High issues; its findings were fixed. README,
  `CONTRIBUTING.md`, `maint.md`, `map.md`, `notes.md` and `cybersec.md` updated.

## 2026-09-27 — v2.2.0 released
- The plan 3 phase A change set was committed as `c9eeb97` (with the `make fuzz` Makefile patch
  applied) and the new history file as `ad5c231`; CI, Docker and Security workflows passed.
- Release `v2.2.0` (tag on `ad5c231`) was built, smoke-tested and published by `cd.yml`, including the
  first windows/arm64 zip (P-027). Plan 2 and plan 3 phase A are released; phases B and C target
  `v2.2.1`.

## 2026-09-27 — Plan 3 phases B and C implemented (uncommitted)
- P-037…P-042, P-044 and P-045 implemented with regression tests: `BEGIN IMMEDIATE` for every
  transaction and a locked migration retry (fixes N-027 and the newly found N-037, concurrent writers
  failing with "database is locked"); each command opens the database only after validating its
  arguments and flags (usage only for those errors); Enter keeps the form cursor at the end;
  calendar `d`/`w` with an injected clock; pinned `tzdata` in the Docker image; `export -f -` to
  standard output, `ApplyImport` dry runs, connection closed on failed setup, rune-based TUI path
  input; CI/CD on the latest Go 1.26 patch release, no release build cache, a govulncheck job, and
  new ignore rules.
- An independent review found no High or Medium issues; its findings were fixed. SEC-018 is In
  Progress until the workflows run on GitHub; N-027 and N-030…N-037 fixed (FIFO paths in the TUI
  still block the interface).

## 2026-09-27 — v2.2.1 released; plan 4 drafted
- Plan 3 phases B and C were committed as `df70f29` (workflow patch applied) and released as `v2.2.1`
  with all checks green (maintainer-confirmed). SEC-018 closed; no security item is open. Plan 3 is
  complete.
- Plan 4 (P-046…P-058) drafted from the remaining residual risks and deferrals: license texts in
  binary distributions, no statically linked glibc, native darwin/amd64 smoke runs, provenance,
  Dependabot, restoring older over-length exports, status changes on stored data, busy timeout, TUI
  file I/O off the event loop, a testable entry point and housekeeping; decisions D-9…D-19 open.


## 2026-09-27 — Plan 4 decided and phase A implemented (uncommitted)
- The plan 4 draft and the maintainer's `AGENTS.md` fix (P-058) were committed as `97cf04b`.
  Decisions D-9…D-19 recorded in `plan.md`: committed, generated license texts; static musl Linux
  builds; provenance attestation; Dependabot; shortening over-length imports; 15 s busy timeout;
  TUI file I/O off the event loop; TUI types stay in `database.go` (P-056 deferred); Ctrl+C default
  kept as final (P-057 done); `AGENTS.md` left to the maintainer; phase A ships as `v2.2.2`, phases
  B and C as `v2.3.0`.
- Phase A (P-046…P-050) implemented: `tools/licenses` and the generated `THIRD_PARTY_LICENSES`
  (checked by `go test`), `NOTICE` without module versions; Linux release binaries built static with
  musl from a new Dockerfile `static` target, with 8 MiB thread stacks (also for the image binary);
  darwin/amd64 built and smoke-run on `macos-15-intel`; license files in every archive and in the
  image; build provenance attestation in a separate least-privilege job; Dependabot configuration;
  the `grep -q` SIGPIPE flake in the smoke tests fixed (N-038).
- An independent review found no High or Medium issues; its findings were fixed. README,
  `CONTRIBUTING.md`, `maint.md`, `map.md`, `notes.md`, `plan.md` and `cybersec.md` updated. The
  workflow, `dependabot.yml` and `Makefile` changes are delivered as a patch.

## 2026-10-03 — Plan 5: default database location and readable list deadlines (uncommitted)
- A production-readiness review of `v3.0.0` (`40f26d4`) found that the default `./munus.db` made the
  task list depend on the working directory and that `munus list` printed raw Go values; the
  maintainer asked for both to be fixed (plan 5, decisions D-20…D-22).
- P-059: without `MUNUS_DB_PATH` the database lives in the per-user data directory, created
  owner-only on first use; an old `./munus.db` in the working directory produces a note on standard
  error and is never used or changed. `internal.DatabaseLocation` and `NewDeferredDatabaseAt` added
  (`NewDeferredDatabase` unchanged).
- P-060: `munus list` shows deadlines as local `YYYY-MM-DD HH:MM` with the TUI's relative label, and
  `none` without a deadline.
- An independent review found no High or Medium issues; its Low findings were fixed (the TUI shows
  the note on every screen, the PowerShell steps remove their export file, a test of the real
  platform lookup), and `writeBackup` now shares `ensurePrivateDir`.
- SEC-019 recorded (In Progress until the change is committed and CI passes on every platform).
  README, `maint.md`, `map.md`, `notes.md`, `plan.md` and `cybersec.md` updated; no workflow,
  `Makefile`, Dockerfile or dependency change. Behaviour change: next major release (D-22, maintainer
  to confirm).

## 2026-10-03 — Plan 4 phase B: data and robustness (uncommitted)
- Plan 5 was committed by the maintainer as `dc6b120` (pushed). Phase B of plan 4 was implemented on
  top of it, following the decisions recorded on 2026-09-27 (D-13, D-14, D-15).
- P-052: `Storage.UpdateTaskStatus` decides a status change on the stored status inside the writing
  transaction; TUI `c`/`s` and `complete --undo` use it (`SetTaskStatus` became a wrapper).
- P-053: the busy timeout defaults to 15 s (`withBusyTimeout`); a DSN that sets `_busy_timeout` or
  `_timeout` keeps its value.
- P-051: default import shortens over-length titles and descriptions at a character boundary and
  reports the count (`Shortened`); `--strict` rejects such files with the task index.
- P-054: TUI export, import preview and import run as Bubble Tea commands with a working state;
  esc cancels, ctrl+c cancels and quits; late results are matched by step identity.
- An independent review found no High issues; its Medium finding (step ids repeated across list
  models, late results lost in the form) and its Low findings were fixed, and `c` now keeps the
  user's intent while deciding on the stored status.
- README, `maint.md`, `map.md`, `notes.md`, `plan.md` and `cybersec.md` updated; no workflow,
  `Makefile`, Dockerfile or dependency change.

## 2026-10-03 — Documentation drift fixed; `plan.md` and `notes.md` cleared
- Commit record: plan 4 phase A (P-046…P-050) was committed in `40f26d4` and released as `v3.0.0`
  (not `v2.2.2`); plan 5 (P-059, P-060) was committed as `dc6b120` and plan 4 phase B
  (P-051…P-054) as `442e63c`, both pushed to `main` and not yet released. Earlier entries that say
  "uncommitted" describe the state when they were written.
- README: build provenance attestations are available from `v3.0.0` (it said `v2.2.2`, which was
  never released). `cybersec.md`: SEC-019 records `dc6b120` and stays In Progress until CI is
  confirmed green on Linux, macOS and Windows.
- At the maintainer's request, `intel/plan.md` and `intel/notes.md` were cleared (only their titles
  remain). Their previous content — plans 1–5, decisions D-1…D-22, notes N-001…N-041, validation
  baselines and residual risks — is in git at `442e63c`; the D-, P- and N- IDs in `maint.md`,
  `cybersec.md` and this file refer to it. Open when they were cleared: P-055 (testable entry
  point), P-056 (deferred), D-22 (release number for plan 5 and plan 4 phase B) and the residual
  risks in `notes.md`.

## 2026-10-03 — SEC-019 closed
- The maintainer confirmed that CI passed on Linux, macOS and Windows for the default database
  location change (`dc6b120`), the remaining validation for SEC-019; it is now Closed and no
  security item is open.

## 2026-10-05 — CD publishes the container image to GitHub Packages
- `cd.yml`: the `build` job's Linux rows (amd64, arm64) also build the runtime image with the
  release version and OCI labels, smoke-test it (`--version`, a database on a volume, license
  files) and upload it as an artifact. Two new tag-only jobs run after the GitHub Release: `image`
  (`packages: write` only) loads both images, pushes `<tag>-amd64` and `<tag>-arm64` to
  `ghcr.io/jabbott-iii/munus` and joins them into `<tag>` (and `latest` for `vX.Y.Z` tags);
  `image-attest` (OIDC, no write access) attests the index digest. No new actions or other
  dependencies.
- `Dockerfile`: the `builder` stage takes `VERSION` (default `dev`) and stamps `main.version`, so
  the published image reports its release; builds without the argument are unchanged.
- Validation: actionlint 1.7.12 with shellcheck 0.11.0 is clean on all workflows and hadolint
  2.15.1 on the Dockerfile; the publish script ran against a local test registry with stand-in
  images on Docker 29.8.2 with both the classic and the containerd image store (two-platform index,
  tags, digest output, rejected version strings, platform mismatch). The full image build, the
  smoke-test steps and the push to ghcr.io were not run (no registry access in the review
  environment); P-061 covers the first real run.
- README, `CONTRIBUTING.md`, `SECURITY.md`, `maint.md`, `map.md`, `cybersec.md`, `plan.md` and
  `notes.md` updated. Uncommitted.
- Independent review: no High or Medium findings. Applied: both images are checked before either
  is pushed; the `builder` stage logs `go version` (SEC-018 evidence for the image binary); README
  and `SECURITY.md` say that only the multi-platform tags are attested and that images come from
  releases after `v3.0.1`; P-061 and N-043 cover first-publish permissions and `latest` on re-runs.
  Re-tested after the fixes: actionlint and hadolint clean; the push script passes with each image
  store and when the images are saved with one store and pushed with the other.

## 2026-10-05 — Security write-ups redacted; documentation drift fixed
- At the maintainer's request, exploit payloads, crafted inputs and step-by-step reproductions were
  removed from `cybersec.md` (SEC-005, 007, 008, 009, 013, 014, 015, 016, 017). All 19 items are
  kept with their issue, impact, remediation, validation and resolution; the earlier wording stays
  in git history.
- `AGENTS.md`: the example item is now a fenced `SEC-NNN` template (it read like an open SEC-001
  for `src/auth/`); new rules keep payloads and reproduction steps out of public docs and handle a
  vulnerability in a private advisory until its fix is released. `cybersec.md` Requirements say the
  same.
- `cd.yml`: the `build` job's checkout no longer persists credentials, as the release requirement
  in `cybersec.md` already stated.
- Release record: `v3.0.1` (tag on `1274b1b`, 2026-10-03) shipped plan 5 (`dc6b120`) and plan 4
  phase B (`442e63c`), which earlier entries describe as not yet released; D-22 is settled by that
  release. SEC-001, SEC-010 and SEC-019 now name their commit and release (`v2.1.1`, `v2.2.0`,
  `v3.0.1`); SEC-005 names the renamed `validateTaskText`, SEC-009 the later 1,000,000,000 cap.
- Correction to the "CD publishes the container image" entry above: `image-attest` has
  `attestations: write` together with OIDC; it cannot write contents or packages.
- Drift fixed: title and description limits are bytes (README, `cybersec.md`); README's text rules,
  the `--file -` replace rule (`--yes` unless `--dry-run`), the CI description and an owner-only
  Docker host directory; `maint.md` (the TUI and the import/export adapter use `Storage`, CLI
  commands take `*Database`; `edit` rejects blank descriptions; the length limits are declared in
  `ui-form.go`; `make check` compared with CI; ID-reference header, also in `cybersec.md`);
  `map.md` (`CODE_OF_CONDUCT.md`, `go.mod`/`go.sum`, attestation permission); `SECURITY.md` (fixes
  are developed privately; the GHCR image as an install source); `CONTRIBUTING.md` (run `make fuzz`
  after export changes too). Uncommitted.

## 2026-10-05 — Application code moved into package `pkg`
- `3e0fecd` (maintainer) renamed the `internal/` directory to `pkg/` so other modules can import
  it; its files still declared `package internal`.
- Now every file in `pkg/` declares `package pkg`, and `database_path.go` and its tests moved from
  the root into `pkg/`. `databaseLocation` became the exported `pkg.LocateDatabase` (the name
  `DatabaseLocation` is the location type); `main.go` only wires `pkg` up. No behaviour changed.
- `TestFormReturnToListRequestsWindowSize` compares `%T` names, so it now expects
  `pkg.DataLoadedMsg`. `make fuzz` targets `./pkg` (it still pointed at `./internal`).
- Docs: `map.md` (also lists `demo/`, where `224b8c0` moved the demo GIFs), `maint.md`, README
  project structure; `cybersec.md` notes that older items name the old paths.
- Validation with Go 1.26.8 (built from source) and modules verified against `go.sum`:
  `make check` (gofmt, vet, tests, race, golangci-lint v2.13.2: 0 issues), `go vet` for windows and
  darwin (cgo off), `go mod tidy` (no drift), `make fuzz` (5 s per target), and the same 660 test
  results as `224b8c0`, all passing (the 24 moved ones in `pkg`). Binaries built before and after
  gave identical CLI output and both TUI modes started. Not run: CI, Docker build. Uncommitted.

## 2026-10-05 — Image tags follow Salus (`X.Y.Z`, `X.Y`, `latest`)
- `cd.yml` `image` job: image tags drop the release tag's `v`. Every release gets `X.Y.Z` (the
  attested multi-platform index) and `X.Y.Z-amd64`/`X.Y.Z-arm64`; stable releases also move `X.Y`
  (new) and `latest`; pre-releases move neither. Git tags, `make release VERSION=vX.Y.Z`, archive
  names and `munus --version` keep the `v`.
- The `build` job now fails a tag that is not `vMAJOR.MINOR.PATCH[-PRERELEASE]` before `package`
  and `release` run (as Salus's image job does), so no release is created for a tag whose image
  tags cannot be derived. Tags with `+` build metadata, which became `_` in image tags, are now
  rejected.
- Docs: README (Docker tag table), `CONTRIBUTING.md`, `maint.md`, `cybersec.md` requirement,
  N-043, P-061. The `v3.0.2` image keeps its `v3.0.2` tags.
- Validation: actionlint 1.7.12 with shellcheck 0.9.0 (clean before and after); the new `Set
  release metadata` and `Push image` scripts run against sample tags with a stubbed `docker`.
  Not run: a real CD run. Uncommitted.
