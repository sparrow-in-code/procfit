---
id: PM-0527
title: cpu-time (cumulative CPU) + per-thread uptime/cpu-time metrics
state: DONE
phase: 1
depends: ["PM-0103"]
owner:
rfc: ["§13"]
---

## Summary

Add a cumulative `cpu-time` metric (total CPU seconds the entity has consumed) and
make it, plus the existing `age` (uptime), work per-thread under `--leaf thread`.
Both sortable.

## Status: DONE (2026-09-23)

- **uptime** already existed as `age` (gauge, `UnitDuration`); only its per-thread
  support was missing. Description clarified to "uptime".
- **`cpu-time`** added as a cost-0 gauge (`UnitDuration`, AggSum): a process/thread
  gauge `(utime+stime)/hz` seconds (`metrics.builtin` + `collect.gaugeSpecs`).
- **Per-thread** support: factored `ageSeconds`/`cpuTimeSeconds` helpers and added a
  `threadGauges` table so `threadMetric` resolves `cpu-time` and `age` from each
  thread's own `StartTicks`/`UTime+STime` (unlike the cpu-family *rates*, these
  absolute gauges need no prior sample). Both are metrics, hence sortable
  automatically.
- Validated live: per-pid `--sort cpu-time:desc`, and per-tid under
  `--leaf thread` (arx_pro main thread 9269 s CPU over a 74675 s uptime).

## Tests

- `collect.TestSampler_CPUTimeAndAge` (process gauges on first sample) and
  `TestSampler_ThreadCPUTimeAndAge` (per-thread cpu-time + age from the thread's
  own start time).

## Notes

Values render as raw seconds by default and as human durations with `-h` (same as
`age`). Aggregation: `cpu-time` sums across a group (total CPU consumed); `age`
takes the max (oldest member).
