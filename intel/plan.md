# Active Plans & Follow-on Work

## GitHub Packages image (2026-10-05)
- P-061: After the first release with the image jobs, confirm that the CD run is green (the Linux
  rows of `build`, `image`, `image-attest`), that the `munus` package on ghcr.io is linked to the
  repository, and that `docker run --rm ghcr.io/jabbott-iii/munus:<tag> --version` prints the tag
  and `gh attestation verify oci://ghcr.io/jabbott-iii/munus:<tag> --repo jabbott-iii/Munus` passes.
  A new package starts private; making it public (package settings) cannot be undone. If a `munus`
  package already exists under the account without access for this repository, the push fails
  with `permission_denied: write_package`; give the repository Write access under the package's
  "Manage Actions access" settings.
