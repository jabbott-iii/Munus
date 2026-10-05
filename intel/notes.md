# Engineering Notes & Open Questions

- N-042: `docker buildx imagetools create` keeps index annotations only when the sources are OCI
  manifests. With Docker's classic image store, `docker push` writes Docker manifests and the
  annotations are dropped without an error (seen with Docker 29.8.2, buildx v0.37.1), so `cd.yml`
  sets none. GHCR links the package to the repository because the workflow publishes it with
  `GITHUB_TOKEN`, and the per-platform images carry `org.opencontainers.image.source`.
- N-043: The `image` job leaves the per-platform tags `<tag>-amd64` and `<tag>-arm64` in the
  package (plain `docker push` needs a tag); only the multi-platform digest is attested. `latest`
  moves on every `vX.Y.Z` run, so tagging an older line after a newer one, or "Re-run all jobs" on
  an older tag, moves it back (`SECURITY.md` only patches the latest line). "Re-run all jobs" also
  rebuilds the images, so `<tag>` then points to a new digest with its own attestation; to retry a
  failed publish, use "Re-run failed jobs", which reuses the built images.
