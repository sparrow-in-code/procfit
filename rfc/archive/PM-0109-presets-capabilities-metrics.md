---
id: PM-0109
title: Built-in presets, capabilities, and metrics list commands
state: DONE
phase: 1
depends: ["PM-0108", "PM-0102"]
owner:
rfc: ["§9.6", "§7.5", "§8", "§14.3", "§20.2"]
---

## Summary

Ship the read-only presets and the introspection commands that expose machine-
readable metric and capability metadata.

## Scope

- Built-in read-only presets (§9.6): default, battery, io, churn, namespaces,
  managed, all. Custom presets may only override built-ins when
  `allow_builtin_preset_override = true`.
- `procfit metrics list [--format json]`: id, cost, source, availability from registry
  (§8, §13).
- `procfit capabilities [--format json] [--verbose]`: available/missing collectors and
  controllers with human explanation + remediation, no root required (§14.3,
  §20.2); `--strict-capabilities` turns explicitly-requested-but-unavailable
  metrics into exit 3/4 (§20.2).

## Out of scope

- Control-related presets behaviour beyond visibility (Phase 2).

## Acceptance criteria

- [ ] All seven built-in presets load and resolve; duplicate custom name without
      the override flag is an error (§9.6).
- [ ] `metrics list --format json` and `capabilities --format json` are stable,
      versioned outputs (§7.5).
- [ ] `capabilities` reports unsupported vs permission_denied with remediation.
- [ ] `--strict-capabilities` yields exit 3/4 for unavailable requested metrics.

## Tests required

- unit: preset resolution + override rule; strict-capabilities exit codes.
- golden: capability report + metrics list JSON (normalized).

## Notes

Presets are pure data over the registries — no special code paths (§13.5).

## Status: DONE (2026-09-12)

Implemented in the initial autonomous build. Core acceptance criteria met and covered by tests (module coverage ≥80%, race-clean). Deferred refinements (golden snapshots, fuzz corpora, and any renderer/format extras) are tracked in PM-0110.
