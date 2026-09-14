---
id: PM-0509
title: Non-eBPF wakeups from /proc/PID/sched (fallback)
state: TODO
phase: 5
depends: ["PM-0503"]
owner:
rfc: ["§13", "§13.6"]
---

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

- [ ] `/proc/PID/sched` parsed for wakeup counters; per-second rate computed by
      the sampler like other rate metrics.
- [ ] `wakeups` resolves via eBPF when available, else procfs; the active source
      is reported and there is no double counting.
- [ ] Absent scheduler stats → unavailable (with a clear reason).
- [ ] Arch-neutral (scheduler stats are not vendor/arch specific).

## Tests required

- unit: `/proc/PID/sched` parser over fixtures (present/absent fields); rate math;
  source-precedence (eBPF over procfs) selection.

## Notes

Purely a portability/no-root win — no new privileges. Complements PM-0503 rather
than replacing it.
