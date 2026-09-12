---
id: PM-0102
title: Collector interface, scheduler, and capability probing
state: TODO
phase: 1
depends: ["PM-0101", "PM-0003"]
owner:
rfc: ["§14", "§14.3", "§20", "§22"]
---

## Summary

The collector plane: a `Collector` interface, a scheduler that resolves
requested metrics to the minimum collector set and runs each on its own
interval, and capability probing that reports available/unsupported/denied.

## Scope

- `Collector` interface (RFC §14): `ID`, `Describe`, `Probe`, `Collect`.
- Scheduler (§14): resolve metrics → minimal collectors; independent per-collector
  intervals; shared identity inventory; immutable sample generations; bounded
  concurrency (no I/O storm); coalesce duplicate proc reads; record duration,
  skipped deadlines, errors, permission failures.
- Capability probing (§14.3): procfs presence/hidepid, cgroup version, pidfd,
  delay accounting, BPF/perf/namespace availability — each `available`,
  `unsupported`, or `permission_denied` with a reason.
- `--collector-interval NAME=DURATION` plumbing (§8.1); disabled collectors do
  no periodic work (§22).

## Out of scope

- The light metric collector body (PM-0103); FD/eBPF/perf collectors (Phase 5).

## Acceptance criteria

- [ ] Requesting a metric enables only the collector(s) that produce it.
- [ ] Each collector honours its own interval; a missed deadline does not fake
      nominal elapsed time (§13.6).
- [ ] Sample generations are immutable; a collector failure does not discard base
      identity (§21.3).
- [ ] Capability report distinguishes unsupported vs permission-denied with
      remediation hints (§14.3, §20.2).
- [ ] Bounded concurrency cap is configurable and enforced (§22).

## Tests required

- unit: metric→collector resolution; interval scheduling with fake clock;
  concurrency bound; deadline-miss accounting.
- unit: capability probe matrix using fake procfs/syscall seams.

## Notes

Interface segregation: collectors depend only on the procfs reader + clock, not
on query/render. Scheduler publishes generations the query layer consumes read-
only.
