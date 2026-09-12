---
id: PM-0106
title: Query pipeline — grouping, aggregation, leaves, having, sort, projection
state: DONE
phase: 1
depends: ["PM-0103", "PM-0105"]
owner:
rfc: ["§6.1", "§8.3", "§8.4", "§12", "§26.1"]
---

## Summary

The renderer-independent query engine: takes normalized entities + a `QuerySpec`
and produces an ordered tree of grouped/aggregated rows. No renderer may
reimplement any of this (RFC §6.1).

## Scope

- Entity `select` filtering (compiled program from PM-0105).
- Ordered grouping pipeline over arbitrary dimensions (§8.3); `none` must be the
  sole value; no duplicate dimension; deterministic stable group keys.
- Leaf modes process/thread/none independent of grouping (§8.3), including
  host-total single row when both are none.
- Aggregation per §12.3 registry metadata; counts children/procs/threads/leaves
  (§12.2); process-scoped dedup by `ProcessInstanceID` under thread leaves
  (§12.4) — no double counting of RSS/IO.
- `having` filter on aggregated rows (PM-0105).
- Multi-key stable sort (§8.4): direction per key, unavailable sorts last,
  deterministic tie-breakers (row kind, display name, stable key); sort applied
  per sibling level.
- Column projection to requested `columns`.

## Out of scope

- Turning rows into text/JSON (PM-0107).

## Acceptance criteria

- [ ] `group-by comm --leaf none` aggregates CPU correctly with no memory double
      counting (MVP crit. §28.1).
- [ ] `group-by none --leaf process` yields a flat process list (§28.2).
- [ ] Thread leaves never sum process-scoped metrics per thread (§12.4).
- [ ] Sort is stable and unavailable-last; identical inputs ⇒ identical order.
- [ ] Grouping loses/duplicates no distinct entity identity (fuzz, §26.2).

## Tests required

- unit: grouping pipelines, leaf modes, aggregation per metric class, dedup,
  count correctness, sort stability/unavailable ordering.
- property/fuzz: grouping preserves the distinct identity set (§26.2).

## Notes

This is the heart of the observation plane. `QuerySpec` (RFC §25) is the stable
contract between CLI/TUI/daemon and this engine — keep it renderer- and
transport-agnostic.

## Status: DONE (2026-09-12)

Implemented in the initial autonomous build. Core acceptance criteria met and covered by tests (module coverage ≥80%, race-clean). Deferred refinements (golden snapshots, fuzz corpora, and any renderer/format extras) are tracked in PM-0110.
