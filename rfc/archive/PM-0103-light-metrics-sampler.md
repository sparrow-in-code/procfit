---
id: PM-0103
title: Light /proc metrics and rate sampler
state: DONE
phase: 1
depends: ["PM-0102"]
owner:
rfc: ["§6.4", "§13.1", "§13.6", "§22"]
---

## Summary

The cost-level-0 process collector and the delta/rate engine that turns counter
samples into per-second rates using real monotonic elapsed time.

## Scope

- Light metrics from RFC §13.1: cpu (+user/system/normalized), rss, vsz,
  threads, minor/major faults, ctxsw voluntary/involuntary, disk-rbps/disk-wbps,
  io-rchar/wchar, read/write-syscalls, cancelled-write-bytes, blkio-delay,
  pstate, pnice, age. Churn metrics (pids/tids born/died) may land here or in a
  sibling ticket — declare scope explicitly.
- Rate sampler (§13.6): monotonic elapsed denominator; first sample
  `warming_up`; counter decrease ⇒ discard delta (reset/replacement); keep
  timestamp + collection duration per batch.
- Metric scope tagging (host/process/thread) for later dedup (§12.4).
- `--instant` / warm-up semantics feed here (used by PM-0108).

## Out of scope

- Grouping/aggregation (PM-0106); thread enumeration dedup lives in query.

## Acceptance criteria

- [ ] First rate sample is `warming_up`, not zero (§6.4, §13.6).
- [ ] Rates use actual elapsed monotonic time, not the configured interval.
- [ ] Counter decrease discards the delta instead of emitting a huge/negative
      rate.
- [ ] Permission-denied `/proc/PID/io` yields `permission_denied`, not zero.
- [ ] cpu can exceed 100% for multithreaded processes; cpu-normalized divides by
      online CPU count.

## Tests required

- unit: delta math, warm-up, reset handling, elapsed-time correctness (fake
  clock); per-metric availability on denied/vanished reads.
- perf: light profile scan against synthetic 100/1k/10k fixtures within budget
  (§22, §26.5).

## Notes

This is the metric that makes the tool useful as `pidstat`. Keep each metric's
delta/aggregate behaviour in its registry descriptor (PM-0003), not hardcoded in
the sampler.

## Status: DONE (2026-09-12)

Implemented in the initial autonomous build. Core acceptance criteria met and covered by tests (module coverage ≥80%, race-clean). Deferred refinements (golden snapshots, fuzz corpora, and any renderer/format extras) are tracked in PM-0110.
