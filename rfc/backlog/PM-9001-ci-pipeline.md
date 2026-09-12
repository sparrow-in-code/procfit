---
id: PM-9001
title: CI pipeline — test, race, fuzz smoke, vet, static analysis, golden
state: TODO
phase: 0
depends: ["PM-0001"]
owner:
rfc: ["§26", "§31.9"]
---

## Summary

Continuous-integration gates enforcing the testing strategy from day one. Should
be stood up right after the module skeleton so every later ticket lands green.

## Scope

- CI running (§31.9): `go build ./...`, `go test ./...`, `go test -race ./...`,
  fuzz smoke (short `-fuzz` runs on registered corpora), `go vet ./...`, and a
  chosen static analyzer (e.g. staticcheck).
- Golden-test verification + a `-update` mode convention (§26.4).
- Coverage reporting (informational threshold, not a hard gate initially).
- `gofmt`/`goimports` check.
- Linux runner (project is Linux-only, §32).

## Out of scope

- Release/packaging automation.

## Acceptance criteria

- [ ] All listed checks run on every PR and block merge on failure.
- [ ] Race detector runs green.
- [ ] Fuzz smoke executes registered fuzz targets briefly.
- [ ] Golden update workflow is documented in DEVELOPMENT.md.

## Tests required

- meta: a trivial passing test proves the pipeline is wired; a deliberately
  broken branch proves gates fail.

## Notes

This ticket operationalizes DEVELOPMENT.md's testing + TDD policy. Keep it
updated as new test categories (integration, perf) come online in later phases.
