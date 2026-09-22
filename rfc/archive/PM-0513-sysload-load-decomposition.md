---
id: PM-0513
title: Load-average decomposition (runq-delay, blkio-delay) + sysload profile
state: DONE
phase: 5
depends: ["PM-0101"]
owner:
rfc: ["§13"]
---

## Summary

Load average counts Runnable + Uninterruptible tasks, so high load can be CPU
contention or I/O blocking and CPU% cannot tell them apart. Add the two per-process
signals that split them, and a `sysload` profile.

## Scope

- `blkio-delay` — % of wall time blocked on block I/O, from /proc/PID/stat field 42
  (delayacct_blkio_ticks), light path. Gated on `/proc/sys/kernel/task_delayacct`
  (Disabled → unavailable, not a false 0).
- `runq-delay` — % of wall time runnable but waiting for a CPU, from
  /proc/PID/schedstat field 2, a cost-1 self-windowed `schedstat` collector. Needs
  CONFIG_SCHEDSTATS (else unavailable).
- `sysload` profile: cpu, runq-delay, blkio-delay, ctxsw-involuntary, major-faults.

## Acceptance criteria

- [x] runq-delay and blkio-delay reported as % of wall time, per process, with
      correct availability gating (never a misleading zero).
- [x] `sysload` profile resolves and drives columns.

## Tests required

- unit: parseSchedstatRunDelay + SchedstatCollector window (fixture, incl. missing
  schedstat → unavailable); stat field-42 parse; delayacct gating.

## Status: DONE (2026-09-22)

Shipped the `schedstat` collector (self-windowed runq-delay), blkio-delay on the
light path with delayacct gating (`detectDelayacct`), `builtin_load.go`, and the
`sysload` profile. Verified live: `ps --metrics sysload` renders runq-delay/blkio-
delay/cpu/ctxsw-involuntary/major-faults; blkio-delay shows unavailable when
delayacct is off. `make check` green (80.7%). Per-process R/D thread counts and PSI
deferred to PM-0514.
