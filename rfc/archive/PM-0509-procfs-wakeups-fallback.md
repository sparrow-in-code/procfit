---
id: PM-0509
title: Non-eBPF wakeups from /proc/PID/sched (fallback)
state: DONE
phase: 5
depends: ["PM-0503"]
owner:
rfc: ["§13", "§13.6"]
---

## Status: DONE (2026-09-23)

`procfs.SchedWakeupsCollector` (`internal/procfs/sched_wakeups.go`) parses
`/proc/PID/sched` `nr_wakeups` and self-windows (read, 200ms, read) into a
per-second `wakeups` rate — no root, no eBPF. Registered in `assembly` after the
eBPF `WakeupsCollector`; it is a fallback that only fills processes whose
`wakeups` the eBPF source did not set (`Metric("wakeups").Present()`), so eBPF
stays preferred and there is no double counting — and it skips its measurement
window entirely when nothing is pending. Absent `CONFIG_SCHEDSTATS` (field
missing) or file → `Unsupported`, never zero. `procfit capabilities` gains a
`wakeups-procfs` line (probes `/proc/self/sched` for `nr_wakeups`). `parseSchedField`
is a reusable `key : value` sched parser. Source label on values is `sched`.
Tests: `TestParseSchedField`, `TestSchedWakeupsCollector_Window`,
`TestSchedWakeupsCollector_EBPFPreferred`. Commit: (local).

## Summary

Provide a per-process wakeup count without eBPF/root. `wakeups`/`timer-wakeups`
today need the eBPF collector (privileged); on an ordinary laptop the `battery`
profile falls back to the `ctxsw-voluntary` proxy. `/proc/PID/sched` exposes real
scheduler wakeup counts (with `CONFIG_SCHEDSTATS`), giving a truer no-root signal.

## Scope

- Parse `/proc/PID/sched` for `nr_wakeups` (and related `nr_switches`,
  `nr_voluntary_switches`, `nr_involuntary_switches`) and derive a per-second
  `wakeups` rate.
- Register it as a **fallback source** for the existing `wakeups` metric id:
  prefer the eBPF collector (richer: waker attribution) when available, else use
  procfs; surface which source is active via `procfit capabilities`.
- Degrade to unavailable when `CONFIG_SCHEDSTATS`/`sched_debug` isn't present
  (fields absent), never zero.

## Out of scope

- Waker attribution / timer-vs-device breakdown (keep those eBPF-only).

## Acceptance criteria

- [x] `/proc/PID/sched` parsed for wakeup counters; per-second rate computed by
      the sampler like other rate metrics.
- [x] `wakeups` resolves via eBPF when available, else procfs; the active source
      is reported and there is no double counting.
- [x] Absent scheduler stats → unavailable (with a clear reason).
- [x] Arch-neutral (scheduler stats are not vendor/arch specific).

## Tests required

- unit: `/proc/PID/sched` parser over fixtures (present/absent fields); rate math;
  source-precedence (eBPF over procfs) selection.

## Notes

Purely a portability/no-root win — no new privileges. Complements PM-0503 rather
than replacing it.
