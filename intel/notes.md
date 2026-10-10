# Engineering Notes & Open Questions

- N-042: `docker buildx imagetools create` keeps index annotations only when the sources are OCI
  manifests. With Docker's classic image store, `docker push` writes Docker manifests and the
  annotations are dropped without an error (seen with Docker 29.8.2, buildx v0.37.1), so `cd.yml`
  sets none. GHCR links the package to the repository because the workflow publishes it with
  `GITHUB_TOKEN`, and the per-platform images carry `org.opencontainers.image.source`.
- N-043: The `image` job leaves the per-platform tags `X.Y.Z-amd64` and `X.Y.Z-arm64` in the
  package (plain `docker push` needs a tag); only the multi-platform digest is attested. `latest`
  and `X.Y` move on every stable `vX.Y.Z` run, so tagging an older release after a newer one
  (`latest`: any older line; `X.Y`: an older patch of that minor), or "Re-run all jobs" on an older
  tag, moves them back (`SECURITY.md` only patches the latest line).
  "Re-run all jobs" also rebuilds the images, so `X.Y.Z` then points to a new digest with its own
  attestation; to retry a failed publish, use "Re-run failed jobs", which reuses the built images.
  Image tags dropped the `v` after `v3.0.2` (Salus's scheme); the `v3.0.2` image keeps its
  `v3.0.2` tags, and no `3.0` tag exists unless it is added by hand.
