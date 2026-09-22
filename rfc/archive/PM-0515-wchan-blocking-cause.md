---
id: PM-0515
title: wchan — the kernel blocking-cause field (column, group-by, filter)
state: DONE
phase: 5
depends: ["PM-0513"]
owner:
rfc: ["§13", "§12"]
---

## Summary

`blkio-delay` only catches *block-I/O* D-state stalls; the sneaky load culprits
(filesystem journaling, NFS, memory reclaim, lock contention, RAID) block on other
kernel waits. Expose `/proc/PID/wchan` — the kernel function a blocked task sleeps
in — which names those causes and, grouped, clusters all tasks stuck in the same
place.

## Scope

- `wchan` as a process string field: render column, group-by dimension, and
  select/having filter field (with a `pstate` alias for the `state` field).
- Read `/proc/PID/wchan` **only when a query references wchan** (column, group-by,
  or filter) — gated via `Source.SetReadWchan`, mirroring thread enumeration, so
  the common path pays nothing. `RowEnv` exposes state/wchan on process leaves.
- Add wchan (+ pstate) to the sysload profile's default columns (see PM-0516).

## Acceptance criteria

- [x] wchan usable as column, `--group-by wchan`, and `--select/--having` field;
      unreadable/running tasks render empty, never fabricated.
- [x] The wchan read happens only when the query uses it (gating verified).

## Tests required

- unit: Source wchan read gating (off by default, "0"→empty); Build surfaces wchan
  in ExprFields for filter gating; RowEnv/AllowedRowFields accept state/wchan.

## Status: DONE (2026-09-22)

`Wchan` added to ProcStat/Process, read lazily via `SetReadWchan` (app enables it
from the resolved query's columns/group-by/ExprFields). New `wchan` render column,
group-by dimension, and select/having field; `state`/`pstate`/`wchan` allowed in
both filter stages and resolved in `RowEnv`. Verified live: `--profile sysload`
shows real symbols (do_epoll_wait, poll_schedule_timeout); `--group-by wchan`
clusters blocked tasks. `make check` green (80.5%). PSI/taskstats remain in PM-0514.
