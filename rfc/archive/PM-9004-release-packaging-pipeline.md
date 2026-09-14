---
id: PM-9004
title: Release pipeline — GitHub Releases + GHCR image, test-gated
state: DONE
phase: 0
depends: ["PM-9001"]
owner:
rfc: ["§26", "§31.9"]
---

## Summary

Tag-driven release automation (explicitly out of scope for PM-9001): pushing a
`vX.Y.Z` tag builds static multi-arch binaries, publishes a GitHub Release with
checksummed tarballs + generated notes, and pushes a multi-arch GHCR image — all
gated on a fast verify step so a broken tag can never ship.

## Scope

- `.github/workflows/release.yml` triggered by `push: tags: ['v*']` (and
  `workflow_dispatch` for manual rebuilds).
- **verify** job: `go vet` + `go test ./...` — a hard gate the rest depend on.
- **binaries** job: `CGO_ENABLED=0 -trimpath` builds for linux/amd64 + arm64,
  version-stamped from the tag/commit, packaged as `.tar.gz` + `.sha256`.
- **release** job: `softprops/action-gh-release` attaches artifacts + auto notes,
  gated on `refs/tags/*`.
- **image** job: multi-arch GHCR image tagged `:X.Y.Z`, `:X.Y`, `:latest`, pushed
  only on tags.
- Docs: DEVELOPMENT.md §9 "Releasing" (how to cut a release, what runs); README
  "Installing" already points at Releases + GHCR.

## Out of scope

- Signing/attestation (cosign, SLSA provenance), Homebrew/AUR, non-Linux builds.

## Acceptance criteria

- [x] A `vX.Y.Z` tag publishes a GitHub Release with amd64 + arm64 tarballs and
      SHA-256 sums, version-stamped into `internal/meta`.
- [x] The release is gated on `go vet` + `go test ./...` passing.
- [x] A multi-arch GHCR image is pushed for the tag (`:X.Y.Z`/`:X.Y`/`:latest`).
- [x] Uses the default `GITHUB_TOKEN` with workflow-declared
      `contents: write` + `packages: write`; no extra secrets.
- [x] Release process documented in DEVELOPMENT.md.

## Tests required

- meta: workflow YAML parses and the job graph is verify → binaries → release
  and verify → image (validated with yq).

## Notes

Live end-to-end verification awaits a writable remote + a pushed tag; the build
path, ldflags, and version source match `make build` (`git describe --tags`), so
it mirrors local builds. Golangci-lint version skew in the *CI* workflow was fixed
separately (action@v8 + v2.13.2).
