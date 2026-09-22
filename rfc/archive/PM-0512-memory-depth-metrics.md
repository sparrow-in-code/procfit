---
id: PM-0512
title: Memory-depth metrics + memory profile (PSS/USS, swap, OOM, RSS breakdown)
state: DONE
phase: 5
depends: ["PM-0101"]
owner:
rfc: ["§13"]
---

## Summary

RSS double-counts shared memory and hides swap; add the metrics that show a
process's honest memory footprint and its risk under pressure, plus a `memory`
profile. Also fix `ipc` display and add `cache-references`.

## Scope

- Cost-0 (already-read /proc/PID/status): `swap` (VmSwap), `mem-peak` (VmHWM),
  `rss-anon`/`rss-file`/`rss-shmem`.
- Cost-1 `procmem` collector (/proc/PID/smaps_rollup + oom_score*): `pss`, `uss`
  (Private_Clean+Dirty), `swap-pss`, `oom-score`, `oom-score-adj`.
- `memory` profile: rss, pss, uss, swap, mem-peak, rss-anon, oom-score, major-faults.
- perf: add `cache-references` (enables a miss rate) and fix `ipc` (was UnitInteger,
  rounded 1.85→2) via a new `UnitRatio` (2-decimal).

## Acceptance criteria

- [x] PSS/USS/swap-pss from smaps_rollup; RSS breakdown, swap, peak from status;
      OOM score/adj — all per process, unavailable (never zero) when unreadable.
- [x] `memory` profile resolves and drives columns; `--metrics memory` works.
- [x] `ipc` shows decimals; `cache-references` registered + collected.

## Tests required

- unit: parseSmapsRollup + ProcMemCollector (fixture, incl. missing rollup →
  unavailable); parseStatus memory fields; UnitRatio formatting.

## Status: DONE (2026-09-22)

Shipped `ProcMemCollector` (smaps_rollup + oom_score/oom_score_adj), status memory
fields on the light path, `builtin_mem.go` descriptors, the `memory` profile,
`UnitRatio` + `ipc` fix, and `cache-references` in the perf collector. Verified
live: `ps --metrics memory` shows swap/mem-peak/oom-score/rss-anon with real
values and `?` for PSS/USS on other users' processes (permission), never a fake
zero. `make check` green (80.7%). PSI / delay-accounting deferred to PM-0514.
