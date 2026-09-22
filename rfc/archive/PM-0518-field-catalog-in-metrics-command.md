---
id: PM-0518
title: procfit metrics lists the full field catalog (metrics + columns + dimensions)
state: DONE
phase: 1
depends: ["PM-0517"]
owner:
rfc: ["§8", "§13"]
---

## Summary

`procfit metrics` only listed metric columns (the metric registry), not the
structural/identity columns (target, pid, pstate, wchan, …) or the group-by
dimensions — so there was no single command to discover everything usable in
`--columns`/`--group-by`/`--sort`/`--having`. Make it complete, each section
sourced from its registry (single source of truth, no drift).

## Scope

- `render.StructuralColumns()` exposes the structural column registry (ordered).
- `procfit metrics` prints three sections — METRICS (registry), COLUMNS (structural,
  tagged [g]roup/[s]ort/[f]ilter), DIMENSIONS (group-by axes, `*` = also a column);
  `--format json` returns `{metrics, columns, dimensions}`.
- `--columns` help points at `procfit metrics` instead of an illustrative ellipsis.
- pstate column made sortable/filterable (consistency with its field usage).

## Acceptance criteria

- [x] `procfit metrics` lists metrics + structural columns + dimensions, noting the
      kind of each; JSON mirrors it.
- [x] Every listed id is sourced from a registry (adding a metric/column/dimension
      shows up automatically).

## Tests required

- unit: metrics list (table + json) includes a metric, a structural column (wchan),
  and the dimensions section/array.

## Status: DONE (2026-09-22)

Three registry-sourced sections in `procfit metrics` (+ JSON object). Combined with
the registry-derived `--group-by`/`--profile` help (PM-0517 follow-up), the CLI's
field vocabulary is now single-sourced end to end: metrics from
`metrics.Registry`, columns from the render registry, dimensions from
`query.Dimensions`. A full unified field-descriptor (one type providing render +
group-key + expr behaviour) remains possible but is a larger refactor and was not
needed to close the discoverability gap. `make check` green (80.6%).
