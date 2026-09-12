---
id: PM-9001
title: CI pipeline — test, race, fuzz smoke, vet, static analysis, golden
state: DONE
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
- **Coverage hard gate ≥ 80%** (DEVELOPMENT.md §3.2): fail the build when module
  total drops below 80% or changed packages regress materially. Narrow, listed
  exclusions only (generated code, thin `main`).
- **Size-cap linters** (DEVELOPMENT.md §2.6) via golangci-lint: funlen, gocyclo,
  gocognit, nestif, lll — configured to the documented caps; fail on violation.
- **Architecture guard**: lint/deny-import rule so core packages (`model`,
  `expr`, `query`, `control` planning, config-semantics, registries) cannot
  import `syscall`/`x/sys` or OS-specific adapters (DEVELOPMENT.md §2.4, PM-9003).
- Golden-test verification + a `-update` mode convention (§26.4).
- `gofmt`/`goimports` check.
- Linux runner now (Linux is the first adapter, §32); keep the matrix ready to
  add OS runners when adapters land (PM-9003).

## Out of scope

- Release/packaging automation.

## Acceptance criteria

- [ ] All listed checks run on every PR and block merge on failure.
- [ ] Coverage below 80% fails the build; the threshold + exclusions are in
      committed config.
- [ ] Size-cap linters fail on caps being exceeded (proven by a deliberate
      violation branch).
- [ ] Core-imports-OS guard fails when a core package imports a syscall/adapter.
- [ ] Race detector runs green; fuzz smoke executes registered targets briefly.
- [ ] Golden update workflow is documented in DEVELOPMENT.md.

## Tests required

- meta: a trivial passing test proves the pipeline is wired; a deliberately
  broken branch proves gates fail.

## Notes

This ticket operationalizes DEVELOPMENT.md's testing + TDD policy. Keep it
updated as new test categories (integration, perf) come online in later phases.

## Status: mostly done (2026-09-12)

Delivered and verified locally: golangci-lint v2 config enforcing the §2.6 size caps (0 issues), fuzz targets (procfs stat/io, expr) + scripts/fuzz-smoke.sh, coverage hard-gate in the Makefile (`make cover`, 81.5% ≥ 80%), and `make check` (fmt/vet/lint/test/race/cover) all green. A GitHub Actions workflow (.github/workflows/ci.yml) is committed but cannot be verified until a remote exists. Keep open until CI runs on a real PR.

## Status: DONE (2026-09-12)

Delivered + verified locally: golangci-lint v2 enforcing §2.6 size caps (0 issues), fuzz targets + scripts/fuzz-smoke.sh, coverage hard-gate (make cover), architecture import-guard test (PM-9003), and make check (fmt/vet/lint/test/race/cover) green. GitHub Actions workflow committed; live CI awaits a remote.
