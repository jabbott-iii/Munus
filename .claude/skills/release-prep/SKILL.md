---
name: release-prep
description: Checks that Munus is ready to release a given version, prepares the documentation that changes with a release line, and prints the release command for the maintainer to run. Never tags or pushes.
argument-hint: "vX.Y.Z[-prerelease]"
disable-model-invocation: true
allowed-tools: Bash(git status *) Bash(git branch *) Bash(git log *) Bash(git for-each-ref *) Bash(git ls-remote *) Bash(git fetch --tags *) Bash(git rev-parse *)
---

# Release preparation for `$ARGUMENTS`

You prepare; the maintainer releases. Do not run `make tag`, `make push-tag`, `make release`,
`git tag`, `git push` or `gh release`. Leave any doc edits uncommitted.

## 1. Version

- `$ARGUMENTS` must match `^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`, the pattern `cd.yml`
  enforces. Reject `+build` metadata even though the `Makefile`'s `check-version` accepts it.
- `git fetch --tags origin` (fetches only), then confirm the tag does not exist locally
  (`git rev-parse -q --verify refs/tags/$ARGUMENTS`) or remotely (`git ls-remote --tags origin`)
  and is greater than the latest release tag
  (`git for-each-ref --sort=-v:refname --format='%(refname:short)' refs/tags`). Say if the tag
  list looks incomplete compared with the versions the docs mention.
- Classify: patch, minor, major or pre-release. Pre-releases do not move the `X.Y` or `latest`
  image tags.

## 2. Repository state

- On `main`, up to date with `origin/main`, clean working tree. Report anything else and stop.
- Ask the maintainer to confirm CI, Docker and Security workflows are green on that commit; you
  cannot see GitHub runs unless `gh run list` works in this session.
- If the maintainer wants a local re-validation, run `/pre-pr <previous-release-tag>` to check
  everything since the last release; otherwise rely on CI and say so.

## 3. Documentation that changes with a release

- `SECURITY.md` supported versions: for a new minor or major line, propose the new table (new
  line supported, older lines per the policy "older releases are not patched") and apply it only
  after confirmation.
- Run `/docs-drift` (scope `all`) and resolve **wrong** findings before tagging.
- README version notes ("Releases after …", "v3.0.0 and older") are historical; change them only
  if a new release makes one of them false.

## 4. Hand-off

Print, for the maintainer to run themselves:

```bash
make release VERSION=$ARGUMENTS
```

Then list the post-release checks:

- The CD run is green: `build` (all rows), `package`, `release`, and for tags `image` and
  `image-attest`.
- The GitHub Release has the six archives, `checksums.txt`, and each archive contains the binary,
  `LICENSE`, `NOTICE` and `THIRD_PARTY_LICENSES`.
- `gh attestation verify <archive> --repo jabbott-iii/Munus` passes for one archive.
- `docker run --rm ghcr.io/jabbott-iii/munus:<X.Y.Z> --version` prints `munus version $ARGUMENTS`
  and `gh attestation verify oci://ghcr.io/jabbott-iii/munus:<X.Y.Z> --repo jabbott-iii/Munus`
  passes (image tags drop the `v`).
- To retry a failed publish, use "Re-run failed jobs", not "Re-run all jobs": re-running all
  rebuilds the images (new digest) and, for an older tag, moves `X.Y`/`latest` back.
