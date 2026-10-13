---
name: dependency-update
description: Updates or reviews Munus dependencies — Go modules, GitHub Actions SHA pins, the Dockerfile's Alpine images and apk pins (including a build broken by a tzdata or other pin that Alpine dropped), and deliberate Go minor-version upgrades. Use for Dependabot pull requests, failing Docker builds caused by pins, or any request to bump a dependency.
argument-hint: "[gomod|actions|docker|go] [details]"
---

# Dependency updates

Kind: `$ARGUMENTS`. If no kind is given, work it out from the diff or the request and say which
one you picked. Never commit, push or merge; leave changes uncommitted. Run `/pre-pr` at the end.

General rules for every kind:

- Change only what the update needs. No opportunistic upgrades of other dependencies.
- Read the upstream release notes for breaking or security-relevant changes and summarise them.
- A dependency update never weakens a pin, a checksum, a permission or a scan.

## gomod — Go modules

1. Apply the update (`go get module@version`, or review the Dependabot diff).
2. `go mod tidy`, then confirm `go mod tidy -diff` is clean and `go.sum` changed only for the
   modules involved.
3. `make licenses`. If a module was added or removed, update the `NOTICE` module list in the
   existing format (module path, copyright holder, license) under the right heading, including
   "Windows builds only" where `go list` shows a Windows-only module. A module without a license
   file needs a reviewed `readmeLicenses` entry in `tools/licenses/main.go`; ask before adding one.
4. `make check`. For updates to `bubbletea`, `lipgloss` or `charmbracelet/x/*`, also exercise
   the TUI manually (`make build`, then `MUNUS_DB_PATH=$(mktemp -d)/t.db ./munus`) or list that as
   not done. For `gorm`, `gorm.io/driver/sqlite` or `go-sqlite3`, also `make race`.
5. Report any change in license (a new license type needs the maintainer's review).

## actions — GitHub Actions

1. Each `uses:` stays pinned to a full 40-character commit SHA with a `# vX.Y.Z` comment.
2. Verify the SHA belongs to that tag: `git ls-remote https://github.com/<owner>/<repo>
   refs/tags/vX.Y.Z` (for annotated tags check the peeled `^{}` line).
3. Check the action's release notes for new inputs, removed inputs, changed defaults or new
   required permissions. Do not widen `permissions` without asking.
4. If `golangci/golangci-lint-action` changes, the `version:` input (golangci-lint itself) is a
   separate decision; if it changes, update the version everywhere listed in
   `.claude/rules/build-and-release.md`.
5. Run `actionlint` if available; otherwise state that only a GitHub run proves the change.

## docker — Alpine images and apk pins

Alpine repositories keep only the newest revision of each package, so a pinned
`tzdata=…`, `ca-certificates=…` or `build-base=…` stops resolving when upstream moves and the
Docker build fails (CI's `docker.yml` and the release in `cd.yml` alike).

**Pin refresh on the same Alpine release** (the usual case):

1. Find the current revisions for the release in the Dockerfile, for example
   `docker run --rm alpine:X.Y sh -c 'apk update -q && apk policy build-base ca-certificates tzdata'`,
   or from the aports `X.Y-stable` branch. If you cannot reach either, say so and do not guess
   a revision.
2. Update only the pins that changed in `Dockerfile`.
3. `hadolint Dockerfile`; `docker build .` and `docker build --target static --output type=local,dest=dist .`
   if Docker is available.

**Moving to a new Alpine release** (ask before starting; it touches several files):

1. Move both images to the same release: `golang:1.26-alpineX.Y` and `alpine:X.Y` in `Dockerfile`.
2. Re-pin `build-base`, `ca-certificates` and `tzdata` for that release (step 1 above).
3. Find that release's musl version (`apk policy musl` in the new image). Replace
   `tools/licenses/musl-COPYRIGHT` with the `COPYRIGHT` file from that exact musl release, and
   update `muslVersion` and `muslAlpineRelease` in `tools/licenses/main.go`. The licenses test fails
   until the Dockerfile and these agree.
4. `make licenses` (the Alpine release appears in `THIRD_PARTY_LICENSES`).
5. Move the smoke-test image `alpine:X.Y` in `cd.yml` and `docker.yml` (not tracked by
   Dependabot); check whether `debian:11` should move too and ask.
6. `make check`, `hadolint`, both Docker builds; list what could not run.

## go — Go minor-version upgrade

Dependabot ignores Go minor releases on purpose. Ask before starting, and confirm whether the
minimum in `go.mod` should also rise (that raises the requirement for everyone building from
source and for modules that import `pkg`). Touch together:

- `go.mod` `go` line (only if the minimum rises), then `go mod tidy`.
- `go-version: "1.NN.x"` in `ci.yml`, `cd.yml`, `security.yml`, and the comments that name the
  version.
- `golang:1.NN-alpineX.Y` in `Dockerfile`. (The `golang:1.26-…` strings in
  `tools/licenses/main.go` comments and `main_test.go` fixtures are examples; leave them unless
  asked.)
- README "Build from source" and `CONTRIBUTING.md` (prerequisites and the CI description).
- Check golangci-lint supports the new Go version; it may need a bump too.

Then `make check` on the new toolchain, and list anything that only a GitHub run can confirm.
