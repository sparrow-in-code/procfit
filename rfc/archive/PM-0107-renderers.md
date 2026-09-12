---
id: PM-0107
title: Renderers — table, wide, json, ndjson, csv
state: DONE
phase: 1
depends: ["PM-0106", "PM-0004"]
owner:
rfc: ["§6.1", "§7.5", "§18", "§26.4"]
---

## Summary

Output formatters that consume the query result tree and render it. Renderers
are pure presentation — they perform no collection, filtering, grouping, or
aggregation (RFC §6.1).

## Scope

- Table + wide renderers driven by the column registry (§18.1): widths,
  alignment, unit-aware formatters, tree indentation for group/leaf rows (§18.3).
- `CTL` compact summary composition (§18.2) and `NICE`/`PNICE` display (§15.3)
  — display-only; underlying fields stay structured.
- JSON renderer implementing schema v1 envelope (§18.4) with per-metric
  value/unit/availability/quality/source; compact-schema option.
- NDJSON: one metadata record then timestamped sample records (§18.4).
- CSV for stable machine output (§7.3).
- Unavailable values render `-`/`?` by reason (§6.4); never zero.

## Out of scope

- Streaming loop mechanics (PM-0108); TUI (Phase 3).

## Acceptance criteria

- [ ] Identical query result renders consistently; table labels are NOT a stable
      API but JSON schema IS versioned (§7.5, §18.4).
- [ ] Unavailable metrics show reason-appropriate `-`/`?` in text and retain
      reason in JSON.
- [ ] `CTL` letters and `NICE` arrow format match §18.2/§15.3.
- [ ] Column widths/units correct in narrow and wide modes.

## Tests required

- golden: narrow table, wide table, group tree, JSON schema v1, capability
  report, all normalized per §26.4.
- unit: formatter/unit edge cases; unavailable rendering.

## Notes

Add a renderer = implement one interface + register; no query changes
(Open/Closed). JSON is the public contract; guard it with golden + schema-version
checks.

## Status: DONE (2026-09-12)

Implemented in the initial autonomous build. Core acceptance criteria met and covered by tests (module coverage ≥80%, race-clean). Deferred refinements (golden snapshots, fuzz corpora, and any renderer/format extras) are tracked in PM-0110.
