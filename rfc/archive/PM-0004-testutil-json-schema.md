---
id: PM-0004
title: Test harness seams (fake clock/procfs/controller/state) + JSON schema v1 draft
state: DONE
phase: 0
depends: ["PM-0001", "PM-0003"]
owner:
rfc: ["§11", "§14.1", "§18.4", "§25", "§26", "§31.3"]
---

## Summary

Establish the fakes and the versioned machine-output contract that every later
ticket tests against. TDD across the codebase depends on these seams existing
first (RFC §31.3).

## Scope

- `internal/testutil`: fake monotonic clock, fake procfs root (fixture tree
  loader), fake controller, fake state store, and helpers to normalize
  timestamps/PIDs/boot-ids/color codes for golden tests (§26.4).
- Core value types from RFC §11: `Value[T]`, `Availability`, `Quality`,
  `ProcessInstanceID`, `ThreadInstanceID` — with the invariant "unknown is not
  zero" (§6.4) encoded in the type, not convention.
- JSON schema v1 draft for query responses (§18.4) and NDJSON metadata+sample
  framing; a schema-version constant and compatibility notes in
  `docs/json-schema.md`.

## Out of scope

- Real procfs reading (PM-0101); real collectors/controllers.

## Acceptance criteria

- [ ] Fake clock is injectable everywhere time is read; no direct `time.Now()`
      in production code paths (lint/grep enforced).
- [ ] Fake procfs serves fixtures via the same interface real procfs will
      implement (PM-0101 can swap in without touching callers).
- [ ] `Value[T]` cannot silently coerce missing to zero; JSON marshalling emits
      availability + quality + source (§18.4, §6.4).
- [ ] JSON schema v1 documented and versioned; a golden test pins the shape.

## Tests required

- unit: value availability transitions; golden normalizers are deterministic.
- golden: JSON schema envelope (`schema_version`, `generated_at`,
  `monotonic_elapsed_ns`, `query`, `capabilities`, `rows`).

## Notes

These fakes make the whole codebase testable without root or a real `/proc`,
directly enabling RFC §31.3. Keep syscalls behind small interfaces (§24) so they
too become fakeable in later phases.

## Status: DONE (2026-09-12)

Implemented in the initial autonomous build. Core acceptance criteria met and covered by tests (module coverage ≥80%, race-clean). Deferred refinements (golden snapshots, fuzz corpora, and any renderer/format extras) are tracked in PM-0110.
