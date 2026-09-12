---
id: PM-0003
title: Metric, dimension, and column registries
state: TODO
phase: 0
depends: ["PM-0001"]
owner:
rfc: ["§12.1", "§12.3", "§13", "§18.1", "§25"]
---

## Summary

Central, data-driven registries describing metrics, grouping dimensions, and
render columns as descriptor metadata. Aggregation, cost, units, scope, and
capability requirements live here — never in renderers or the query engine.

## Scope

- `MetricDescriptor` registry (RFC §25/§13): id, unit, scope (host/process/
  thread), kind, cost level, collector, monotonicity, counter width, delta
  behaviour, aggregation function, description. Populate light metrics (§13.1)
  and declare (unimplemented) higher-cost ones with availability metadata.
- Dimension registry (RFC §12.1): stable machine id, display label, key type,
  availability.
- Column registry (RFC §18.1): id + aliases, header, widths, alignment, unit,
  formatter hook, compatible row/entity kinds, required metrics/capabilities,
  default aggregate display.
- Metric profiles (§13.5) as named sets referencing metric ids.
- Aggregation kinds enum + registry mapping (§12.3), including
  "sum-once-per-distinct-process" and mixed/min-max for nice.
- Lookup + validation API consumed by config (PM-0002) and expr (PM-0105).

## Out of scope

- Actually collecting any metric (Phase 1).
- Column rendering (PM-0107).

## Acceptance criteria

- [ ] Every metric/dimension/column has a unique stable id; duplicates fail at
      init (test enforced).
- [ ] Canonical vs alias ids resolve (e.g. `disk-rbps` alias handling per §13.1).
- [ ] Registry exposes cost, scope, and aggregation so double-counting rules
      (§12.4) are expressible without renderer logic.
- [ ] `procfit metrics list` and `procfit capabilities` (PM-0109) can be built purely from
      registry metadata.

## Tests required

- unit: uniqueness, alias resolution, profile expansion, aggregation lookup,
  scope tagging for dedup.
- golden: `metrics list --format json` shape.

## Notes

Registries are the Open/Closed seam: adding a metric = adding a descriptor +
collector, touching no query/render code. Choose canonical disk metric names now
(RFC §30.3 recommends `disk-rbps`/`disk-wbps`); record the decision in a
PM-90NN note if it differs.
